package config

import "testing"

// TestValidatePprofAddr pins the config-load half of the loopback rule. The
// bind-time guard in internal/server refuses the same values again; this one
// makes a bad config.yaml fail before the server owns any socket.
func TestValidatePprofAddr(t *testing.T) {
	cases := []struct {
		addr string
		ok   bool
	}{
		{"", true}, // disabled
		{"127.0.0.1:6060", true},
		{"127.0.0.1:0", true},
		{"localhost:6060", true},
		{"[::1]:6060", true},
		{":6060", false},
		{"[::]:6060", false},
		{"0.0.0.0:6060", false},
		{"10.0.0.5:6060", false},
		{"c2.example.com:6060", false},
		{"127.0.0.1", false},
		{"127.0.0.1:", false},
		{"not an address", false},
	}
	for _, tc := range cases {
		err := validatePprofAddr(tc.addr)
		if tc.ok && err != nil {
			t.Errorf("validatePprofAddr(%q) = %v, want nil", tc.addr, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("validatePprofAddr(%q) = nil, want an error", tc.addr)
		}
	}
}
