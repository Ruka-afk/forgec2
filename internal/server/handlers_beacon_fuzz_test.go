package server

import (
	"encoding/json"
	"strings"
	"testing"
)

// FuzzBeaconRequestParsing tests that arbitrary JSON input to beacon parsing
// never panics or causes unexpected behavior.
func FuzzBeaconRequestParsing(f *testing.F) {
	// Seed with known valid inputs
	f.Add([]byte(`{"uuid":"test-123","info":{"hostname":"srv01","ip":"10.0.0.1","os":"windows","arch":"amd64"}}`))
	f.Add([]byte(`{"uuid":"test-456","results":[{"task_id":1,"type":"shell","output":"hello"}],"acks":[1,2]}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"uuid":""}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`{"uuid":"a","ecdh_pub":"AAAA","c":"BBBB"}`))
	f.Add([]byte(`{"uuid":"evil","info":{"hostname":"` + strings.Repeat("A", 10000) + `"}}`))
	f.Add([]byte(`{"uuid":"x","results":[{"task_id":0,"type":"","output":"","error":""}]}`))
	f.Add([]byte(`{"uuid":"x","socks_data":[{"action":"connect","session_id":"s1","data":"dGVzdA=="}]}`))
	f.Add([]byte(`{"uuid":"x","relayed":[{"agent_id":"child-1","results":[]}]}`))
	// Fuzz with arbitrary bytes
	f.Fuzz(func(t *testing.T, data []byte) {
		// Should never panic regardless of input
		var envelope struct {
			UUID      string `json:"uuid"`
			ECDHPub   string `json:"ecdh_pub,omitempty"`
			CipherB64 string `json:"c,omitempty"`
		}
		_ = json.Unmarshal(data, &envelope)

		var req beaconRequest
		_ = json.Unmarshal(data, &req)

		// Verify sanitizeInfo handles any input
		if req.Info != nil {
			_ = sanitizeInfo(req.Info)
		}
	})
}

// FuzzSanitizeInfo tests that sanitizeInfo handles any map input safely.
func FuzzSanitizeInfo(f *testing.F) {
	f.Add("hostname", "test")
	f.Add("ip", "10.0.0.1")
	f.Add(strings.Repeat("x", 1000), strings.Repeat("y", 10000))
	f.Add("", "")
	f.Add("bad\x00key", "val\x00ue")
	f.Fuzz(func(t *testing.T, key, value string) {
		info := map[string]string{key: value}
		result := sanitizeInfo(info)
		// Should never panic, result should always be valid
		if result != nil {
			for k, v := range result {
				if len([]rune(v)) > maxInfoValueLen {
					t.Errorf("value too long: %d > %d", len([]rune(v)), maxInfoValueLen)
				}
				if !allowedInfoKeys[k] {
					t.Errorf("unexpected key passed through: %q", k)
				}
			}
		}
	})
}

// FuzzBeaconFingerprint tests that dedup fingerprint generation never panics.
func FuzzBeaconFingerprint(f *testing.F) {
	f.Add("test-agent", uint(3), uint(2))
	f.Add("", uint(0), uint(0))
	f.Add("a", uint(1), uint(100))
	f.Fuzz(func(t *testing.T, uuid string, numResults uint, numAcks uint) {
		req := beaconRequest{
			UUID:       uuid,
			AckTaskIDs: make([]uint, numAcks),
		}
		for i := range req.AckTaskIDs {
			req.AckTaskIDs[i] = uint(i + 1)
		}
		req.Results = make([]taskResult, numResults)
		fp := beaconFingerprint(req)
		// Fingerprint should be deterministic
		fp2 := beaconFingerprint(req)
		if fp != fp2 {
			t.Errorf("non-deterministic fingerprint: %q != %q", fp, fp2)
		}
	})
}

// FuzzDecodeBeaconIdentity tests identity decoding with arbitrary inputs.
func FuzzDecodeBeaconIdentity(f *testing.F) {
	f.Add([]byte(`{"hostname":"test","ip":"10.0.0.1","username":"admin"}`))
	f.Add([]byte(`{"encoding":"base64","hostname":"dGVzdA=="}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`{"hostname":"\u0000evil","ip":"","username":""}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		var info map[string]string
		_ = json.Unmarshal(data, &info)
		hostname, username, ip := decodeBeaconIdentity(info)
		// Should never panic; results should be strings
		_ = hostname
		_ = username
		_ = ip
	})
}
