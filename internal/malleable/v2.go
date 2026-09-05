package malleable

import (
	"fmt"
	"regexp"
	"strings"
)

// ProfileV2 is the unified canonical Malleable schema.
// v1 JSON (payload MalleableProfile), legacy Profile (http-get/post blocks),
// and compiler YAML all migrate into this form. New writes are v2 only.
type ProfileV2 struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	UserAgent   string `json:"user_agent,omitempty"`
	// UA rotation pool: one per line in the UI; empty falls back to UserAgent.
	UserAgents []string          `json:"user_agents,omitempty"`
	BeaconURIs []string          `json:"beacon_uris,omitempty"`
	BeaconURI  string            `json:"beacon_uri,omitempty"`
	Method     string            `json:"method,omitempty"`
	Verb       string            `json:"verb,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Sleep      int               `json:"sleep,omitempty"`
	Jitter     int               `json:"jitter,omitempty"`
	// Response wrapping (server -> agent)
	Prepend string `json:"prepend,omitempty"`
	Append  string `json:"append,omitempty"`
	// Request wrapping (agent -> server)
	RequestPrepend string            `json:"request_prepend,omitempty"`
	RequestAppend  string            `json:"request_append,omitempty"`
	RequestHeaders map[string]string `json:"request_headers,omitempty"`
	// Full transform chains (CS parity). Wire format is the same
	// "name" or "name:value" ';'-joined string the agent already parses,
	// but stored structured so the UI can edit step-by-step.
	ClientMetadata []StepV2 `json:"client_metadata,omitempty"`
	ClientID       []StepV2 `json:"client_id,omitempty"`
	ServerOutput   []StepV2 `json:"server_output,omitempty"`
	// Jitter extensions
	ContentLengthJitter int      `json:"content_length_jitter,omitempty"`
	JitterURI           bool     `json:"jitter_uri,omitempty"`
	JitterParameter     bool     `json:"jitter_parameter,omitempty"`
	ParameterNames      []string `json:"parameter_names,omitempty"`
	Parameter           string   `json:"parameter,omitempty"`
	// Multi-verb / rotation
	URIs  []string `json:"uris,omitempty"`
	Verbs []string `json:"verbs,omitempty"`
	// Working-hours window (HH:MM + IANA TZ); empty = disabled.
	WorkStart string `json:"work_start,omitempty"`
	WorkEnd   string `json:"work_end,omitempty"`
	WorkTZ    string `json:"work_tz,omitempty"`
	// Request placements: cover copies of the envelope at non-body
	// locations. Each entry: {target, chain} where target is one of
	// body | query:<name> | cookie:<name> | header:<Name>.
	Placements []PlacementV2 `json:"placements,omitempty"`
}

// PlacementV2 pins an encoded copy of the beacon envelope to a location.
type PlacementV2 struct {
	Target string `json:"target"`
	Chain  string `json:"chain,omitempty"`
}

// ParsePlacementTarget splits "query:id" into kind + name.
func ParsePlacementTarget(target string) (kind, name string, ok bool) {
	t := strings.TrimSpace(target)
	if t == "" || strings.EqualFold(t, "body") {
		return "body", "", true
	}
	if i := strings.Index(t, ":"); i >= 0 {
		kind, name = strings.ToLower(strings.TrimSpace(t[:i])), strings.TrimSpace(t[i+1:])
	} else {
		kind, name = strings.ToLower(t), ""
	}
	switch kind {
	case "query", "cookie", "header":
		if name == "" {
			return "", "", false
		}
		if strings.ContainsAny(name, " \t\r\n;=") {
			return "", "", false
		}
		return kind, name, true
	case "body":
		return "body", "", true
	}
	return "", "", false
}

type StepV2 struct {
	Type   string `json:"type"`
	Value  string `json:"value,omitempty"`
	Target string `json:"target,omitempty"`
}

var v2NameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

var workHMRe = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

var allowedStepTypes = map[string]bool{
	"base64": true, "base64url": true, "netbios": true, "netbiosu": true,
	"mask": true, "print": true, "append": true, "prepend": true,
	"xor": true, "urlencode": true, "uri_append": true,
	"header": true, "parameter": true, "strrep": true, "case": true,
}

func ValidateProfileV2(p *ProfileV2) error {
	if p == nil {
		return fmt.Errorf("profile is required")
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return fmt.Errorf("profile name is required")
	}
	if !v2NameRe.MatchString(name) {
		return fmt.Errorf("profile name must match ^[a-zA-Z0-9_-]+$")
	}
	if p.Sleep < 0 || p.Sleep > 86400 {
		return fmt.Errorf("sleep must be 0..86400")
	}
	if p.Jitter < 0 || p.Jitter > 100 {
		return fmt.Errorf("jitter must be 0..100")
	}
	if p.ContentLengthJitter < 0 || p.ContentLengthJitter > 4096 {
		return fmt.Errorf("content_length_jitter must be 0..4096")
	}
	for _, u := range append(append(append([]string{}, p.BeaconURIs...), p.BeaconURI), append(p.URIs, "")...) {
		if u == "" {
			continue
		}
		if !strings.HasPrefix(u, "/") {
			return fmt.Errorf("uri %q must start with /", u)
		}
		if strings.ContainsAny(u, " \t\r\n") {
			return fmt.Errorf("uri %q contains whitespace", u)
		}
	}
	for _, v := range append([]string{p.Method, p.Verb}, p.Verbs...) {
		if v == "" {
			continue
		}
		up := strings.ToUpper(strings.TrimSpace(v))
		if up != "GET" && up != "POST" {
			return fmt.Errorf("verb %q must be GET or POST", v)
		}
	}
	for _, chain := range [][]StepV2{p.ClientMetadata, p.ClientID, p.ServerOutput} {
		for _, s := range chain {
			t := strings.ToLower(strings.TrimSpace(s.Type))
			if t == "" {
				return fmt.Errorf("transform type is required")
			}
			if !allowedStepTypes[t] {
				return fmt.Errorf("unknown transform %q", s.Type)
			}
			if (t == "xor" || t == "mask") && strings.TrimSpace(s.Value) == "" {
				return fmt.Errorf("transform %q requires a value", t)
			}
		}
	}
	for _, pl := range p.Placements {
		if _, _, ok := ParsePlacementTarget(pl.Target); !ok {
			return fmt.Errorf("placement target %q must be body | query:<name> | cookie:<name> | header:<Name>", pl.Target)
		}
		for _, s := range ParseWire(pl.Chain) {
			t := strings.ToLower(strings.TrimSpace(s.Type))
			if t == "" {
				continue
			}
			if !allowedStepTypes[t] {
				return fmt.Errorf("placement chain: unknown transform %q", s.Type)
			}
			if (t == "xor" || t == "mask") && strings.TrimSpace(s.Value) == "" {
				return fmt.Errorf("placement chain: transform %q requires a value", t)
			}
		}
	}
	for _, s := range []string{p.Name, p.Description, p.UserAgent, p.BeaconURI, p.Method, p.Prepend, p.Append, p.RequestPrepend, p.RequestAppend, p.WorkStart, p.WorkEnd, p.WorkTZ} {
		if !fieldSafeV2(s) {
			return fmt.Errorf("field contains unsafe characters (quote, backtick, $ or control)")
		}
	}
	for _, w := range []string{p.WorkStart, p.WorkEnd} {
		if w != "" && !workHMRe.MatchString(w) {
			return fmt.Errorf("work window %q must be HH:MM", w)
		}
	}
	for k, v := range p.Headers {
		if !fieldSafeV2(k) || !fieldSafeV2(v) {
			return fmt.Errorf("header %q contains unsafe characters", k)
		}
	}
	for k, v := range p.RequestHeaders {
		if !fieldSafeV2(k) || !fieldSafeV2(v) {
			return fmt.Errorf("request header %q contains unsafe characters", k)
		}
	}
	return nil
}

func fieldSafeV2(s string) bool {
	if strings.ContainsAny(s, "`\"$") {
		return false
	}
	if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
		return false
	}
	for _, r := range s {
		if r < 0x20 && r != '\t' && r != '\n' {
			return false
		}
		if r == 0x7f {
			return false
		}
	}
	return true
}

// StepsToWire serializes a chain to the agent wire form.
func StepsToWire(steps []StepV2) string {
	parts := make([]string, 0, len(steps))
	for _, s := range steps {
		t := strings.ToLower(strings.TrimSpace(s.Type))
		if t == "" {
			continue
		}
		if s.Value != "" {
			parts = append(parts, t+":"+s.Value)
		} else {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, ";")
}

// ParseWire parses the agent wire form back to steps. Values may contain ':'
// (split on first colon) but must not contain ';'.
func ParseWire(wire string) []StepV2 {
	wire = strings.TrimSpace(wire)
	if wire == "" {
		return nil
	}
	var out []StepV2
	for _, part := range strings.Split(wire, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, value := part, ""
		if i := strings.Index(part, ":"); i >= 0 {
			name, value = strings.TrimSpace(part[:i]), part[i+1:]
		}
		out = append(out, StepV2{Type: strings.ToLower(name), Value: value})
	}
	return out
}

// PrimaryURI returns the effective single beacon URI for legacy consumers.
func (p *ProfileV2) PrimaryURI() string {
	if p.BeaconURI != "" {
		return p.BeaconURI
	}
	for _, u := range p.BeaconURIs {
		if u != "" {
			return u
		}
	}
	for _, u := range p.URIs {
		if u != "" {
			return u
		}
	}
	return "/api/v1/beacon"
}

// PrimaryMethod returns the effective verb.
func (p *ProfileV2) PrimaryMethod() string {
	for _, v := range []string{p.Method, p.Verb} {
		if v != "" {
			return strings.ToUpper(v)
		}
	}
	if len(p.Verbs) > 0 && p.Verbs[0] != "" {
		return strings.ToUpper(p.Verbs[0])
	}
	return "POST"
}
