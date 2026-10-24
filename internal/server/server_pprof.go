package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"
)

// Profiling is opt-in and loopback-only by construction. The endpoints expose
// process internals (heap contents, goroutine stacks, a 30s CPU profile trace)
// and accept no authentication, so they are worth having exactly when an
// operator is measuring a loaded server — and never otherwise. Configuring a
// non-loopback address is refused at startup rather than downgraded to a
// warning: a profiler served on the C2's own beacon port would be a
// remote-triggerable DoS and an information leak.
//
// See docs/CAPACITY.md for the workflow these endpoints exist to serve.

// pprofMux returns the handler set served on the profiling listener. It is a
// fresh mux with only the /debug/pprof endpoints: operator routes, the SPA
// catch-all and every authenticated API are unreachable through it, so
// enabling profiling cannot widen the operator plane's attack surface.
func pprofMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}

// validatePprofAddr rejects anything but an explicit loopback host:port.
// "localhost" is accepted as the operator's spelling of 127.0.0.1 (net.Listen
// resolves it), but a missing host (":6060") or an unspecified one
// ("0.0.0.0:6060", "[::]:6060") is not.
func validatePprofAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("server.pprof_addr %q must be host:port: %w", addr, err)
	}
	if port == "" {
		return fmt.Errorf("server.pprof_addr %q is missing a port", addr)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	if host == "" {
		return fmt.Errorf("server.pprof_addr %q must name a loopback host (127.0.0.1 or ::1); an empty host binds every interface and would expose the profiler", addr)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("server.pprof_addr %q must be a loopback address (127.0.0.1 or ::1); the profiling endpoints have no authentication", addr)
	}
	return nil
}

// startPprof binds the opt-in profiling listener. Failures abort startup: a
// profile that silently is not there when an incident happens is worse than a
// server that refuses to boot with a clear error.
func (s *Server) startPprof() error { // config-lock: boot listener start, no reload can overlap yet
	addr := strings.TrimSpace(s.cfg.Server.PprofAddr)
	if addr == "" {
		return nil
	}
	if err := validatePprofAddr(addr); err != nil {
		return err
	}
	// Synchronous bind, like the extra HTTP listeners: a port conflict must
	// surface here, not in a background goroutine's log after "started".
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("binding pprof listener %s: %w", addr, err)
	}
	srv := &http.Server{
		Handler:           pprofMux(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	s.pprofMu.Lock()
	s.pprofSrv = srv
	s.pprofAddr = ln.Addr().String()
	s.pprofMu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if serveErr := srv.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			slog.Warn("pprof listener stopped", "addr", ln.Addr().String(), "error", serveErr)
		}
	}()
	slog.Info("Profiling endpoints enabled (loopback only)", "addr", ln.Addr().String(), "path", "/debug/pprof/")
	return nil
}

// stopPprof closes the profiling listener during shutdown. Safe to call when
// profiling was never enabled and more than once.
func (s *Server) stopPprof() {
	s.pprofMu.Lock()
	srv := s.pprofSrv
	s.pprofSrv = nil
	s.pprofAddr = ""
	s.pprofMu.Unlock()
	if srv != nil {
		_ = srv.Close()
	}
}
