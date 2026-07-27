package server

import "testing"

func TestClampIntervalJitter_Defaults(t *testing.T) {
	interval, jitter := clampIntervalJitter(0, 0, 0)

	if interval != 5 {
		t.Fatalf("interval = %d, want 5 (default)", interval)
	}
	if jitter != 0 {
		t.Fatalf("jitter = %d, want 0", jitter)
	}
}

func TestClampIntervalJitter_BeaconTime(t *testing.T) {
	interval, jitter := clampIntervalJitter(5, 20, 30)

	if interval != 30 {
		t.Fatalf("interval = %d, want 30 (from beaconTime)", interval)
	}
	if jitter != 20 {
		t.Fatalf("jitter = %d, want 20", jitter)
	}
}

func TestClampIntervalJitter_Range(t *testing.T) {
	tests := []struct {
		name         string
		interval     int
		jitter       int
		beaconTime   int
		wantInterval int
		wantJitter   int
	}{
		{"negative interval clamped to 5", -1, 50, 0, 5, 50},
		{"interval capped at 86400", 99999, 50, 0, 86400, 50},
		{"negative jitter clamped to 0", 10, -10, 0, 10, 0},
		{"jitter capped at 100", 10, 200, 0, 10, 100},
		{"valid range unchanged", 15, 50, 0, 15, 50},
		{"beaconTime overrides interval", 5, 10, 60, 60, 10},
		{"beaconTime overrides interval even if interval valid", 100, 30, 120, 120, 30},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotInterval, gotJitter := clampIntervalJitter(tc.interval, tc.jitter, tc.beaconTime)
			if gotInterval != tc.wantInterval {
				t.Errorf("interval = %d, want %d", gotInterval, tc.wantInterval)
			}
			if gotJitter != tc.wantJitter {
				t.Errorf("jitter = %d, want %d", gotJitter, tc.wantJitter)
			}
		})
	}
}

func TestParseArchitecture_Default(t *testing.T) {
	got := parseArchitecture("")
	if got != "amd64" {
		t.Fatalf("parseArchitecture(\"\") = %q, want \"amd64\"", got)
	}
}

func TestParseArchitecture_Arm64(t *testing.T) {
	got := parseArchitecture("arm64")
	if got != "arm64" {
		t.Fatalf("parseArchitecture(\"arm64\") = %q, want \"arm64\"", got)
	}
}

func TestParseArchitecture_Whitespace(t *testing.T) {
	got := parseArchitecture("  amd64  ")
	if got != "amd64" {
		t.Fatalf("parseArchitecture(\"  amd64  \") = %q, want \"amd64\"", got)
	}
}
