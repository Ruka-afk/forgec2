package server

import (
	"io"
	"net"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
)

func TestListenerOverlapsMainAddr(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     int
		mainHost string
		mainPort int
		want     bool
	}{
		// The regression that broke fresh Linux boots: the seeded default
		// listener (127.0.0.1) against a wildcard main bind (0.0.0.0).
		{"loopback row vs wildcard main", "127.0.0.1", 8000, "0.0.0.0", 8000, true},
		{"wildcard row vs loopback main", "0.0.0.0", 8000, "127.0.0.1", 8000, true},
		{"empty main host counts as wildcard", "127.0.0.1", 8000, "", 8000, true},
		{"ipv6 wildcard main", "127.0.0.1", 8000, "::", 8000, true},
		{"wildcard spellings overlap", "0.0.0.0", 8000, "::", 8000, true},
		{"localhost aliases loopback", "localhost", 8000, "127.0.0.1", 8000, true},
		{"identical loopback", "127.0.0.1", 8000, "127.0.0.1", 8000, true},
		{"identical concrete", "192.168.1.5", 8000, "192.168.1.5", 8000, true},
		{"ipv6 loopback with itself", "::1", 8000, "::1", 8000, true},
		{"loopback row vs concrete main", "127.0.0.1", 8000, "192.168.1.5", 8000, false},
		{"concrete row vs loopback main", "192.168.1.5", 8000, "127.0.0.1", 8000, false},
		{"distinct concrete addresses", "192.168.1.5", 8000, "192.168.1.6", 8000, false},
		{"ipv6 loopback distinct from ipv4 loopback", "::1", 8000, "127.0.0.1", 8000, false},
		{"different ports never overlap", "127.0.0.1", 8001, "0.0.0.0", 8000, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := listenerOverlapsMainAddr(tc.host, tc.port, tc.mainHost, tc.mainPort)
			if got != tc.want {
				t.Errorf("listenerOverlapsMainAddr(%q, %d, %q, %d) = %v, want %v",
					tc.host, tc.port, tc.mainHost, tc.mainPort, got, tc.want)
			}
		})
	}
}

// TestStartExtraListenersFromDB_SkipsSeededDefault reproduces the startup
// conflict seen in CI: EnsureDefaultListener stores the seeded default as
// 127.0.0.1:<port> while the main server binds 0.0.0.0:<port>. The restore
// pass must treat the row as already served instead of binding it, otherwise
// the extra listener wins the port and the main bind fails with
// "port 0.0.0.0:<port> is already in use".
func TestStartExtraListenersFromDB_SkipsSeededDefault(t *testing.T) {
	port := freeTCPPort(t)
	cfg := config.DefaultConfig()
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = port

	s := &Server{
		db:             testutil.SetupTestDB(t),
		cfg:            cfg,
		extraListeners: make(map[string]io.Closer),
	}
	defer closeExtraListeners(s)

	s.EnsureDefaultListener(cfg.Server.Host, cfg.Server.Port)

	var seeded db.Listener
	if err := s.db.First(&seeded).Error; err != nil {
		t.Fatalf("seeded default listener missing: %v", err)
	}
	if seeded.Host != "127.0.0.1" || seeded.Port != port {
		t.Fatalf("unexpected seed address %s:%d", seeded.Host, seeded.Port)
	}

	s.startExtraListenersFromDB()

	s.extraListenersMu.Lock()
	n := len(s.extraListeners)
	s.extraListenersMu.Unlock()
	if n != 0 {
		t.Fatalf("seeded default listener must not be restored as an extra listener (got %d)", n)
	}
}

// TestStartExtraListenersFromDB_RestoresNonConflictingRows guards the other
// direction: the overlap skip must not swallow listeners on other ports.
func TestStartExtraListenersFromDB_RestoresNonConflictingRows(t *testing.T) {
	port := freeTCPPort(t)
	other := freeTCPPort(t)
	for other == port {
		other = freeTCPPort(t)
	}

	cfg := config.DefaultConfig()
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = port

	s := &Server{
		db:             testutil.SetupTestDB(t),
		cfg:            cfg,
		extraListeners: make(map[string]io.Closer),
	}
	defer closeExtraListeners(s)

	s.EnsureDefaultListener(cfg.Server.Host, cfg.Server.Port)
	row := db.Listener{
		Name: "extra", Scheme: "http", Type: "http", Protocol: "http",
		Host: "127.0.0.1", Port: other, Enabled: true, Status: "stopped",
	}
	if err := s.db.Create(&row).Error; err != nil {
		t.Fatalf("create extra listener row: %v", err)
	}

	s.startExtraListenersFromDB()

	key := "http://127.0.0.1:" + itoa(other)
	s.extraListenersMu.Lock()
	_, ok := s.extraListeners[key]
	s.extraListenersMu.Unlock()
	if !ok {
		t.Fatalf("non-conflicting listener %s was not restored", key)
	}
}

// freeTCPPort reserves and immediately releases a loopback port so tests can
// talk about "some free port" without hard-coding one.
func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// closeExtraListeners releases any listener the restore pass actually bound
// so a failing assertion cannot leak an open socket into the test process.
func closeExtraListeners(s *Server) {
	s.extraListenersMu.Lock()
	defer s.extraListenersMu.Unlock()
	for key, closer := range s.extraListeners {
		_ = closer.Close()
		delete(s.extraListeners, key)
	}
}
