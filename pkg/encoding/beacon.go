// Package encoding provides multi-format beacon serialization with automatic
// format rotation for defeating signature-based detection.
package encoding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync/atomic"

	"github.com/fxamacker/cbor/v2"
	"github.com/vmihailenco/msgpack/v5"
)

// Format identifiers (first byte of every encoded beacon payload)
const (
	FormatJSON byte = iota
	FormatCBOR
	FormatMsgpack
	FormatMax // sentinel — number of supported formats
)

// formatNames maps format IDs to human-readable names
var formatNames = map[byte]string{
	FormatJSON:    "json",
	FormatCBOR:    "cbor",
	FormatMsgpack: "msgpack",
}

var (
	lastFormat atomic.Uint32 // last-used format for rotation
)

// randomFormat selects a format, cycling through to avoid repeating the same format.
func randomFormat() byte {
	last := lastFormat.Load()
	// Pick next format, skipping the last one used (rotation reduces repeat)
	for {
		f := byte(rand.Intn(int(FormatMax)))
		if f != byte(last) {
			lastFormat.Store(uint32(f))
			return f
		}
	}
}

// Marshal encodes v using a randomly selected format, prepending a format byte.
// Returns: [format_byte][encoded_data].
func Marshal(v any) ([]byte, error) {
	fmtID := randomFormat()
	var encoded []byte
	var err error

	switch fmtID {
	case FormatJSON:
		encoded, err = json.Marshal(v)
	case FormatCBOR:
		encoded, err = cbor.Marshal(v)
	case FormatMsgpack:
		encoded, err = msgpack.Marshal(v)
	default:
		encoded, err = json.Marshal(v)
		fmtID = FormatJSON
	}
	if err != nil {
		return nil, err
	}
	// Prepend format byte
	out := make([]byte, 1, 1+len(encoded))
	out[0] = fmtID
	out = append(out, encoded...)
	return out, nil
}

// MarshalFormat encodes v using the specified format, prepending a format byte.
func MarshalFormat(v any, fmtID byte) ([]byte, error) {
	var encoded []byte
	var err error

	switch fmtID {
	case FormatJSON:
		encoded, err = json.Marshal(v)
	case FormatCBOR:
		encoded, err = cbor.Marshal(v)
	case FormatMsgpack:
		encoded, err = msgpack.Marshal(v)
	default:
		encoded, err = json.Marshal(v)
		fmtID = FormatJSON
	}
	if err != nil {
		return nil, err
	}
	out := make([]byte, 1, 1+len(encoded))
	out[0] = fmtID
	out = append(out, encoded...)
	return out, nil
}

// Unmarshal decodes data as produced by Marshal/MarshalFormat.
// If the first byte is a known format identifier (0x00-0x02), it decodes accordingly.
// For backward compatibility, if the first byte is NOT a known format ID (e.g. plain JSON starting with '{'),
// the entire data is treated as JSON.
func Unmarshal(data []byte, v any) error {
	if len(data) < 1 {
		return fmt.Errorf("encoding: empty data")
	}
	fmtID := data[0]
	// Only interpret the first byte as a format marker if it's a known format ID.
	// Plain JSON starting with '{' (0x7B) or '[' (0x5B) is handled by the default branch.
	if fmtID < FormatMax {
		payload := data[1:]
		switch fmtID {
		case FormatJSON:
			// JSON with format marker: strip marker and decode
			if len(payload) == 0 {
				return fmt.Errorf("encoding: empty payload")
			}
			return decodeJSON(payload, v)
		case FormatCBOR:
			return cbor.Unmarshal(payload, v)
		case FormatMsgpack:
			return msgpack.Unmarshal(payload, v)
		}
	}
	// Backward compatibility: treat as plain JSON
	return decodeJSON(data, v)
}

// FormatName returns the human-readable name for a format byte.
func FormatName(fmtID byte) string {
	if name, ok := formatNames[fmtID]; ok {
		return name
	}
	return fmt.Sprintf("unknown(0x%02x)", fmtID)
}

// decodeJSON decodes the first JSON value from data, ignoring any trailing
// bytes. The agent appends random padding after the JSON payload
// (see applyTrafficShaping), so a strict json.Unmarshal on the whole slice
// would fail. json.Decoder stops after the first complete value.
func decodeJSON(data []byte, v any) error {
	return json.NewDecoder(bytes.NewReader(data)).Decode(v)
}
