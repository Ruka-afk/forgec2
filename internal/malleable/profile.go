// Package malleable implements a Cobalt Strike-compatible Malleable C2 profile parser,
// data transform chain, and HTTP traffic shaping system.
package malleable

import (
	"fmt"
	"strings"
)

// Profile represents a parsed Malleable C2 profile.
type Profile struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	HttpGet     HTTPGet    `json:"http_get,omitempty"`
	HttpPost    HTTPPost   `json:"http_post,omitempty"`
	HTTPConfig  HTTPConfig `json:"http_config,omitempty"`
	Jitter      JitterCfg  `json:"jitter,omitempty"`
}

// HTTPGet configures the agent's GET request.
type HTTPGet struct {
	URI      []string          `json:"uri"`
	Verb     string            `json:"verb"` // GET by default
	Headers  map[string]string `json:"headers"`
	Metadata *TransformBlock   `json:"metadata,omitempty"` // how to embed metadata in GET
	Output   *TransformBlock   `json:"output,omitempty"`   // how to parse server response
}

// HTTPPost configures the agent's POST request.
type HTTPPost struct {
	URI       []string          `json:"uri"`
	Verb      string            `json:"verb"` // POST by default
	Headers   map[string]string `json:"headers"`
	ID        *TransformBlock   `json:"id,omitempty"`     // how to embed agent ID in POST
	Output    *TransformBlock   `json:"output,omitempty"` // how to parse server response
	Parameter string            `json:"parameter"`        // POST body parameter name
}

// JitterCfg configures various jitter settings.
type JitterCfg struct {
	ContentLength  int      `json:"content_length"`  // max random padding bytes (0=disabled)
	URI            bool     `json:"uri"`              // random URI selection
	Parameter      bool     `json:"parameter"`        // random parameter names
	ParameterNames []string `json:"parameter_names"`  // pool of param names for jitter
}

// TransformBlock defines a chain of data transforms.
type TransformBlock struct {
	Transforms []Transform `json:"transforms"`
}

// Transform is a single data transform operation.
type Transform struct {
	Type  string `json:"type"`  // "base64", "netbios", "mask", "print", "append", "prepend", "xor", "urlencode", "uri_append", "header", "parameter"
	Value string `json:"value"` // parameter for transforms that need one
}

// HTTPConfig configures global HTTP settings.
type HTTPConfig struct {
	BlockRetry    int               `json:"block_retry"`
	BlockTimeout  int               `json:"block_timeout"`
	Headers       map[string]string `json:"headers"`
	TrustCert     bool              `json:"trust_cert"`
	UserAgent     string            `json:"user_agent"`
	XForwardedFor string            `json:"x_forwarded_for"`
}

// Parse parses a CS-style Malleable C2 profile text and returns a Profile.
func Parse(name, profileText string) (*Profile, error) {
	p := &Profile{Name: name}
	lines := strings.Split(profileText, "\n")
	var current string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		lower := strings.ToLower(line)

		switch {
		case strings.HasPrefix(lower, "set description"):
			p.Description = extractQuoted(line)

		case strings.HasPrefix(lower, "http-get {"):
			current = "http-get"
		case strings.HasPrefix(lower, "http-post {"):
			current = "http-post"
		case strings.HasPrefix(lower, "http-config {"):
			current = "http-config"

		case lower == "}":
			current = ""

		case current == "http-get":
			parseHTTPGetLine(&p.HttpGet, line)

		case current == "http-post":
			parseHTTPPostLine(&p.HttpPost, line)

		default:
			parseJitterLine(&p.Jitter, line)
		}
	}

	return p, nil
}

func extractQuoted(line string) string {
	start := strings.Index(line, "\"")
	if start == -1 {
		return ""
	}
	end := strings.LastIndex(line, "\"")
	if end <= start {
		return ""
	}
	return line[start+1 : end]
}

func parseHTTPGetLine(get *HTTPGet, line string) {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch {
	case strings.HasPrefix(lower, "uri") || strings.HasPrefix(lower, "set uri"):
		get.URI = append(get.URI, extractURIs(line)...)
	case strings.HasPrefix(lower, "header"):
		k, v := extractKV(line)
		if get.Headers == nil {
			get.Headers = make(map[string]string)
		}
		get.Headers[k] = v
	case strings.HasPrefix(lower, "verb"):
		get.Verb = extractWord(line)
	}
}

func parseHTTPPostLine(post *HTTPPost, line string) {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch {
	case strings.HasPrefix(lower, "uri") || strings.HasPrefix(lower, "set uri"):
		post.URI = append(post.URI, extractURIs(line)...)
	case strings.HasPrefix(lower, "header"):
		k, v := extractKV(line)
		if post.Headers == nil {
			post.Headers = make(map[string]string)
		}
		post.Headers[k] = v
	case strings.HasPrefix(lower, "verb"):
		post.Verb = extractWord(line)
	case strings.HasPrefix(lower, "parameter") || strings.HasPrefix(lower, "id") || strings.HasPrefix(lower, "idparameter"):
		post.Parameter = extractWord(line)
	}
}

func parseJitterLine(j *JitterCfg, line string) {
	lower := strings.ToLower(strings.TrimSpace(line))
	if strings.HasPrefix(lower, "jitter") {
		val := extractWord(line)
		fmt.Sscanf(val, "%d", &j.ContentLength)
	}
}

func extractURIs(line string) []string {
	content := extractQuoted(line)
	if content == "" {
		return nil
	}
	parts := strings.Fields(content)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, "\"")
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func extractKV(line string) (string, string) {
	firstQ := strings.Index(line, "\"")
	if firstQ == -1 {
		return "", ""
	}
	secondQ := strings.Index(line[firstQ+1:], "\"")
	if secondQ == -1 {
		return "", ""
	}
	key := line[firstQ+1 : firstQ+1+secondQ]

	rest := line[firstQ+1+secondQ+1:]
	thirdQ := strings.Index(rest, "\"")
	if thirdQ == -1 {
		return key, ""
	}
	fourthQ := strings.Index(rest[thirdQ+1:], "\"")
	if fourthQ == -1 {
		return key, rest[thirdQ+1:]
	}
	value := rest[thirdQ+1 : thirdQ+1+fourthQ]
	return key, value
}

func extractWord(line string) string {
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		return strings.Trim(parts[len(parts)-1], "\"")
	}
	return ""
}

// PredefinedProfiles returns the built-in profile presets.
func PredefinedProfiles() map[string]*Profile {
	return map[string]*Profile{
		"default":           DefaultProfile(),
		"microsoft":         MicrosoftProfile(),
		"google_analytics":  GoogleAnalyticsProfile(),
		"cloudflare_cdn":    CloudflareProfile(),
		"akamai":            AkamaiProfile(),
	}
}

// DefaultProfile returns the default ForgeC2 profile.
func DefaultProfile() *Profile {
	return &Profile{
		Name:        "default",
		Description: "Default ForgeC2 beacon profile",
		HttpGet: HTTPGet{
			URI:  []string{"/api/v1/beacon"},
			Verb: "GET",
			Headers: map[string]string{
				"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			},
		},
		HttpPost: HTTPPost{
			URI:  []string{"/api/v1/beacon"},
			Verb: "POST",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
	}
}

// MicrosoftProfile simulates Microsoft 365 telemetry traffic.
func MicrosoftProfile() *Profile {
	return &Profile{
		Name:        "microsoft",
		Description: "Microsoft 365 telemetry simulation",
		HttpGet: HTTPGet{
			URI:  []string{"/common/oauth2/token", "/login.srf", "/api/healthcheck"},
			Verb: "GET",
			Headers: map[string]string{
				"User-Agent":      "Microsoft Office/16.0 (Windows NT 10.0; Microsoft Windows 10 Pro; en-US)",
				"Accept":          "application/json",
				"Client-Request-Id": "{{guid}}",
			},
			Metadata: &TransformBlock{
				Transforms: []Transform{
					{Type: "base64"},
					{Type: "prepend", Value: "session_id="},
					{Type: "print"},
				},
			},
			Output: &TransformBlock{
				Transforms: []Transform{
					{Type: "netbios"},
					{Type: "base64"},
				},
			},
		},
		HttpPost: HTTPPost{
			URI:  []string{"/common/oauth2/token", "/api/telemetry"},
			Verb: "POST",
			Headers: map[string]string{
				"User-Agent":      "Microsoft Office/16.0 (Windows NT 10.0; Microsoft Windows 10 Pro; en-US)",
				"Content-Type":    "application/x-www-form-urlencoded",
				"Accept":          "application/json",
			},
			Parameter: "data",
			ID: &TransformBlock{
				Transforms: []Transform{
					{Type: "append", Value: "@microsoft.com"},
				},
			},
			Output: &TransformBlock{
				Transforms: []Transform{
					{Type: "base64"},
					{Type: "xor", Value: "microsoft"},
				},
			},
		},
	}
}

// GoogleAnalyticsProfile simulates Google Analytics traffic.
func GoogleAnalyticsProfile() *Profile {
	return &Profile{
		Name:        "google_analytics",
		Description: "Google Analytics beacon simulation",
		HttpGet: HTTPGet{
			URI:  []string{"/collect", "/r/collect", "/j/collect"},
			Verb: "GET",
			Headers: map[string]string{
				"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			},
			Metadata: &TransformBlock{
				Transforms: []Transform{
					{Type: "base64"},
					{Type: "prepend", Value: "v=1&t=pageview&tid=UA-"},
				},
			},
		},
		HttpPost: HTTPPost{
			URI:  []string{"/batch", "/collect"},
			Verb: "POST",
			Headers: map[string]string{
				"User-Agent":    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
				"Content-Type":  "text/plain",
			},
		},
		Jitter: JitterCfg{
			ContentLength:  512,
			ParameterNames: []string{"v", "t", "tid", "cid", "dp", "dl", "ul", "de", "dt"},
		},
	}
}

// CloudflareProfile simulates Cloudflare CDN traffic.
func CloudflareProfile() *Profile {
	return &Profile{
		Name:        "cloudflare_cdn",
		Description: "Cloudflare CDN edge request simulation",
		HttpGet: HTTPGet{
			URI:  []string{"/cdn-cgi/trace", "/cdn-cgi/rum", "/cdn-cgi/performance"},
			Verb: "GET",
			Headers: map[string]string{
				"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
				"CF-Connecting-IP": "{{ip}}",
				"CDN-Loop":        "cloudflare",
			},
		},
		HttpPost: HTTPPost{
			URI:  []string{"/cdn-cgi/rum", "/cdn-cgi/beacon"},
			Verb: "POST",
			Headers: map[string]string{
				"Content-Type": "text/plain",
				"CDN-Loop":     "cloudflare",
			},
		},
	}
}

// AkamaiProfile simulates Akamai CDN traffic.
func AkamaiProfile() *Profile {
	return &Profile{
		Name:        "akamai",
		Description: "Akamai CDN request simulation",
		HttpGet: HTTPGet{
			URI:  []string{"/akamai/collect", "/akamai/pixel"},
			Verb: "GET",
			Headers: map[string]string{
				"User-Agent":  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
				"X-Akamai-Config": "true",
			},
		},
		HttpPost: HTTPPost{
			URI:  []string{"/akamai/beacon"},
			Verb: "POST",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
		Jitter: JitterCfg{
			ContentLength: 1024,
		},
	}
}


