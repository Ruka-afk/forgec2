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

var networkWakeOnce sync.Once

// startNetworkWakeMonitor begins watching local interface state. Safe to call
// more than once: only the first call starts the goroutine.
func startNetworkWakeMonitor() {
	networkWakeOnce.Do(func() {
		go monitorNetworkChanges(networkWakePollInterval, networkFingerprint, wakeBeaconLoop, nil)
	})
}

// monitorNetworkChanges wakes the agent whenever the fingerprint changes. An
// empty fingerprint (transient OS error) never counts as a change, so a failed
// query cannot cause a wake storm. The arguments are injected for testability;
// the real monitor passes the global beacon-loop wake and a nil stop channel
// (which blocks forever).
func monitorNetworkChanges(interval time.Duration, fingerprint func() string, wake func(), stop <-chan struct{}) {
	if interval <= 0 {
		interval = networkWakePollInterval
	}
	last := fingerprint()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		current := fingerprint()
		if current == "" || current == last {
			continue
		}
		last = current
		logDebugf("Network change detected, waking beacon loop")
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
