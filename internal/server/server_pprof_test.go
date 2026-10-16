package server

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
)

// TestValidatePprofAddrRefusesNonLoopback locks the security property the
// config comment promises: the profiling endpoints carry no authentication, so
// only a loopback bind is a valid configuration. Anything that would accept a
// remote connection is a startup error, not a warning.
func TestValidatePprofAddrRefusesNonLoopback(t *testing.T) {
	cases := []struct {
		addr string
		ok   bool
	}{
		{"127.0.0.1:6060", true},
		{"127.0.0.1:0", true},
		{"localhost:6060", true},
		{"[::1]:6060", true},
		// Missing host binds every interface.
		{":6060", false},
		{"[::]:6060", false},
		{"0.0.0.0:6060", false},
		// A routable address is not a loopback address, whatever it resolves to.
		{"10.0.0.5:6060", false},
		{"c2.example.com:6060", false},
		// Malformed values must not slip through as "empty == disabled".
		{"127.0.0.1", false},
		{"127.0.0.1:", false},
		{"", false},
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

// TestStartPprofRefusesRoutableAddress proves the refusal happens before any
// socket work: a rejected configuration must not leave a listener bound.
func TestStartPprofRefusesRoutableAddress(t *testing.T) {
	s := &Server{cfg: &config.Config{}}
	s.cfg.Server.PprofAddr = "0.0.0.0:6060"
	if err := s.startPprof(); err == nil {
		s.stopPprof()
		t.Fatal("startPprof accepted a routable bind address")
	}
	if s.pprofSrv != nil || s.pprofAddr != "" {
		t.Fatalf("startPprof left state behind after a rejected address: srv=%v addr=%q", s.pprofSrv, s.pprofAddr)
	}
}

// TestPprofListenerServesOnlyDebugEndpoints starts the profiling listener on
// an ephemeral loopback port and pins both halves of the contract: the pprof
// index answers, and the operator API does not exist on that socket.
func TestPprofListenerServesOnlyDebugEndpoints(t *testing.T) {
	s := &Server{cfg: &config.Config{}}
	s.cfg.Server.PprofAddr = "127.0.0.1:0"
	if err := s.startPprof(); err != nil {
		t.Fatalf("startPprof: %v", err)
	}
	addr := s.pprofAddr
	if addr == "" {
		s.stopPprof()
		t.Fatal("startPprof did not record the bound address")
	}
	// stopPprof runs on both paths; the second call is a no-op by contract.
	defer s.stopPprof()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://" + addr + "/debug/pprof/")
	if err != nil {
		t.Fatalf("GET /debug/pprof/: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /debug/pprof/ status=%d body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "goroutine") {
		t.Fatalf("pprof index did not look like the profiler index: %s", body[:min(len(body), 200)])
	}

	// The operator API and the SPA live on the main listener; this socket must
	// not answer for them even though it belongs to the same process.
	for _, path := range []string{"/api/v1/agents", "/health", "/"} {
		apiResp, err := client.Get("http://" + addr + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		apiResp.Body.Close()
		if apiResp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s on the profiling listener status=%d, want 404", path, apiResp.StatusCode)
		}
	}

	// Disabling profiling must actually release the socket: stopPprof is what
	// shutdown calls, so a listener that survives it is a leaked port.
	s.stopPprof()
	if _, err := client.Get("http://" + addr + "/debug/pprof/"); err == nil {
		t.Error("the profiling listener still answered after stopPprof")
	}
}
