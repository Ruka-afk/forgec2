package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// spaFS wraps http.FileSystem to serve index.html for SPA client-side routes.
type spaFS struct {
	inner http.FileSystem
}

func (s *spaFS) Open(name string) (http.File, error) {
	f, err := s.inner.Open(name)
	if err != nil {
		return s.inner.Open("index.html")
	}
	// Next.js static export emits both a `<route>/` directory (RSC chunks) and a
	// sibling `<route>.html` page. embed.FS.Open("<route>") resolves to the
	// directory, so serve the sibling .html page, not the root index.html —
	// otherwise /login and /dashboard boot the root Home page, which redirects to
	// /dashboard -> 401 -> /login, an infinite redirect loop.
	if fi, err := f.Stat(); err == nil && fi.IsDir() && name != "" && name != "/" {
		f.Close()
		if hf, herr := s.inner.Open(name + ".html"); herr == nil {
			return hf, nil
		}
		return s.inner.Open("index.html")
	}
	return f, nil
}

// SetStaticFS configures the server to serve embedded frontend static files.
// Must be called BEFORE SetupRoutes() so the middleware runs before route handlers.
func (s *Server) SetStaticFS(staticFS fs.FS) {
	s.staticFS = staticFS

	// http.FileServer with SPA fallback
	fsrv := http.FileServer(&spaFS{http.FS(staticFS)})

	s.router.Use(func(c *gin.Context) {
		if c.Request.Method != "GET" && c.Request.Method != "HEAD" {
			c.Next()
			return
		}
		path := c.Request.URL.Path

		// Never intercept API, WS, beacon, health, extc2, admin, or JSON (XHR) requests.
		// NOTE /generate_204 and /th are beacon endpoints: the SPA middleware
		// previously swallowed every unmatched GET (malleable profile URIs
		// included), feeding implants HTML instead of task replies and killing
		// all GET-based C2 channels (P1).
		if strings.HasPrefix(path, "/api") ||
			strings.HasPrefix(path, "/ws") ||
			strings.HasPrefix(path, "/extc2") ||
			strings.HasPrefix(path, "/admin/") ||
			strings.HasPrefix(path, "/debug/") ||
			strings.HasPrefix(path, "/rd/") ||
			strings.Contains(c.GetHeader("Accept"), "application/json") ||
			c.GetHeader("X-Requested-With") == "XMLHttpRequest" ||
			c.GetHeader("X-CSRF-Token") != "" ||
			path == "/th" || path == "/generate_204" || path == "/health" ||
			path == "/ready" || path == "/metrics" ||
			strings.HasPrefix(path, "/payloads/") ||
			strings.HasPrefix(path, "/stage/") ||
			strings.HasPrefix(path, "/screenshots/") {
			c.Next()
			return
		}

		// Malleable-profile safety net (P1): with a malleable profile active,
		// any GET that is NOT a real SPA asset and NOT browser navigation
		// (Accept: text/html) can only be a profile-defined beacon URI — it
		// must fall through to the router so the NoRoute beacon handler sees
		// it. Previously every unmatched GET was fed index.html, silently
		// killing ALL GET-based C2 channels while malleable mode was on.
		if s.malleableEnabled() && !spaAssetExists(staticFS, path) &&
			!strings.Contains(c.GetHeader("Accept"), "text/html") {
			c.Next()
			return
		}

		// Unauthenticated root/dashboard access: redirect to /login so the
		// user sees the login form immediately instead of a blank SPA shell.
		if path == "/" || path == "/dashboard" {
			if tokenStr, err := c.Cookie("forgec2_session"); err != nil || tokenStr == "" {
				c.Redirect(http.StatusFound, "/login")
				c.Abort()
				return
			}
		}

		// Cache control for hashed static assets (immutable) vs everything else
		if strings.HasPrefix(path, "/_next/static/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Header("Cache-Control", "no-store")
		}

		// Normalize trailing slash for SPA client routes so http.FileServer
		// never emits a 301. Browsers cache 301s aggressively, and the
		// /login <-> /login/ pair becomes an infinite redirect loop client-side
		// that never reaches the server.
		if len(path) > 1 && strings.HasSuffix(path, "/") {
			c.Request.URL.Path = path[:len(path)-1]
		}

		fsrv.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	})

	slog.Info("Embedded frontend enabled")
}

// malleableEnabled reports whether a malleable C2 profile is active (its GET
// URIs must never be shadowed by the SPA fallback). Nil-safe for tests.
func (s *Server) malleableEnabled() bool {
	return s.cfg != nil && s.cfg.Malleable.Enabled
}

// spaAssetExists reports whether the raw static FS holds a real file at path
// (directories and the index.html fallback of spaFS do not count).
func spaAssetExists(staticFS fs.FS, path string) bool {
	if path == "" || path == "/" {
		return false
	}
	f, err := staticFS.Open(strings.TrimPrefix(path, "/"))
	if err != nil {
		return false
	}
	defer f.Close()
	fi, err := f.Stat()
	return err == nil && !fi.IsDir()
}

// newHTTPServer creates an http.Server with standard timeout configuration.
func (s *Server) newHTTPServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           s.router,
		ReadTimeout:       HTTPReadTimeout,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      HTTPWriteTimeout,
		IdleTimeout:       HTTPIdleTimeout,
	}
}

// configureTLS sets up the TLS configuration on the given http.Server,
// including JARM/JA3 fingerprint randomization and optional mTLS.
func (s *Server) configureTLS(srv *http.Server) error {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

	if s.tlsFingerprint != nil {
		tlsConfig = s.tlsFingerprint.WrapTLSConfig(tlsConfig)
	}

	if s.cfg.Server.RequireClientCert && s.cfg.Server.ClientCAFile != "" {
		caCert, caErr := os.ReadFile(s.cfg.Server.ClientCAFile)
		if caErr != nil {
			slog.Error("Failed to read client CA file", "err", caErr)
			return fmt.Errorf("loading client CA for mTLS: %w", caErr)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("failed to parse client CA certificate")
		}
		tlsConfig.ClientCAs = caPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		slog.Info("mTLS enabled for beacon connections", "ca", s.cfg.Server.ClientCAFile)
	}

	srv.TLSConfig = tlsConfig
	return nil
}
