package malleable

import (
	"fmt"
	"strings"
)

// ParseCSFull parses a full Cobalt Strike .profile text into v2. It is the
// compatibility wrapper around ParseCSProfile used by import/build callers
// that only need an error/no-error result.
func ParseCSFull(name, text string) (*ProfileV2, error) {
	res := ParseCSProfile(name, text)
	if res.Profile == nil {
		return nil, fmt.Errorf("CS profile could not be parsed")
	}
	if res.HasErrors() {
		msgs := res.ErrorMessages()
		if len(msgs) > 0 {
			return res.Profile, fmt.Errorf("CS profile has %d error(s): %s", len(msgs), msgs[0])
		}
		return res.Profile, fmt.Errorf("CS profile has syntax errors")
	}
	return res.Profile, nil
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
