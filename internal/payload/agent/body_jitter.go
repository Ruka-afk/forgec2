//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/binary"
	mathRand "math/rand"
)

// padBeaconBody implements the ContentLengthJitter option as REAL body
// padding. The envelope is prefixed with an 8-byte big-endian length and
// trailed with up to ContentLengthJitter random bytes, so the on-wire body
// length varies between beacons while the enclosed payload stays intact. The
// server strips the prefix via stripBodyPadding; bodies without the prefix
// (jitter disabled, non-HTTP/WS transports) pass through untouched.
func padBeaconBody(body []byte) []byte {
	if ContentLengthJitter <= 0 {
		return body
	}
	pad := mathRand.Intn(ContentLengthJitter + 1)
	out := make([]byte, 8, 8+len(body)+pad)
	binary.BigEndian.PutUint64(out, uint64(len(body)))
	out = append(out, body...)
	for i := 0; i < pad; i++ {
		out = append(out, byte(mathRand.Intn(256)))
	}
	return out
}
