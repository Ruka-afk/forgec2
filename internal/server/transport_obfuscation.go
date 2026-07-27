package server

import (
	"crypto/rand"
	"log/slog"
	"math/big"
	"sync"
	"time"
)

type DNSTransportObfuscator struct {
	mu            sync.RWMutex
	domainFlux    bool
	subdomainPool []string
	domainList    []string
	currentDomain int
	rotateTimer   *time.Ticker
	fakeQueryRate float64
	jitterMax     int
	stopCh        chan struct{}
}

type ICMPTransportObfuscator struct {
	mu             sync.RWMutex
	payloadSize    int
	timingJitter   int
	decoyRate      float64
	currentPattern int
	stopCh         chan struct{}
}

type TransportObfuscationManager struct {
	dns  *DNSTransportObfuscator
	icmp *ICMPTransportObfuscator
}

func NewTransportObfuscationManager() *TransportObfuscationManager {
	return &TransportObfuscationManager{
		dns:  NewDNSTransportObfuscator(),
		icmp: NewICMPTransportObfuscator(),
	}
}

func NewDNSTransportObfuscator() *DNSTransportObfuscator {
	return &DNSTransportObfuscator{
		domainFlux:    true,
		subdomainPool: []string{"cdn", "assets", "static", "img", "media", "api", "v2", "ws", "live", "edge"},
		fakeQueryRate: 0.3,
		jitterMax:     5000,
		stopCh:        make(chan struct{}),
	}
}

func NewICMPTransportObfuscator() *ICMPTransportObfuscator {
	return &ICMPTransportObfuscator{
		payloadSize:  64,
		timingJitter: 2000,
		decoyRate:    0.2,
		stopCh:       make(chan struct{}),
	}
}

func (dom *TransportObfuscationManager) Start() {
	dom.dns.Start()
	dom.icmp.Start()
	slog.Info("Transport obfuscation started (DNS + ICMP)")
}

func (dom *TransportObfuscationManager) Stop() {
	dom.dns.Stop()
	dom.icmp.Stop()
}

func (dto *DNSTransportObfuscator) Start() {
	dto.rotateTimer = time.NewTicker(5 * time.Minute)
	go func() {
		for {
			select {
			case <-dto.stopCh:
				return
			case <-dto.rotateTimer.C:
				dto.rotateDomain()
			}
		}
	}()
}

func (dto *DNSTransportObfuscator) Stop() {
	if dto.rotateTimer != nil {
		dto.rotateTimer.Stop()
	}
	close(dto.stopCh)
}

func (dto *DNSTransportObfuscator) rotateDomain() {
	dto.mu.Lock()
	defer dto.mu.Unlock()
	if len(dto.domainList) > 1 {
		prev := dto.currentDomain
		for dto.currentDomain == prev {
			dto.currentDomain = int(fastrandn(uint32(len(dto.domainList))))
		}
		slog.Debug("DNS domain rotated", "domain", dto.domainList[dto.currentDomain])
	}
}

func (dto *DNSTransportObfuscator) GetCurrentDomain() string {
	dto.mu.RLock()
	defer dto.mu.RUnlock()
	if len(dto.domainList) == 0 {
		return ""
	}
	return dto.domainList[dto.currentDomain]
}

func (dto *DNSTransportObfuscator) GenerateSubdomain() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(dto.subdomainPool))))
	prefix := dto.subdomainPool[n.Int64()]
	randBytes := make([]byte, 8)
	rand.Read(randBytes)
	return prefix + "-" + hexEncode(randBytes)
}

func (dto *DNSTransportObfuscator) ShouldSendFakeQuery() bool {
	r, _ := rand.Int(rand.Reader, big.NewInt(1000))
	return r.Int64() < int64(dto.fakeQueryRate*1000)
}

func (dto *DNSTransportObfuscator) GetJitter() time.Duration {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(dto.jitterMax)))
	return time.Duration(n.Int64()) * time.Millisecond
}

func (dto *DNSTransportObfuscator) SetDomains(domains []string) {
	dto.mu.Lock()
	defer dto.mu.Unlock()
	dto.domainList = domains
	dto.currentDomain = 0
}

func (ico *ICMPTransportObfuscator) Start() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ico.stopCh:
				return
			case <-ticker.C:
				ico.mu.Lock()
				ico.currentPattern = (ico.currentPattern + 1) % 3
				ico.mu.Unlock()
			}
		}
	}()
}

func (ico *ICMPTransportObfuscator) Stop() {
	close(ico.stopCh)
}

func (ico *ICMPTransportObfuscator) GetPayloadSize() int {
	ico.mu.RLock()
	defer ico.mu.RUnlock()
	sizes := []int{48, 64, 128}
	return sizes[ico.currentPattern]
}

func (ico *ICMPTransportObfuscator) GetTimingJitter() time.Duration {
	ico.mu.RLock()
	defer ico.mu.RUnlock()
	jitters := []int{1000, 2000, 3500}
	return time.Duration(jitters[ico.currentPattern]) * time.Millisecond
}

func (ico *ICMPTransportObfuscator) ShouldSendDecoy() bool {
	r, _ := rand.Int(rand.Reader, big.NewInt(1000))
	return r.Int64() < int64(ico.decoyRate*1000)
}

func (ico *ICMPTransportObfuscator) GenerateDecoyPayload() []byte {
	size := ico.GetPayloadSize()
	payload := make([]byte, size)
	rand.Read(payload)
	return payload
}

func hexEncode(data []byte) string {
	const hexTable = "0123456789abcdef"
	result := make([]byte, len(data)*2)
	for i, b := range data {
		result[i*2] = hexTable[b>>4]
		result[i*2+1] = hexTable[b&0x0f]
	}
	return string(result)
}

func fastrandn(n uint32) uint32 {
	return uint32(time.Now().UnixNano()) % n
}
