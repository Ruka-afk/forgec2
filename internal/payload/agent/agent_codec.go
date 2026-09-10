package main

import (
	"crypto/rand"
	"encoding/binary"
	mathRand "math/rand"

	"github.com/forgec2/forgec2/pkg/encoding"
)

// encodeBeacon marshals a BeaconRequest using multi-format rotation.
func encodeBeacon(v any) ([]byte, error) {
	return encoding.Marshal(v)
}

// decodeBeacon unmarshals a response into BeaconResponse using multi-format detection.
func decodeBeacon(data []byte, v any) error {
	return encoding.Unmarshal(data, v)
}

func (l *lockedRand) Intn(n int) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.Intn(n)
}

func (l *lockedRand) Int63n(n int64) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.Int63n(n)
}

func (l *lockedRand) Float64() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.Float64()
}

func (l *lockedRand) Uint32() uint32 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.Uint32()
}

func (l *lockedRand) Uint64() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.Uint64()
}

func newCryptoRand() *lockedRand {
	seed := make([]byte, 8)
	rand.Read(seed)
	src := mathRand.NewSource(int64(binary.LittleEndian.Uint64(seed)))
	return &lockedRand{r: mathRand.New(src)}
}
