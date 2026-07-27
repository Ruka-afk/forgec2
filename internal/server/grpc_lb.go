package server

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ── Load Balancing Configuration ────────────────────────────────────────────

type LBStrategy string

const (
	LBRoundRobin LBStrategy = "round_robin"
	LBPickFirst   LBStrategy = "pick_first"
	LBWeighted    LBStrategy = "weighted"
)

type GRPCLBConfig struct {
	Strategy         LBStrategy `yaml:"strategy" json:"strategy"`
	MaxConnections   int        `yaml:"max_connections" json:"max_connections"`
	Weight           int        `yaml:"weight" json:"weight"`
	MetricsEnabled   bool       `yaml:"metrics_enabled" json:"metrics_enabled"`
	HealthyThreshold int        `yaml:"healthy_threshold" json:"healthy_threshold"`
}

func DefaultLBConfig() GRPCLBConfig {
	return GRPCLBConfig{
		Strategy:         LBRoundRobin,
		MaxConnections:   1000,
		Weight:           1,
		MetricsEnabled:   true,
		HealthyThreshold: 3,
	}
}

// ── Per-Method gRPC Metrics ─────────────────────────────────────────────────

type GRPCMethodMetrics struct {
	RequestCount    atomic.Int64
	ErrorCount      atomic.Int64
	TotalLatencyNs  atomic.Int64
	ActiveRequests  atomic.Int64
	LastRequestUnix atomic.Int64
}

func (m *GRPCMethodMetrics) record(latency time.Duration, isErr bool) {
	m.RequestCount.Add(1)
	m.TotalLatencyNs.Add(int64(latency))
	m.LastRequestUnix.Store(time.Now().Unix())
	if isErr {
		m.ErrorCount.Add(1)
	}
}

func (m *GRPCMethodMetrics) snapshot() (count int64, errors int64, avgLatency time.Duration) {
	c := m.RequestCount.Load()
	e := m.ErrorCount.Load()
	t := m.TotalLatencyNs.Load()
	if c > 0 {
		return c, e, time.Duration(t / c)
	}
	return c, e, 0
}

type GRPCMetricsCollector struct {
	mu      sync.RWMutex
	methods map[string]*GRPCMethodMetrics
}

func NewGRPCMetricsCollector() *GRPCMetricsCollector {
	return &GRPCMetricsCollector{
		methods: make(map[string]*GRPCMethodMetrics),
	}
}

func (mc *GRPCMetricsCollector) getOrCreate(method string) *GRPCMethodMetrics {
	mc.mu.RLock()
	m, ok := mc.methods[method]
	mc.mu.RUnlock()
	if ok {
		return m
	}
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if m, ok = mc.methods[method]; ok {
		return m
	}
	m = &GRPCMethodMetrics{}
	mc.methods[method] = m
	return m
}

func (mc *GRPCMetricsCollector) Snapshot() map[string][3]int64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	out := make(map[string][3]int64, len(mc.methods))
	for k, v := range mc.methods {
		count, errors, avg := v.snapshot()
		out[k] = [3]int64{count, errors, int64(avg)}
	}
	return out
}

func (mc *GRPCMetricsCollector) TotalRequests() int64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	var total int64
	for _, m := range mc.methods {
		total += m.RequestCount.Load()
	}
	return total
}

// ── Server Interceptors ─────────────────────────────────────────────────────

func MetricsUnaryInterceptor(collector *GRPCMetricsCollector) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		method := info.FullMethod
		m := collector.getOrCreate(method)

		m.ActiveRequests.Add(1)
		start := time.Now()
		resp, err := handler(ctx, req)
		elapsed := time.Since(start)
		m.ActiveRequests.Add(-1)

		m.record(elapsed, err != nil)

		if elapsed > 5*time.Second {
			slog.Warn("slow gRPC unary", "method", method, "latency", elapsed)
		}

		return resp, err
	}
}

func MetricsStreamInterceptor(collector *GRPCMetricsCollector) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		method := info.FullMethod
		m := collector.getOrCreate(method)

		m.ActiveRequests.Add(1)
		start := time.Now()
		err := handler(srv, ss)
		elapsed := time.Since(start)
		m.ActiveRequests.Add(-1)

		m.record(elapsed, err != nil)
		return err
	}
}

// ── Connection Balancer ─────────────────────────────────────────────────────

type connEntry struct {
	id        uint64
	createdAt time.Time
	weight    int
	active    bool
}

type ConnectionBalancer struct {
	cfg       GRPCLBConfig
	mu        sync.RWMutex
	conns     map[string]*connEntry
	counter   atomic.Uint64
	collector *GRPCMetricsCollector
}

func NewConnectionBalancer(cfg GRPCLBConfig) *ConnectionBalancer {
	if cfg.MaxConnections <= 0 {
		cfg.MaxConnections = 1000
	}
	if cfg.Weight <= 0 {
		cfg.Weight = 1
	}
	return &ConnectionBalancer{
		cfg:       cfg,
		conns:     make(map[string]*connEntry),
		collector: NewGRPCMetricsCollector(),
	}
}

func (cb *ConnectionBalancer) Accept(addr string) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.cfg.MaxConnections > 0 && len(cb.conns) >= cb.cfg.MaxConnections {
		slog.Warn("gRPC LB at capacity",
			"active", len(cb.conns),
			"max", cb.cfg.MaxConnections,
			"rejected", addr,
		)
		return false
	}

	id := cb.counter.Add(1)
	cb.conns[addr] = &connEntry{
		id:        id,
		createdAt: time.Now(),
		weight:    cb.cfg.Weight,
		active:    true,
	}
	slog.Debug("gRPC LB accept", "addr", addr, "id", id, "active", len(cb.conns))
	return true
}

func (cb *ConnectionBalancer) Release(addr string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	delete(cb.conns, addr)
}

func (cb *ConnectionBalancer) ActiveCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return len(cb.conns)
}

func (cb *ConnectionBalancer) Metrics() *GRPCMetricsCollector {
	return cb.collector
}

func (cb *ConnectionBalancer) Strategy() LBStrategy {
	return cb.cfg.Strategy
}

func (cb *ConnectionBalancer) Select() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if len(cb.conns) == 0 {
		return ""
	}

	switch cb.cfg.Strategy {
	case LBRoundRobin:
		return cb.selectRoundRobin()
	case LBWeighted:
		return cb.selectWeighted()
	default:
		return cb.selectPickFirst()
	}
}

func (cb *ConnectionBalancer) selectRoundRobin() string {
	idx := cb.counter.Add(1)
	var keys []string
	for k, v := range cb.conns {
		if v.active {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	return keys[idx%uint64(len(keys))]
}

func (cb *ConnectionBalancer) selectWeighted() string {
	type candidate struct {
		addr string
		w    int
	}
	var total int
	var candidates []candidate
	for k, v := range cb.conns {
		if v.active && v.weight > 0 {
			total += v.weight
			candidates = append(candidates, candidate{addr: k, w: v.weight})
		}
	}
	if len(candidates) == 0 {
		return ""
	}

	roll := int(cb.counter.Add(1)) % total
	for _, c := range candidates {
		roll -= c.w
		if roll < 0 {
			return c.addr
		}
	}
	return candidates[len(candidates)-1].addr
}

func (cb *ConnectionBalancer) selectPickFirst() string {
	for k, v := range cb.conns {
		if v.active {
			return k
		}
	}
	return ""
}

// ── ServerOptions: drop-in integration with GRPCListener ────────────────────

// ServerOptions returns grpc.ServerOptions that install both the metrics
// interceptors and the connection-tracking interceptor into a gRPC server.
// Usage:
//
//	balancer := NewConnectionBalancer(cfg)
//	srv := grpc.NewServer(balancer.ServerOptions()...)
func (cb *ConnectionBalancer) ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			MetricsUnaryInterceptor(cb.collector),
			connectionTrackingUnary(cb),
		),
		grpc.ChainStreamInterceptor(
			MetricsStreamInterceptor(cb.collector),
		),
	}
}

func connectionTrackingUnary(cb *ConnectionBalancer) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if addrs := md.Get(":authority"); len(addrs) > 0 {
				if !cb.Accept(addrs[0]) {
					return nil, status.Errorf(codes.ResourceExhausted,
						"server at capacity (%d connections)", cb.cfg.MaxConnections)
				}
				defer cb.Release(addrs[0])
			}
		}
		return handler(ctx, req)
	}
}

// LogMetrics periodically logs aggregated gRPC metrics.
// Call in a goroutine; blocks until ctx is cancelled.
func (cb *ConnectionBalancer) LogMetrics(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap := cb.collector.Snapshot()
			active := cb.ActiveCount()
			slog.Info("gRPC LB metrics",
				"strategy", string(cb.cfg.Strategy),
				"active_conns", active,
				"methods", len(snap),
			)
			for method, stats := range snap {
				slog.Debug("gRPC method stats",
					"method", method,
					"requests", stats[0],
					"errors", stats[1],
					"avg_latency_ns", stats[2],
				)
			}
		}
	}
}
