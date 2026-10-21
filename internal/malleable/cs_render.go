package malleable

import (
	"fmt"
	"sort"
	"strings"
)

// RenderCS serializes a v2 profile into a Cobalt Strike-style profile. It emits
// the HTTP blocks ForgeC2 can compile plus a documented forgec2 { ... }
// extension block for fields CS does not model. The result is intended for the
// online profile editor; builds continue to consume the compiled JSON.
func RenderCS(p *ProfileV2) string {
	if p == nil {
		return ""
	}
	var b strings.Builder
	name := strings.TrimSpace(p.Name)
	if name != "" {
		fmt.Fprintf(&b, "# ForgeC2 Malleable C2 profile: %s\n", name)
	}
	b.WriteString("# Syntax-compatible with Cobalt Strike; forgec2 { ... } preserves ForgeC2-only fields.\n\n")

	if p.Description != "" {
		fmt.Fprintf(&b, "set description %s;\n", csQuote(p.Description))
	}
	if p.Sleep > 0 {
		fmt.Fprintf(&b, "set sleeptime %q;\n", fmt.Sprintf("%d", p.Sleep*1000))
	}
	if p.Jitter > 0 {
		fmt.Fprintf(&b, "set jitter %q;\n", fmt.Sprintf("%d", p.Jitter))
	}
	if p.UserAgent != "" {
		fmt.Fprintf(&b, "set useragent %s;\n", csQuote(p.UserAgent))
	}
	b.WriteString("\n")

	method := strings.ToUpper(strings.TrimSpace(p.PrimaryMethod()))
	if method != "GET" {
		method = "POST"
	}
	uris := p.BeaconURIs
	if len(uris) == 0 {
		uris = p.URIs
	}
	uris = dedupStrings(uris)
	if p.BeaconURI != "" {
		uris = prependUniqueURI(p.BeaconURI, uris)
	}
	if len(uris) == 0 {
		uris = []string{"/collect"}
	}

	if method == "GET" {
		renderHTTPBlock(&b, "http-get", "GET", uris, p.Headers, p.ClientMetadata, nil, p.ServerOutput, p.Parameter)
	} else {
		// CS models metadata on http-get and id/output on http-post. Emitting
		// both preserves a POST profile's metadata chain without changing the
		// primary method (the compiler prefers http-post when both exist).
		if len(p.ClientMetadata) > 0 {
			renderHTTPBlock(&b, "http-get", "GET", uris, nil, p.ClientMetadata, nil, nil, "")
		}
		renderHTTPBlock(&b, "http-post", "POST", uris, p.Headers, nil, p.ClientID, p.ServerOutput, p.Parameter)
	}

	renderForgec2(&b, p, method)
	return b.String()
}

func renderHTTPBlock(b *strings.Builder, block, verb string, uris []string, headers map[string]string, metadata, id []StepV2, output []StepV2, parameter string) {
	fmt.Fprintf(b, "%s {\n", block)
	if len(uris) > 0 {
		b.WriteString("  set uri")
		for _, u := range uris {
			fmt.Fprintf(b, " %s", csQuote(u))
		}
		b.WriteString(";\n")
	}
	fmt.Fprintf(b, "  set verb %q;\n", verb)
	if parameter != "" {
		fmt.Fprintf(b, "  set parameter %s;\n", csQuote(parameter))
	}
	clientHasContent := len(headers) > 0 || len(metadata) > 0 || len(id) > 0
	if clientHasContent {
		b.WriteString("  client {\n")
		for _, k := range sortedKeys(headers) {
			fmt.Fprintf(b, "    header %s %s;\n", csQuote(k), csQuote(headers[k]))
		}
		if len(metadata) > 0 {
			b.WriteString("    metadata {\n")
			renderSteps(b, metadata)
			b.WriteString("    }\n")
		}
		if len(id) > 0 {
			b.WriteString("    id {\n")
			renderSteps(b, id)
			b.WriteString("    }\n")
		}
		b.WriteString("  }\n")
	}
	if len(output) > 0 {
		b.WriteString("  server {\n    output {\n")
		renderSteps(b, output)
		b.WriteString("    }\n  }\n")
	}
	b.WriteString("}\n\n")
}

func renderSteps(b *strings.Builder, steps []StepV2) {
	for _, st := range steps {
		t := normalizeTransform(st.Type)
		if t == "" {
			continue
		}
		switch t {
		case "strrep":
			old, rep, ok := strings.Cut(st.Value, ":")
			if ok {
				fmt.Fprintf(b, "      strrep %s %s;\n", csQuote(old), csQuote(rep))
			} else {
				fmt.Fprintf(b, "      strrep %s \"\";\n", csQuote(st.Value))
			}
		case "append", "prepend", "xor", "mask", "header", "parameter", "case":
			fmt.Fprintf(b, "      %s %s;\n", t, csQuote(st.Value))
		default:
			if t == "uri_append" {
				t = "uri-append"
			}
			b.WriteString("      " + t + ";\n")
		}
	}
}

func renderForgec2(b *strings.Builder, p *ProfileV2, method string) {
	has := p.Prepend != "" || p.Append != "" || p.RequestPrepend != "" || p.RequestAppend != "" ||
		len(p.RequestHeaders) > 0 || len(p.RequestHeaderPool) > 0 || len(p.UserAgents) > 0 ||
		len(p.ParameterNames) > 0 || len(p.Placements) > 0 || p.ContentLengthJitter != 0 ||
		p.JitterURI || p.JitterParameter || p.WorkStart != "" || p.WorkEnd != "" || p.WorkTZ != "" ||
		(method == "GET" && len(p.ClientID) > 0)
	if !has {
		return
	}
	b.WriteString("forgec2 {\n")
	if p.Prepend != "" {
		fmt.Fprintf(b, "  set prepend %s;\n", csQuote(p.Prepend))
	}
	if p.Append != "" {
		fmt.Fprintf(b, "  set append %s;\n", csQuote(p.Append))
	}
	if p.RequestPrepend != "" {
		fmt.Fprintf(b, "  set request_prepend %s;\n", csQuote(p.RequestPrepend))
	}
	if p.RequestAppend != "" {
		fmt.Fprintf(b, "  set request_append %s;\n", csQuote(p.RequestAppend))
	}
	for _, k := range sortedKeys(p.RequestHeaders) {
		fmt.Fprintf(b, "  set request_header %s %s;\n", csQuote(k), csQuote(p.RequestHeaders[k]))
	}
	for _, line := range p.RequestHeaderPool {
		fmt.Fprintf(b, "  set request_header_pool %s;\n", csQuote(line))
	}
	if len(p.UserAgents) > 0 {
		b.WriteString("  set user_agents")
		for _, ua := range p.UserAgents {
			fmt.Fprintf(b, " %s", csQuote(ua))
		}
		b.WriteString(";\n")
	}
	if len(p.ParameterNames) > 0 {
		b.WriteString("  set parameter_names")
		for _, n := range p.ParameterNames {
			fmt.Fprintf(b, " %s", csQuote(n))
		}
		b.WriteString(";\n")
	}
	for _, pl := range p.Placements {
		fmt.Fprintf(b, "  set placement %s %s;\n", csQuote(pl.Target), csQuote(pl.Chain))
	}
	if p.ContentLengthJitter != 0 {
		fmt.Fprintf(b, "  set content_length_jitter %q;\n", fmt.Sprintf("%d", p.ContentLengthJitter))
	}
	if p.JitterURI {
		b.WriteString("  set jitter_uri \"true\";\n")
	}
	if p.JitterParameter {
		b.WriteString("  set jitter_parameter \"true\";\n")
	}
	if p.WorkStart != "" {
		fmt.Fprintf(b, "  set work_start %s;\n", csQuote(p.WorkStart))
	}
	if p.WorkEnd != "" {
		fmt.Fprintf(b, "  set work_end %s;\n", csQuote(p.WorkEnd))
	}
	if p.WorkTZ != "" {
		fmt.Fprintf(b, "  set work_tz %s;\n", csQuote(p.WorkTZ))
	}
	if method == "GET" && len(p.ClientID) > 0 {
		fmt.Fprintf(b, "  set client_id %s;\n", csQuote(StepsToWire(p.ClientID)))
	}
	b.WriteString("}\n")
}

func sortedKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func csQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
