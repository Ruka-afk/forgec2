package main

import (
	"net/http"
	"sync/atomic"
	"time"
)

// serverClockOffsetSeconds is the difference between the teamserver's clock
// and this host's clock, learned from the Date header of HTTP responses. Frames
// carry a Unix timestamp that the server accepts only within five minutes
// (beaconTSTolerance); a host whose clock is off by more — a fresh VM clone, a
// board without a working RTC, an offline laptop whose NTP has not converged —
// could otherwise never register, and would retry forever because nothing ever
// corrected the clock.
//
// The offset is recomputed from every response rather than accumulated, so a
// later NTP correction (or the operator fixing the time) brings it back to zero
// automatically. It is used only for frame timestamps, never for timeouts,
// scheduling or TLS.
var serverClockOffsetSeconds atomic.Int64

// noteServerClock records the offset implied by an HTTP Date header. It runs
// for every HTTP response, including rejected registrations: that rejection is
// exactly the case this exists to recover from. Missing or invalid headers
// leave the current offset untouched.
func noteServerClock(h http.Header) {
	raw := h.Get("Date")
	if raw == "" {
		return
	}
	server, err := http.ParseTime(raw)
	if err != nil {
		return
	}
	offset := server.Unix() - time.Now().Unix()
	// Date has one-second resolution and the response spent time in flight;
	// ignore sub-two-second noise so a correct clock stays at zero.
	if offset > -2 && offset < 2 {
		offset = 0
	}
	serverClockOffsetSeconds.Store(offset)
}

// beaconTimestamp is the timestamp stamped into registration, handshake and
// ciphertext frames. It follows the server clock once one has been learned.
func beaconTimestamp() int64 {
	return time.Now().Unix() + serverClockOffsetSeconds.Load()
}
