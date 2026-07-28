//go:build chameleon

package main

import (
	"context"
	"crypto/rand"
	"math/big"
	"net"
	"net/http"
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
	ja3RotateInterval = (12*60 + int(n.Int64())) * time.Minute
	ja3LastRotate = time.Now()
	ja3ProfileName = chameleonProfile
	if ja3ProfileName == "" {
		ja3ProfileName = "random"
	}
}

func getJA3Profile() string {
	ja3Mu.RLock()
	profile := ja3ProfileName
	interval := ja3RotateInterval
	lastRotate := ja3LastRotate
	ja3Mu.RUnlock()

	if interval > 0 && time.Since(lastRotate) > interval {
		ja3Mu.Lock()
		if time.Since(ja3LastRotate) > ja3RotateInterval {
			ja3LastRotate = time.Now()
			if ja3ProfileName == "random" || ja3ProfileName == "randomized" {
				n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(ja3Profiles))))
				profile = ja3Profiles[n.Int64()].Name
				ja3ProfileName = profile
			}
		} else {
			profile = ja3ProfileName
		}
		ja3Mu.Unlock()
	}

	return profile
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

func utlsDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	d := net.Dialer{Timeout: 15 * time.Second}
	rawConn, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	serverName := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		serverName = h
	}
	if DomainFront != "" {
		serverName = DomainFront
	}

	profile := getJA3Profile()
	helloID := getUTLSHelloID(profile)

	uconn := utls.UClient(rawConn, &utls.Config{
		InsecureSkipVerify: SkipTLSVerify,
		ServerName:         serverName,
	}, helloID)
	if len(pinnedCertSHA256) > 0 {
		uconn.Config.VerifyPeerCertificate = verifyPinnedCert
	}

	if err := uconn.Handshake(); err != nil {
		rawConn.Close()
		return nil, err
	}
	return uconn, nil
}

func newUTLSTransport() *http.Transport {
	tr := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     60 * time.Second,
	}
	tr.DialTLSContext = utlsDialContext
	return tr
}
