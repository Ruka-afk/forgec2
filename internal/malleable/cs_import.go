package malleable

import (
	"fmt"
	"strings"
)

// ParseCSFull parses a full Cobalt Strike .profile text into v2, including
// http-get/http-post client{metadata,id} and server{output} blocks,
// header/parameter/uri-append terminators, and http-config user-agent.
func ParseCSFull(name, text string) (*ProfileV2, error) {
	base, err := Parse(name, text)
	if err != nil {
		return nil, err
	}
	v2 := FromLegacyProfile(base)
	if v2.Name == "" {
		v2.Name = name
	}
	// Deep parse for blocks Parse() drops.
	full := parseCSBlocks(text)
	if full.userAgent != "" && v2.UserAgent == "" {
		v2.UserAgent = full.userAgent
	}
	if len(full.getURIs) > 0 || len(full.postURIs) > 0 {
		v2.URIs = append(append([]string{}, full.getURIs...), full.postURIs...)
		v2.BeaconURIs = dedupStrings(v2.URIs)
		if v2.BeaconURI == "" && len(v2.URIs) > 0 {
			v2.BeaconURI = v2.URIs[0]
		}
	}
	if full.getVerb != "" || full.postVerb != "" {
		v2.Verbs = nil
		if full.getVerb != "" {
			v2.Verbs = append(v2.Verbs, strings.ToUpper(full.getVerb))
		}
		if full.postVerb != "" {
			v2.Verbs = append(v2.Verbs, strings.ToUpper(full.postVerb))
		}
		v2.Method = v2.Verbs[len(v2.Verbs)-1]
	}
	for k, val := range full.headers {
		if v2.Headers == nil {
			v2.Headers = map[string]string{}
		}
		if _, ok := v2.Headers[k]; !ok {
			v2.Headers[k] = val
		}
	}
	if len(full.getMetadata) > 0 {
		v2.ClientMetadata = full.getMetadata
	}
	if len(full.postID) > 0 {
		v2.ClientID = full.postID
	}
	if len(full.postOutput) > 0 {
		v2.ServerOutput = full.postOutput
	} else if len(full.getOutput) > 0 {
		v2.ServerOutput = full.getOutput
	}
	if full.parameter != "" {
		v2.Parameter = full.parameter
	}
	if v2.Description == "" && full.description != "" {
		v2.Description = full.description
	}
	if err := ValidateProfileV2(v2); err != nil {
		return v2, fmt.Errorf("CS profile converts with warnings: %w", err)
	}
	return v2, nil
}

type csBlocks struct {
	description string
	userAgent   string
	getURIs     []string
	postURIs    []string
	getVerb     string
	postVerb    string
	headers     map[string]string
	getMetadata []StepV2
	postID      []StepV2
	getOutput   []StepV2
	postOutput  []StepV2
	parameter   string
}

func parseCSBlocks(text string) *csBlocks {
	out := &csBlocks{headers: map[string]string{}}
	var stack []string // block path, e.g. http-get > client > metadata
	push := func(s string) { stack = append(stack, strings.ToLower(s)) }
	pop := func() {
		if len(stack) > 0 {
			stack = stack[:len(stack)-1]
		}
	}
	path := func() string { return strings.Join(stack, "/") }

	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		low := strings.ToLower(line)
		switch {
		case strings.HasPrefix(low, "set description"):
			out.description = extractQuoted(line)
		case strings.HasPrefix(low, "set useragent") || strings.HasPrefix(low, "set user_agent"):
			out.userAgent = extractQuoted(line)
		case strings.HasSuffix(low, "{"):
			head := strings.TrimSpace(strings.TrimSuffix(line, "{"))
			hl := strings.ToLower(head)
			switch {
			case strings.HasPrefix(hl, "http-get"):
				push("http-get")
			case strings.HasPrefix(hl, "http-post"):
				push("http-post")
			case strings.HasPrefix(hl, "http-config"):
				push("http-config")
			case strings.HasPrefix(hl, "client"):
				push("client")
			case strings.HasPrefix(hl, "server"):
				push("server")
			case strings.HasPrefix(hl, "metadata"):
				push("metadata")
			case strings.HasPrefix(hl, "output"):
				push("output")
			case strings.HasPrefix(hl, "id"):
				push("id")
			default:
				push(head)
			}
		case low == "}":
			pop()
		default:
			handleCSLine(out, path(), line)
		}
	}
	return out
}

func handleCSLine(out *csBlocks, path, line string) {
	low := strings.ToLower(strings.TrimSpace(line))
	getStep := func() (StepV2, bool) {
		t, v := parseCSStatement(line)
		if t == "" {
			return StepV2{}, false
		}
		return StepV2{Type: strings.ToLower(t), Value: v}, true
	}
	switch {
	case strings.HasPrefix(low, "set uri"):
		for _, u := range extractURIs(line) {
			if strings.Contains(path, "http-post") {
				out.postURIs = append(out.postURIs, u)
			} else {
				out.getURIs = append(out.getURIs, u)
			}
		}
	case strings.HasPrefix(low, "set verb"):
		v := extractWord(line)
		if strings.Contains(path, "http-post") {
			out.postVerb = v
		} else {
			out.getVerb = v
		}
	case strings.HasPrefix(low, "set parameter") || strings.HasPrefix(low, "set idparameter"):
		out.parameter = extractWord(line)
	case strings.HasPrefix(low, "header "):
		k, v := extractKV(line)
		if k != "" {
			out.headers[k] = v
		}
	default:
		// Transform statements inside client/server blocks.
		if st, ok := getStep(); ok {
			switch path {
			case "http-get/client/metadata":
				out.getMetadata = append(out.getMetadata, st)
			case "http-post/client/id":
				out.postID = append(out.postID, st)
			case "http-get/server/output":
				out.getOutput = append(out.getOutput, st)
			case "http-post/server/output":
				out.postOutput = append(out.postOutput, st)
			}
		}
	}
}

// parseCSStatement parses lines like: base64; prepend "SES"; header "Cookie"; parameter "id";
func parseCSStatement(line string) (string, string) {
	line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ";"))
	if line == "" {
		return "", ""
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", ""
	}
	typ := strings.ToLower(fields[0])
	rest := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
	rest = strings.Trim(strings.TrimSpace(rest), "\"")
	// header "Name" "Value" -> value "Name: Value"
	if typ == "header" {
		parts := splitQuoted(rest)
		if len(parts) >= 2 {
			return "header", parts[0] + ": " + parts[1]
		}
		if len(parts) == 1 {
			return "header", parts[0]
		}
		return "header", rest
	}
	return typ, rest
}

func splitQuoted(s string) []string {
	var out []string
	var cur strings.Builder
	inQ := false
	for _, r := range s {
		switch {
		case r == '"':
			inQ = !inQ
		case (r == ' ' || r == '\t') && !inQ:
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}
