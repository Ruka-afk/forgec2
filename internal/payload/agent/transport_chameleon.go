//go:build chameleon

package main

import (
	"crypto/rand"
	"math/big"
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
)

var (
	ja3ProfileName    string
	ja3RotateInterval time.Duration
	ja3LastRotate     time.Time
	ja3Mu             sync.RWMutex
)

type ja3ProfileEntry struct {
	Name string
	ID   utls.ClientHelloID
}

var ja3Profiles = []ja3ProfileEntry{
	{"chrome_120", utls.HelloChrome_120},
	{"chrome_120_pq", utls.HelloChrome_120_PQ},
	{"chrome_auto", utls.HelloChrome_Auto},
	{"firefox_120", utls.HelloFirefox_120},
	{"firefox_auto", utls.HelloFirefox_Auto},
	{"safari_16", utls.HelloSafari_16_0},
	{"safari_auto", utls.HelloSafari_Auto},
	{"edge_85", utls.HelloEdge_85},
	{"edge_auto", utls.HelloEdge_Auto},
	{"ios_14", utls.HelloIOS_14},
	{"ios_auto", utls.HelloIOS_Auto},
	{"android_11", utls.HelloAndroid_11_OkHttp},
	{"360_auto", utls.Hello360_Auto},
	{"qq_auto", utls.HelloQQ_Auto},
	{"randomized", utls.HelloRandomized},
	{"randomized_alpn", utls.HelloRandomizedALPN},
}

func initJA3Rotator() {
	n, _ := rand.Int(rand.Reader, big.NewInt(1440))
	ja3RotateInterval = time.Duration(12*60+int(n.Int64())) * time.Minute
	ja3LastRotate = time.Now()
	ja3ProfileName = chameleonProfile
	if ja3ProfileName == "" {
		ja3ProfileName = "random"
	}
	// Route all TLS transports (HTTPS/WSS/mTLS/gRPC/DoT) through the utls
	// dialer with the rotated JA3 profile selected here.
	ja3HelloProvider = func() utls.ClientHelloID {
		return getUTLSHelloID(getJA3Profile())
	}
}

func getJA3Profile() string {
	ja3Mu.RLock()
	base := ja3ProfileName
	ja3Mu.RUnlock()

	// In random mode, pick a fresh ClientHello on every dial so the JA3
	// fingerprint varies per connection (instead of being pinned for 12-36h,
	// which is itself a high-fidelity static IOC). Fixed profiles stay stable.
	if base == "random" || base == "randomized" {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(ja3Profiles))))
		return ja3Profiles[n.Int64()].Name
	}
	return base
}

func getUTLSHelloID(profile string) utls.ClientHelloID {
	if profile == "random" || profile == "" {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(ja3Profiles))))
		return ja3Profiles[n.Int64()].ID
	}
	for _, p := range ja3Profiles {
		if p.Name == profile {
			return p.ID
		}
	}
	return utls.HelloChrome_Auto
}
