package malleable

import (
	"math/rand"
	"testing"
)

// Test helpers (moved from production code — only used in tests)
func (g *HTTPGet) RandomURI() string {
	if len(g.URI) == 0 {
		return "/"
	}
	return g.URI[rand.Intn(len(g.URI))]
}

func (p *HTTPPost) RandomURI() string {
	if len(p.URI) == 0 {
		return "/"
	}
	return p.URI[rand.Intn(len(p.URI))]
}

func (j *JitterCfg) RandomPadding() []byte {
	if j.ContentLength <= 0 {
		return nil
	}
	n := rand.Intn(j.ContentLength)
	if n == 0 {
		return nil
	}
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = byte(rand.Intn(256))
	}
	return buf
}

func (j *JitterCfg) RandomParamName() string {
	if len(j.ParameterNames) == 0 {
		return "data"
	}
	return j.ParameterNames[rand.Intn(len(j.ParameterNames))]
}

func TestDefaultProfile(t *testing.T) {
	p := DefaultProfile()
	if p.Name != "default" {
		t.Fatalf("Name = %q, want %q", p.Name, "default")
	}
	if len(p.HttpGet.URI) == 0 {
		t.Fatal("HttpGet.URI should not be empty")
	}
	if len(p.HttpPost.URI) == 0 {
		t.Fatal("HttpPost.URI should not be empty")
	}
}

func TestPredefinedProfiles(t *testing.T) {
	profiles := PredefinedProfiles()
	expected := []string{"default", "microsoft", "google_analytics", "cloudflare_cdn", "akamai"}
	for _, name := range expected {
		if _, ok := profiles[name]; !ok {
			t.Fatalf("missing profile: %s", name)
		}
	}
	if len(profiles) != len(expected) {
		t.Fatalf("expected %d profiles, got %d", len(expected), len(profiles))
	}
}

func TestMicrosoftProfile(t *testing.T) {
	p := MicrosoftProfile()
	if p.Name != "microsoft" {
		t.Fatalf("Name = %q", p.Name)
	}
	if p.HttpPost.Parameter != "data" {
		t.Fatalf("HttpPost.Parameter = %q, want %q", p.HttpPost.Parameter, "data")
	}
	if p.HttpPost.ID == nil {
		t.Fatal("HttpPost.ID should not be nil")
	}
	if p.HttpPost.Output == nil {
		t.Fatal("HttpPost.Output should not be nil")
	}
	if p.HttpGet.Metadata == nil {
		t.Fatal("HttpGet.Metadata should not be nil")
	}
}

func TestParse(t *testing.T) {
	profileText := `
set description "Test profile"

http-get {
    set uri "/api/test"
    header "Host" "example.com"
    verb "GET"
}

http-post {
    set uri "/api/post"
    header "Content-Type" "text/plain"
    verb "POST"
}

http-config {
    jitter "10"
}
`
	p, err := Parse("test", profileText)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if p.Name != "test" {
		t.Fatalf("Name = %q", p.Name)
	}
	if p.Description != "Test profile" {
		t.Fatalf("Description = %q", p.Description)
	}
	if len(p.HttpGet.URI) != 1 || p.HttpGet.URI[0] != "/api/test" {
		t.Fatalf("HttpGet.URI = %v", p.HttpGet.URI)
	}
	if p.HttpGet.Verb != "GET" {
		t.Fatalf("HttpGet.Verb = %q", p.HttpGet.Verb)
	}
	if p.HttpGet.Headers["Host"] != "example.com" {
		t.Fatalf("HttpGet.Headers[Host] = %q", p.HttpGet.Headers["Host"])
	}
}

func TestParseMultipleURIs(t *testing.T) {
	profileText := `
http-get {
    set uri "/a" "/b" "/c";
}
`
	p, err := Parse("test", profileText)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.HttpGet.URI) != 3 {
		t.Fatalf("expected 3 URIs, got %d: %v", len(p.HttpGet.URI), p.HttpGet.URI)
	}
}

func TestParseJitter(t *testing.T) {
	profileText := `jitter "50"`
	p, err := Parse("test", profileText)
	if err != nil {
		t.Fatal(err)
	}
	if p.Jitter.ContentLength != 50 {
		t.Fatalf("Jitter.ContentLength = %d, want 50", p.Jitter.ContentLength)
	}
}

func TestRandomURI(t *testing.T) {
	t.Run("http get", func(t *testing.T) {
		g := &HTTPGet{URI: []string{"/a", "/b", "/c"}}
		seen := make(map[string]bool)
		for i := 0; i < 100; i++ {
			uri := g.RandomURI()
			if uri == "/" {
				t.Fatal("RandomURI returned fallback despite having URIs")
			}
			seen[uri] = true
		}
		if len(seen) != 3 {
			t.Fatalf("expected all 3 URIs to appear, got %d", len(seen))
		}
	})

	t.Run("empty URI list", func(t *testing.T) {
		g := &HTTPGet{}
		if uri := g.RandomURI(); uri != "/" {
			t.Fatalf("expected /, got %q", uri)
		}
	})
}

func TestRandomPadding(t *testing.T) {
	t.Run("zero content length", func(t *testing.T) {
		j := &JitterCfg{ContentLength: 0}
		if p := j.RandomPadding(); p != nil {
			t.Fatal("expected nil padding")
		}
	})

	t.Run("positive content length", func(t *testing.T) {
		j := &JitterCfg{ContentLength: 100}
		padding := j.RandomPadding()
		if len(padding) >= 100 {
			t.Fatalf("padding too large: %d", len(padding))
		}
	})
}

func TestRandomParamName(t *testing.T) {
	t.Run("with names", func(t *testing.T) {
		j := &JitterCfg{ParameterNames: []string{"a", "b", "c"}}
		seen := make(map[string]bool)
		for i := 0; i < 100; i++ {
			seen[j.RandomParamName()] = true
		}
		if len(seen) != 3 {
			t.Fatalf("expected all 3 names, got %d", len(seen))
		}
	})

	t.Run("empty names", func(t *testing.T) {
		j := &JitterCfg{}
		if name := j.RandomParamName(); name != "data" {
			t.Fatalf("expected data, got %q", name)
		}
	})
}
