package server

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/config"
)

type ListenerHealth int

const (
	HealthUnknown  ListenerHealth = 0
	HealthHealthy  ListenerHealth = 1
	HealthUnstable ListenerHealth = 2
	HealthBurned   ListenerHealth = 3
)

type ProbeTarget struct {
	ID     string
	Scheme string
	Host   string
	Port   int
}

type ProbeResult struct {
	Target       ProbeTarget
	TCPOK        bool
	TLSOK        bool
	HTTPStatus   int
	ResponseTime time.Duration
	Burned       bool
	Timestamp    time.Time
	Error        string
}

type CircuitBreaker struct {
	mu            sync.RWMutex
	targets       map[string]*TargetHealth
	vantagePoints []string // external proxy URLs for probing
	checkInterval time.Duration
	config        *config.Config
	onBurned      func(targetID string) // callback to rotate agents
	stopCh        chan struct{}
}

type TargetHealth struct {
	Target      ProbeTarget
	Status      ListenerHealth
	LastProbe   time.Time
	ConsecutiveFails int
	Results     []ProbeResult // ring buffer, keep last 10
	FailReasons []string
}

var (
	circuitBreaker     *CircuitBreaker
	circuitBreakerOnce sync.Once
)

func NewCircuitBreaker(cfg *config.Config) *CircuitBreaker {
	cb := &CircuitBreaker{
		targets:       make(map[string]*TargetHealth),
		vantagePoints: cfg.Server.VantagePoints,
		checkInterval: CBCheckInterval,
		config:        cfg,
		onBurned:      nil,
		stopCh:        make(chan struct{}),
	}
	circuitBreaker = cb
	return cb
}

func GetCircuitBreaker() *CircuitBreaker {
	return circuitBreaker
}

func (cb *CircuitBreaker) RegisterTarget(id, scheme, host string, port int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.targets[id] = &TargetHealth{
		Target: ProbeTarget{ID: id, Scheme: scheme, Host: host, Port: port},
		Status: HealthUnknown,
	}
}

func (cb *CircuitBreaker) UnregisterTarget(id string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	delete(cb.targets, id)
}

func (cb *CircuitBreaker) Start() {
	go cb.probeLoop()
}

func (cb *CircuitBreaker) Stop() {
	close(cb.stopCh)
}

func (cb *CircuitBreaker) probeLoop() {
	for {
		select {
		case <-cb.stopCh:
			return
		default:
			cb.probeAll()
			time.Sleep(cb.checkInterval)
		}
	}
}

func (cb *CircuitBreaker) probeAll() {
	cb.mu.RLock()
	targets := make([]*TargetHealth, 0, len(cb.targets))
	for _, th := range cb.targets {
		targets = append(targets, th)
	}
	cb.mu.RUnlock()

	for _, th := range targets {
		result := cb.probeTarget(th.Target)
		cb.recordResult(th.Target.ID, result)
	}
}

func (cb *CircuitBreaker) probeTarget(target ProbeTarget) ProbeResult {
	start := time.Now()
	result := ProbeResult{
		Target:    target,
		Timestamp: time.Now(),
	}

	// TCP check
	addr := fmt.Sprintf("%s:%d", target.Host, target.Port)
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: CBDialTimeout}, "tcp", addr, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		result.Error = fmt.Sprintf("TCP/TLS dial failed: %v", err)
		result.ResponseTime = time.Since(start)
		return result
	}
	conn.Close()
	result.TCPOK = true
	result.TLSOK = true

	// HTTP check
	if target.Scheme == "http" || target.Scheme == "https" {
		proto := target.Scheme
		if proto == "" {
			proto = "https"
		}
		url := fmt.Sprintf("%s://%s:%d/health", proto, target.Host, target.Port)
		client := &http.Client{Timeout: CBHealthCheckTimeout}
		resp, err := client.Get(url)
		if err == nil {
			result.HTTPStatus = resp.StatusCode
			resp.Body.Close()
		} else {
			result.Error = fmt.Sprintf("HTTP check failed: %v", err)
		}
	}

	result.ResponseTime = time.Since(start)

	// Detect burn: TCP RST or unexpected response patterns
	if result.Error != "" {
		result.Burned = true
	}

	return result
}

func (cb *CircuitBreaker) recordResult(targetID string, result ProbeResult) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	th, ok := cb.targets[targetID]
	if !ok {
		return
	}

	// Keep last 10 results
	th.Results = append(th.Results, result)
	if len(th.Results) > 10 {
		th.Results = th.Results[len(th.Results)-10:]
	}

	th.LastProbe = time.Now()

	if result.Error != "" {
		th.ConsecutiveFails++
		th.FailReasons = append(th.FailReasons, result.Error)
		if len(th.FailReasons) > 5 {
			th.FailReasons = th.FailReasons[len(th.FailReasons)-5:]
		}
	} else {
		th.ConsecutiveFails = 0
		th.FailReasons = nil
	}

	// Determine health status
	switch {
	case th.ConsecutiveFails >= 3:
		th.Status = HealthBurned
		if cb.onBurned != nil {
			go cb.onBurned(targetID)
		}
	case th.ConsecutiveFails >= 1:
		th.Status = HealthUnstable
	default:
		th.Status = HealthHealthy
	}
}

func (cb *CircuitBreaker) GetStatus(targetID string) (ListenerHealth, *TargetHealth) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	th, ok := cb.targets[targetID]
	if !ok {
		return HealthUnknown, nil
	}
	return th.Status, th
}

func (cb *CircuitBreaker) GetAllStatus() map[string]ListenerHealth {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	result := make(map[string]ListenerHealth)
	for id, th := range cb.targets {
		result[id] = th.Status
	}
	return result
}

func (cb *CircuitBreaker) SetOnBurnedHandler(handler func(targetID string)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onBurned = handler
}
