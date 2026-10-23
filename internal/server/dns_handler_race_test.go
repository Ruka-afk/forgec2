package server

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

// s.dnsListener was read and swapped by the status poll, the start/stop
// handlers and shutdown with no lock, and handleDNSStart mutated the DNS
// config fields under cfg.Lock() while every other config reader used
// configMu — the two locks never excluded each other. The status poll on a
// running console (the DNS page polls /api/dns/status) therefore raced any
// start/stop. These tests pin the fix: the pointer now moves under
// dnsListenerMu and the config fields under configMu.

func TestDNSStatusPollRacesStartStop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := mustTenantServer(t)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request, _ = http.NewRequest(http.MethodGet, "/api/dns/status", nil)
				s.handleDNSStatus(c)
			}
		}()
	}

	// Swap the pointer the way start/stop do, without binding a real socket:
	// start is guarded by IsRunning(), so a constructed-but-not-started
	// listener exercises the same field.
	for i := 0; i < 200; i++ {
		s.dnsListenerMu.Lock()
		if i%2 == 0 {
			s.dnsListener = NewDNSBeaconListener("dns.example", "127.0.0.1", 0, ":0")
		} else {
			s.dnsListener = nil
		}
		s.dnsListenerMu.Unlock()
	}

	close(stop)
	wg.Wait()
}

func TestDNSConfigWritesRaceReaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := mustTenantServer(t)
	s.configPath = "" // no Save in this test path

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Reader: the DNS status endpoint snapshots the config under configMu.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request, _ = http.NewRequest(http.MethodGet, "/api/dns/status", nil)
				s.handleDNSStatus(c)
			}
		}()
	}

	// Writer: the same mutation handleDNSStart performs on the config.
	for i := 0; i < 200; i++ {
		s.configMu.Lock()
		s.cfg.Server.DNSDomain = "a.example"
		s.cfg.Server.DNSAddr = ":5300"
		s.cfg.Server.DNSEnabled = true
		s.configMu.Unlock()
	}

	close(stop)
	wg.Wait()
}
