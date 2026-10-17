//go:build linux || windows || darwin

package main

import (
	"testing"

	utls "github.com/refraction-networking/utls"
)

func resetJA3State(t *testing.T) {
	t.Helper()
	prevProvider := ja3HelloProvider
	prevProfile := ja3ProfileName
	t.Cleanup(func() {
		ja3HelloProvider = prevProvider
		ja3ProfileName = prevProfile
	})
}

func TestInitJA3RotationDisabledKeepsChromeAuto(t *testing.T) {
	resetJA3State(t)
	initJA3Rotation("random", false)
	if ja3HelloProvider != nil {
		t.Fatal("provider must stay unregistered when rotation is disabled")
	}
	if got := utlsClientHello(); got != utls.HelloChrome_Auto {
		t.Fatalf("utlsClientHello() = %v, want HelloChrome_Auto", got)
	}
}

func TestInitJA3RotationFixedProfileIsStable(t *testing.T) {
	resetJA3State(t)
	initJA3Rotation("firefox", true)
	if ja3HelloProvider == nil {
		t.Fatal("provider not registered")
	}
	for i := 0; i < 5; i++ {
		if got := utlsClientHello(); got != utls.HelloFirefox_Auto {
			t.Fatalf("utlsClientHello() = %v, want HelloFirefox_Auto", got)
		}
	}
}

func TestInitJA3RotationRandomStaysInProfilePool(t *testing.T) {
	resetJA3State(t)
	initJA3Rotation("random", true)
	valid := map[utls.ClientHelloID]bool{}
	for _, p := range ja3Profiles {
		valid[p.ID] = true
	}
	seen := map[utls.ClientHelloID]bool{}
	for i := 0; i < 32; i++ {
		id := utlsClientHello()
		if !valid[id] {
			t.Fatalf("random rotation returned an ID outside the pool: %v", id)
		}
		seen[id] = true
	}
	if len(seen) < 2 {
		t.Fatal("random rotation drew the same profile 32 times")
	}
}

func TestGetUTLSHelloIDProfileNames(t *testing.T) {
	cases := []struct {
		profile string
		want    utls.ClientHelloID
	}{
		{"chrome", utls.HelloChrome_Auto},
		{"firefox", utls.HelloFirefox_Auto},
		{"safari", utls.HelloSafari_Auto},
		{"edge", utls.HelloEdge_Auto},
		{"ios", utls.HelloIOS_Auto},
		{"android", utls.HelloAndroid_11_OkHttp},
		{"360", utls.Hello360_Auto},
		{"qq", utls.HelloQQ_Auto},
		{"chrome_120", utls.HelloChrome_120},
		{"firefox_120", utls.HelloFirefox_120},
		{"SAFARI", utls.HelloSafari_Auto},
		{"unknown-browser", utls.HelloChrome_Auto},
	}
	for _, tc := range cases {
		if got := getUTLSHelloID(tc.profile); got != tc.want {
			t.Errorf("getUTLSHelloID(%q) = %v, want %v", tc.profile, got, tc.want)
		}
	}
}
