package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

// The static-serving tests below drive the real SetStaticFS middleware over a
// MapFS instead of the embedded dist, so the cache/conditional/compression
// matrix stays pinned without touching cmd/server.

var staticTestJS = "console.log('" + strings.Repeat("x", 4096) + "');"

func staticTestFiles() map[string]string {
	return map[string]string{
		"index.html":            "<html><body>index shell</body></html>",
		"login.html":            "<html><body>login shell</body></html>",
		"login/index.html":      "<html><body>login dir index</body></html>",
		"dashboard.html":        "<html><body>dashboard shell</body></html>",
		"dashboard/index.html":  "<html><body>dashboard dir index</body></html>",
		"assets/app-abc123.js":  staticTestJS,
		"assets/tiny-0000.js":   "console.log(1);",
		"assets/app-abc123.css": strings.Repeat(".a{color:#f00}", 400),
		"favicon.ico":           strings.Repeat("\xff", 4096),
		"js/vis-network.min.js": strings.Repeat("var v=1; ", 512),
	}
}

func newStaticTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	files := fstest.MapFS{}
	for name, body := range staticTestFiles() {
		files[name] = &fstest.MapFile{Data: []byte(body)}
	}
	s := &Server{router: gin.New()}
	// Mirror the production chain: middleware.NoCache() runs before the static
	// middleware and stamps the API policy onto every response.
	s.router.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	})
	s.SetStaticFS(files)
	return s.router
}

func serveStatic(t *testing.T, router *gin.Engine, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestStaticHashedAssetCachePolicy(t *testing.T) {
	router := newStaticTestRouter(t)
	w := serveStatic(t, router, http.MethodGet, "/assets/app-abc123.js", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q, want immutable policy", got)
	}
	if got := w.Header().Get("ETag"); got == "" {
		t.Error("ETag missing on hashed asset")
	}
	if got := w.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("unnegotiated response is encoded: %q", got)
	}
	// The blanket API NoCache middleware stamped Pragma/Expires first; the
	// static policy must have removed them.
	if got := w.Header().Get("Pragma"); got != "" {
		t.Errorf("Pragma leaked into static response: %q", got)
	}
	if got := w.Header().Get("Expires"); got != "" {
		t.Errorf("Expires leaked into static response: %q", got)
	}
	if got := w.Body.String(); got != staticTestJS {
		t.Errorf("body mismatch (%d bytes)", w.Body.Len())
	}
	if got := w.Header().Get("Content-Length"); got != strconv.Itoa(len(staticTestJS)) {
		t.Errorf("Content-Length = %q, want %d", got, len(staticTestJS))
	}
}

func TestStaticGzipNegotiation(t *testing.T) {
	router := newStaticTestRouter(t)
	w := serveStatic(t, router, http.MethodGet, "/js/vis-network.min.js",
		map[string]string{"Accept-Encoding": "gzip, deflate, br"})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := w.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
		t.Errorf("Vary = %q, want Accept-Encoding", got)
	}
	if got := w.Header().Get("Content-Length"); got != "" {
		t.Errorf("compressed response kept identity Content-Length %q", got)
	}
	if got := w.Body.Len(); got >= len(staticTestFiles()["js/vis-network.min.js"]) {
		t.Errorf("compressed body is not smaller: %d bytes", got)
	}

	zr, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if string(raw) != staticTestFiles()["js/vis-network.min.js"] {
		t.Error("decompressed body does not match the file")
	}
}

func TestStaticCompressionSkips(t *testing.T) {
	router := newStaticTestRouter(t)
	files := staticTestFiles()

	t.Run("q=0 refusal", func(t *testing.T) {
		w := serveStatic(t, router, http.MethodGet, "/assets/app-abc123.js",
			map[string]string{"Accept-Encoding": "gzip;q=0"})
		if got := w.Header().Get("Content-Encoding"); got != "" {
			t.Errorf("Content-Encoding = %q despite gzip;q=0", got)
		}
		if w.Body.String() != staticTestJS {
			t.Error("identity body mismatch")
		}
		if got := w.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
			t.Errorf("Vary = %q, want Accept-Encoding on the identity variant", got)
		}
	})

	t.Run("range request", func(t *testing.T) {
		w := serveStatic(t, router, http.MethodGet, "/assets/app-abc123.js",
			map[string]string{"Accept-Encoding": "gzip", "Range": "bytes=0-9"})
		if w.Code != http.StatusPartialContent {
			t.Fatalf("status = %d, want 206", w.Code)
		}
		if got := w.Header().Get("Content-Encoding"); got != "" {
			t.Errorf("Content-Encoding = %q on a range response", got)
		}
		if got := w.Header().Get("Content-Range"); got != "bytes 0-9/"+strconv.Itoa(len(staticTestJS)) {
			t.Errorf("Content-Range = %q", got)
		}
		if w.Body.String() != staticTestJS[:10] {
			t.Error("range body mismatch")
		}
	})

	t.Run("head request", func(t *testing.T) {
		w := serveStatic(t, router, http.MethodHead, "/assets/app-abc123.js",
			map[string]string{"Accept-Encoding": "gzip"})
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if got := w.Header().Get("Content-Encoding"); got != "" {
			t.Errorf("Content-Encoding = %q on a HEAD response", got)
		}
		if got := w.Header().Get("Content-Length"); got != strconv.Itoa(len(staticTestJS)) {
			t.Errorf("Content-Length = %q, want identity size", got)
		}
		if w.Body.Len() != 0 {
			t.Errorf("HEAD response carries a body (%d bytes)", w.Body.Len())
		}
	})

	t.Run("binary body", func(t *testing.T) {
		w := serveStatic(t, router, http.MethodGet, "/favicon.ico",
			map[string]string{"Accept-Encoding": "gzip"})
		if got := w.Header().Get("Content-Encoding"); got != "" {
			t.Errorf("Content-Encoding = %q for a non-compressible type", got)
		}
		if got := w.Header().Get("Content-Length"); got != strconv.Itoa(len(files["favicon.ico"])) {
			t.Errorf("Content-Length = %q, want raw size", got)
		}
	})

	t.Run("below threshold", func(t *testing.T) {
		w := serveStatic(t, router, http.MethodGet, "/assets/tiny-0000.js",
			map[string]string{"Accept-Encoding": "gzip"})
		if got := w.Header().Get("Content-Encoding"); got != "" {
			t.Errorf("Content-Encoding = %q for a sub-threshold file", got)
		}
		if got := w.Header().Get("Vary"); got != "" {
			t.Errorf("Vary = %q, want none for a sub-threshold file", got)
		}
	})
}

func TestStaticConditionalRequests(t *testing.T) {
	router := newStaticTestRouter(t)
	files := staticTestFiles()

	first := serveStatic(t, router, http.MethodGet, "/login", nil)
	if first.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", first.Code)
	}
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag on the route shell")
	}
	if got := first.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
	if first.Body.String() != files["login.html"] {
		t.Error("route shell body mismatch")
	}

	for _, inm := range []string{etag, "W/" + etag, `"stale", ` + etag, "*"} {
		w := serveStatic(t, router, http.MethodGet, "/login", map[string]string{"If-None-Match": inm})
		if w.Code != http.StatusNotModified {
			t.Errorf("If-None-Match %q: status = %d, want 304", inm, w.Code)
		}
		if w.Body.Len() != 0 {
			t.Errorf("If-None-Match %q: 304 carries a body (%d bytes)", inm, w.Body.Len())
		}
		if got := w.Header().Get("ETag"); got != etag {
			t.Errorf("If-None-Match %q: ETag = %q, want %q", inm, got, etag)
		}
	}

	stale := serveStatic(t, router, http.MethodGet, "/login", map[string]string{"If-None-Match": `"stale"`})
	if stale.Code != http.StatusOK {
		t.Errorf("non-matching validator: status = %d, want 200", stale.Code)
	}
	if stale.Body.String() != files["login.html"] {
		t.Error("non-matching validator: body mismatch")
	}
}

func TestStaticSpaFallbackUsesShellValidators(t *testing.T) {
	router := newStaticTestRouter(t)
	files := staticTestFiles()

	// Route paths with no matching file/dir fall back to the SPA shell. Use one
	// such route as the shell reference: /index.html itself is 301-redirected
	// to ./ by http.FileServer's canonical-path rule before any body is served.
	shell := serveStatic(t, router, http.MethodGet, "/agents/42", nil)
	if shell.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", shell.Code)
	}
	if shell.Body.String() != files["index.html"] {
		t.Fatal("fallback body is not the shell")
	}

	for _, route := range []string{"/deep/nested/route"} {
		w := serveStatic(t, router, http.MethodGet, route, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", route, w.Code)
		}
		if w.Body.String() != files["index.html"] {
			t.Errorf("%s: fallback body is not the shell", route)
		}
		if got := w.Header().Get("Cache-Control"); got != "no-cache" {
			t.Errorf("%s: Cache-Control = %q, want no-cache", route, got)
		}
		if got, want := w.Header().Get("ETag"), shell.Header().Get("ETag"); got == "" || got != want {
			t.Errorf("%s: ETag = %q, want the shell's %q", route, got, want)
		}
	}

	// A missing hashed asset must not inherit the immutable policy: the body
	// is the SPA shell, not the addressed file.
	missing := serveStatic(t, router, http.MethodGet, "/assets/nope-000000.js", nil)
	if got := missing.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("missing asset Cache-Control = %q, want no-cache", got)
	}
	if missing.Body.String() != files["index.html"] {
		t.Error("missing asset body is not the SPA shell")
	}
}

func TestStaticDirectoryShellServesSiblingHTML(t *testing.T) {
	router := newStaticTestRouter(t)

	w := serveStatic(t, router, http.MethodGet, "/dashboard", map[string]string{"Cookie": "forgec2_session=test"})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != staticTestFiles()["dashboard.html"] {
		t.Error("directory route did not serve its .html sibling")
	}
	if got := w.Header().Get("ETag"); got == "" {
		t.Error("directory route shell has no ETag")
	}
}

func TestAcceptsGzip(t *testing.T) {
	cases := map[string]bool{
		"":                          false,
		"deflate":                   false,
		"gzip":                      true,
		"gzip;q=0":                  false,
		"gzip;q=0.0":                false,
		"gzip;q=0.001":              true,
		"GZIP":                      true,
		"deflate, gzip;q=0.8":       true,
		"deflate;q=1, gzip;q=0, br": false,
		"*":                         true,
		"*;q=0":                     false,
		"br, *;q=0":                 false,
		"identity":                  false,
		"gzip;q=bogus":              true, // malformed q is not an explicit zero
	}
	for header, want := range cases {
		if got := acceptsGzip(header); got != want {
			t.Errorf("acceptsGzip(%q) = %v, want %v", header, got, want)
		}
	}
}

func TestEtagMatches(t *testing.T) {
	cases := map[string]bool{
		`"abc"`:      true,
		`W/"abc"`:    true,
		`"x", "abc"`: true,
		`"x"`:        false,
		`*`:          true,
		``:           false,
		`abc`:        false,
	}
	for header, want := range cases {
		if got := etagMatches(header, `"abc"`); got != want {
			t.Errorf("etagMatches(%q) = %v, want %v", header, got, want)
		}
	}
}

func TestBuildStaticAssetMetaHandlesNilFS(t *testing.T) {
	if meta := buildStaticAssetMeta(nil); meta == nil || len(meta) != 0 {
		t.Fatalf("nil FS produced %v, want empty non-nil map", meta)
	}
}
