package malleable

import (
	"strings"
	"testing"
)

func TestParseCSProfileHTTPAndTiming(t *testing.T) {
	src := `
set sleeptime "60000";
set jitter "20";
set useragent "Mozilla/5.0 Test";

http-get {
  set uri "/get-a" "/get-b";
  client {
    header "Accept" "*/*";
    metadata {
      base64;
      prepend "SES";
    }
  }
  server {
    output { base64; }
  }
}

http-post {
  set uri "/post-a";
  set verb "POST";
  set parameter "id";
  client {
    header "Content-Type" "application/octet-stream";
    id {
      parameter "id";
      mask;
    }
  }
  server {
    output { base64; }
  }
}
`
	res := ParseCSProfile("test", src)
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.ErrorMessages())
	}
	p := res.Profile
	if p.Sleep != 60 || p.Jitter != 20 {
		t.Fatalf("sleep/jitter = %d/%d, want 60/20", p.Sleep, p.Jitter)
	}
	if p.UserAgent != "Mozilla/5.0 Test" {
		t.Fatalf("user agent = %q", p.UserAgent)
	}
	if p.Method != "POST" || p.Verb != "POST" {
		t.Fatalf("method/verb = %q/%q", p.Method, p.Verb)
	}
	if p.BeaconURI != "/post-a" {
		t.Fatalf("primary URI = %q, want /post-a", p.BeaconURI)
	}
	if strings.Join(p.BeaconURIs, ",") != "/post-a,/get-a,/get-b" {
		t.Fatalf("URIs = %v", p.BeaconURIs)
	}
	if p.Headers["Accept"] != "*/*" || p.Headers["Content-Type"] != "application/octet-stream" {
		t.Fatalf("headers = %v", p.Headers)
	}
	if got := StepsToWire(p.ClientMetadata); got != "base64;prepend:SES" {
		t.Fatalf("client metadata = %q", got)
	}
	if got := StepsToWire(p.ClientID); got != "parameter:id" {
		t.Fatalf("client id = %q", got)
	}
	if got := StepsToWire(p.ServerOutput); got != "base64" {
		t.Fatalf("server output = %q", got)
	}
	if p.Parameter != "id" {
		t.Fatalf("parameter = %q", p.Parameter)
	}
}

func TestParseCSProfileGetOnly(t *testing.T) {
	res := ParseCSProfile("getter", `
set sleeptime "1000";
http-get {
  set uri "/a";
  client { metadata { base64; } }
  server { output { base64; } }
}
`)
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.ErrorMessages())
	}
	if res.Profile.Method != "GET" || res.Profile.BeaconURI != "/a" {
		t.Fatalf("method/uri = %q/%q", res.Profile.Method, res.Profile.BeaconURI)
	}
	if len(res.Profile.BeaconURIs) != 1 || res.Profile.BeaconURIs[0] != "/a" {
		t.Fatalf("uris = %v", res.Profile.BeaconURIs)
	}
}

func TestParseCSProfileNormalizesTransforms(t *testing.T) {
	res := ParseCSProfile("transforms", `
http-post {
  set uri "/x";
  client {
    id {
      uri-append;
      strrep "a" "b";
      base64url;
    }
  }
}
`)
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.ErrorMessages())
	}
	got := StepsToWire(res.Profile.ClientID)
	if got != "uri_append;strrep:a:b;base64url" {
		t.Fatalf("client id = %q", got)
	}
}

func TestParseCSProfileReportsSyntaxDiagnostics(t *testing.T) {
	res := ParseCSProfile("broken", `
set sleeptime "abc";
http-post {
  set uri "/x";
  client {
    id {
      made-up-transform;
    }
  }
`)
	if !res.HasErrors() {
		t.Fatal("expected errors")
	}
	var joined []string
	for _, d := range res.Diagnostics {
		joined = append(joined, d.Message)
	}
	all := strings.Join(joined, "\n")
	for _, want := range []string{"sleeptime", "unknown transform", "not closed"} {
		if !strings.Contains(all, want) {
			t.Fatalf("missing %q in diagnostics:\n%s", want, all)
		}
	}
	for _, d := range res.Diagnostics {
		if d.Line < 1 || d.Column < 1 {
			t.Fatalf("diagnostic lacks location: %+v", d)
		}
	}
}

func TestParseCSProfileUnterminatedString(t *testing.T) {
	res := ParseCSProfile("broken", "set useragent \"unterminated;\n")
	if !res.HasErrors() {
		t.Fatal("expected an unterminated string error")
	}
	found := false
	for _, d := range res.Diagnostics {
		if d.Code == "cs.unterminated_string" {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v", res.Diagnostics)
	}
}

func TestParseCSProfileUnsupportedBlocksAreWarnings(t *testing.T) {
	res := ParseCSProfile("unsupported", `
http-stager {
  set uri_x86 "/stage";
}
post-ex {
  set pipename "foo";
}
http-get {
  set uri "/a";
}
`)
	if res.HasErrors() {
		t.Fatalf("unsupported blocks should not be errors: %v", res.ErrorMessages())
	}
	if len(res.Ignored) != 2 || res.Ignored[0] != "http-stager" || res.Ignored[1] != "post-ex" {
		t.Fatalf("ignored = %v", res.Ignored)
	}
	if len(res.WarningMessages()) == 0 {
		t.Fatal("expected warnings for unsupported blocks")
	}
}

func TestParseCSProfileForgeExtension(t *testing.T) {
	res := ParseCSProfile("ext", `
set sleeptime "5";
set jitter "10";
http-post {
  set uri "/x";
  client { header "X-Test" "yes"; }
}
forgec2 {
  set content_length_jitter "128";
  set jitter_uri "true";
  set jitter_parameter "true";
  set request_header "X-Decoy" "one";
  set request_header_pool "X-Pool: two";
  set user_agents "UA1" "UA2";
  set parameter_names "id" "token";
  set placement "query:q" "base64";
  set work_start "09:00";
  set work_end "17:30";
  set work_tz "UTC";
  set prepend "PRE";
  set append "POST";
}
`)
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.ErrorMessages())
	}
	p := res.Profile
	if p.ContentLengthJitter != 128 || !p.JitterURI || !p.JitterParameter {
		t.Fatalf("jitter extensions = %+v", p)
	}
	if p.RequestHeaders["X-Decoy"] != "one" || len(p.RequestHeaderPool) != 1 {
		t.Fatalf("request headers = %v pool=%v", p.RequestHeaders, p.RequestHeaderPool)
	}
	if len(p.UserAgents) != 2 || len(p.ParameterNames) != 2 {
		t.Fatalf("user agents/parameter names = %v/%v", p.UserAgents, p.ParameterNames)
	}
	if len(p.Placements) != 1 || p.Placements[0].Target != "query:q" {
		t.Fatalf("placements = %+v", p.Placements)
	}
	if p.WorkStart != "09:00" || p.WorkEnd != "17:30" || p.WorkTZ != "UTC" {
		t.Fatalf("work window = %q/%q/%q", p.WorkStart, p.WorkEnd, p.WorkTZ)
	}
	if p.Prepend != "PRE" || p.Append != "POST" {
		t.Fatalf("wrap = %q/%q", p.Prepend, p.Append)
	}
}

func TestParseCSProfileForgeExtensionOverridesHTTP(t *testing.T) {
	res := ParseCSProfile("override", `
set useragent "UA/1.0";
http-post {
  set uri "/post";
  set verb "POST";
  client { id { base64; } }
  server { output { base64; } }
}
forgec2 {
  set method "GET";
  set beacon_uri "/forced";
  set header "X-Ext" "yes";
  set client_id "prepend:ID";
  set client_metadata "base64";
  set server_output "append:END";
}
`)
	if res.HasErrors() {
		t.Fatalf("unexpected errors: %v", res.ErrorMessages())
	}
	p := res.Profile
	if p.Method != "GET" || p.PrimaryURI() != "/forced" {
		t.Fatalf("extension method/uri lost: method=%q uri=%q", p.Method, p.PrimaryURI())
	}
	if len(p.BeaconURIs) == 0 || p.BeaconURIs[0] != "/forced" || p.URIs[0] != "/forced" {
		t.Fatalf("extension beacon uri not primary: beacon_uris=%v uris=%v", p.BeaconURIs, p.URIs)
	}
	if p.Headers["X-Ext"] != "yes" {
		t.Fatalf("extension header lost: %v", p.Headers)
	}
	if StepsToWire(p.ClientID) != "prepend:ID" || StepsToWire(p.ClientMetadata) != "base64" || StepsToWire(p.ServerOutput) != "append:END" {
		t.Fatalf("extension chains lost: id=%q meta=%q output=%q",
			StepsToWire(p.ClientID), StepsToWire(p.ClientMetadata), StepsToWire(p.ServerOutput))
	}
}

func TestRenderCSPreservesPrimaryURI(t *testing.T) {
	p := &ProfileV2{
		Name:       "primary",
		Method:     "POST",
		BeaconURI:  "/primary",
		BeaconURIs: []string{"/secondary", "/primary"},
		URIs:       []string{"/secondary", "/primary"},
	}
	res := ParseCSProfile(p.Name, RenderCS(p))
	if res.HasErrors() {
		t.Fatalf("rendered profile has errors: %v\n%s", res.ErrorMessages(), RenderCS(p))
	}
	if res.Profile.PrimaryURI() != "/primary" {
		t.Fatalf("primary URI = %q, want /primary\n%s", res.Profile.PrimaryURI(), RenderCS(p))
	}
}

func TestParseCSProfileMissingSemicolonWarns(t *testing.T) {
	res := ParseCSProfile("semi", "http-get {\n  set uri \"/a\"\n}\n")
	if res.HasErrors() {
		t.Fatalf("missing semicolon should be a warning: %v", res.ErrorMessages())
	}
	found := false
	for _, d := range res.Diagnostics {
		if d.Code == "cs.missing_semicolon" {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v", res.Diagnostics)
	}
}

func TestRenderCSRoundTripsForgeFields(t *testing.T) {
	src := `
set sleeptime "30";
set jitter "25";
set useragent "UA/1.0";
http-post {
  set uri "/submit" "/alt";
  set verb "POST";
  set parameter "id";
  client {
    header "Accept" "*/*";
    id { parameter "id"; base64; }
  }
  server { output { base64; } }
}
forgec2 {
  set request_header "X-Test" "one";
  set request_header_pool "X-Pool: two";
  set user_agents "UA2";
  set parameter_names "id" "token";
  set placement "query:q" "base64";
  set content_length_jitter "64";
  set jitter_uri "true";
  set work_start "09:00";
  set work_end "17:00";
  set work_tz "UTC";
}
`
	first := ParseCSProfile("roundtrip", src)
	if first.HasErrors() {
		t.Fatalf("source errors: %v", first.ErrorMessages())
	}
	rendered := RenderCS(first.Profile)
	second := ParseCSProfile("roundtrip", rendered)
	if second.HasErrors() {
		t.Fatalf("rendered source errors: %v\n%s", second.ErrorMessages(), rendered)
	}
	a, b := first.Profile, second.Profile
	if a.Sleep != b.Sleep || a.Jitter != b.Jitter || a.UserAgent != b.UserAgent ||
		a.Method != b.Method || a.BeaconURI != b.BeaconURI ||
		StepsToWire(a.ClientID) != StepsToWire(b.ClientID) ||
		StepsToWire(a.ServerOutput) != StepsToWire(b.ServerOutput) ||
		a.ContentLengthJitter != b.ContentLengthJitter ||
		a.RequestHeaders["X-Test"] != b.RequestHeaders["X-Test"] {
		t.Fatalf("round-trip changed fields:\nfirst=%+v\nsecond=%+v\nsource:\n%s", a, b, rendered)
	}
	if len(b.BeaconURIs) != 2 || len(b.Placements) != 1 || len(b.UserAgents) != 1 {
		t.Fatalf("round-trip lost lists: %+v", b)
	}
}
