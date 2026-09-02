//go:build linux || windows || darwin

package main

import "testing"

func TestParseScreenStreamSettingsBoundsAndQuality(t *testing.T) {
	tests := []struct {
		command      string
		wantInterval int
		wantQuality  int
	}{
		{"3,high", 3, 85},
		{"10,low", 10, 40},
		{"60,75", 60, 75},
		{"3000,high", 5, 85},
		{"0,medium", 5, 65},
		{"5,invalid", 5, 65},
	}
	for _, test := range tests {
		interval, quality := parseScreenStreamSettings(test.command)
		if interval != test.wantInterval || quality != test.wantQuality {
			t.Errorf("parseScreenStreamSettings(%q) = (%d,%d), want (%d,%d)", test.command, interval, quality, test.wantInterval, test.wantQuality)
		}
	}
}
