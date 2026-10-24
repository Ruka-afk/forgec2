package server

import (
	"encoding/json"
	"testing"
)

// unmarshalData decodes the canonical success envelope's data payload into
// out. Use it where a test previously unmarshalled the raw body into a struct
// holding sibling fields: those fields now live under "data".
func unmarshalData(t *testing.T, body []byte, out any) {
	t.Helper()
	var env struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode envelope: %v (%s)", err, body)
	}
	if !env.Success {
		t.Fatalf("envelope success = false (%s)", body)
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		t.Fatalf("decode envelope data: %v (%s)", err, body)
	}
}
