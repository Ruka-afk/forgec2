package protocol

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// uuidv7Nibbles is unused; kept only as documentation of the RFC 9562 layout.
const _ = "timestamp(48) version7 rand(12) variant10 rand(62)"

// UUIDv7 returns a time-ordered, sortable RFC 9562 UUID (version 7, variant
// 10) built from the current Unix millisecond timestamp plus 74 random bits.
// It is used for agent-generated result ids and other identifiers that benefit
// from being orderable while staying unpredictable.
func UUIDv7() string {
	var b [16]byte
	if _, err := rand.Read(b[6:]); err != nil {
		// crypto/rand failing is unrecoverable for an id; still produce a
		// version-7-shaped value from the timestamp with zeroed rand bits.
	}
	setUUIDv7Timestamp(b[:], time.Now().UnixMilli())
	return formatUUIDv7(b)
}

// setUUIDv7 lays out the 48-bit unix-millis timestamp and nails the version
// and variant nibbles into an in-progress 16-byte uuid buffer.
func setUUIDv7Timestamp(b []byte, ms int64) {
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	b[6] = (b[6] & 0x0f) | 0x70 // version 7
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
}

func formatUUIDv7(b [16]byte) string {
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}
