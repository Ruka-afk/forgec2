package malleable

import (
	"fmt"
	"strconv"
	"strings"
)

// Diagnostic is a line/column-addressed parse or validation message for the
// CS profile editor. Severity is "error" or "warning".
type Diagnostic struct {
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// CSResult is the partial result of parsing a CS-style profile. Profile is
// populated even when Diagnostics contains errors so the editor can show a
// compiled preview while the operator fixes syntax.
type CSResult struct {
	Profile     *ProfileV2
	Diagnostics []Diagnostic
	Ignored     []string
}

// HasErrors reports whether any diagnostic is an error.
func (r CSResult) HasErrors() bool {
	for _, d := range r.Diagnostics {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

// ErrorMessages returns the error diagnostics as human-readable strings.
func (r CSResult) ErrorMessages() []string {
	var out []string
	for _, d := range r.Diagnostics {
		if d.Severity == "error" {
			out = append(out, fmt.Sprintf("line %d:%d: %s", d.Line, d.Column, d.Message))
		}
	}
	return out
}

// WarningMessages returns warning diagnostics as human-readable strings.
func (r CSResult) WarningMessages() []string {
	var out []string
	for _, d := range r.Diagnostics {
		if d.Severity == "warning" {
			out = append(out, d.Message)
		}
	}
	return out
}

type csTokenKind int

const (
	csTokEOF csTokenKind = iota
	csTokIdent
	csTokString
	csTokLBrace
	csTokRBrace
	csTokSemicolon
	csTokNewline
)

type csToken struct {
	kind  csTokenKind
	text  string
	line  int
	col   int
	quote bool
}

// csStatement is a deliberately generic node. Every CS profile construct is
// either a `set` statement, a bare statement (header/transform), or a block
// with children. Keeping the tree generic lets the parser accept unknown CS
// blocks without losing syntax errors inside them.
type csStatement struct {
	name     string
	args     []string
	children []*csStatement
	isSet    bool
	line     int
	col      int
}

func csLex(text string) ([]csToken, []Diagnostic) {
	var toks []csToken
	var diags []Diagnostic
	line, col := 1, 1
	i := 0
	if strings.HasPrefix(text, "\ufeff") {
		i += len("\ufeff")
	}
	diag := func(l, c int, code, msg string) {
		diags = append(diags, Diagnostic{Line: l, Column: c, Severity: "error", Code: code, Message: msg})
	}
	for i < len(text) {
		c := text[i]
		switch c {
		case '\n':
			toks = append(toks, csToken{kind: csTokNewline, text: "\n", line: line, col: col})
			i++
			line++
			col = 1
		case '\r':
			i++
			col++
		case ' ', '\t':
			i++
			col++
		case '#':
			for i < len(text) && text[i] != '\n' {
				i++
				col++
			}
		case '"':
			startLine, startCol := line, col
			i++
			col++
			var b strings.Builder
			closed := false
			for i < len(text) {
				ch := text[i]
				if ch == '\n' {
					break
				}
				if ch == '\\' && i+1 < len(text) {
					next := text[i+1]
					switch next {
					case 'n':
						b.WriteByte('\n')
					case 'r':
						b.WriteByte('\r')
					case 't':
						b.WriteByte('\t')
					case '"':
						b.WriteByte('"')
					case '\\':
						b.WriteByte('\\')
					default:
						b.WriteByte(next)
					}
					i += 2
					col += 2
					continue
				}
				if ch == '"' {
					i++
					col++
					closed = true
					break
				}
				b.WriteByte(ch)
				i++
				col++
			}
			if !closed {
				diag(startLine, startCol, "cs.unterminated_string", "unterminated string")
			}
			toks = append(toks, csToken{kind: csTokString, text: b.String(), line: startLine, col: startCol, quote: true})
		case '{':
			toks = append(toks, csToken{kind: csTokLBrace, text: "{", line: line, col: col})
			i++
			col++
		case '}':
			toks = append(toks, csToken{kind: csTokRBrace, text: "}", line: line, col: col})
			i++
			col++
		case ';':
			toks = append(toks, csToken{kind: csTokSemicolon, text: ";", line: line, col: col})
			i++
			col++
		default:
			start, startCol := i, col
			for i < len(text) {
				ch := text[i]
				if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' ||
					ch == '{' || ch == '}' || ch == ';' || ch == '"' || ch == '#' {
					break
				}
				i++
				col++
			}
			if i == start {
				i++
				col++
				continue
			}
			toks = append(toks, csToken{kind: csTokIdent, text: text[start:i], line: line, col: startCol})
		}
	}
	toks = append(toks, csToken{kind: csTokEOF, line: line, col: col})
	return toks, diags
}

type csParser struct {
	toks  []csToken
	pos   int
	diags []Diagnostic
}

func (p *csParser) cur() csToken {
	if p.pos >= len(p.toks) {
		return csToken{kind: csTokEOF}
	}
	return p.toks[p.pos]
}

func (p *csParser) next() csToken {
	t := p.cur()
	if p.pos < len(p.toks) {
		p.pos++
	}
	return t
}

func (p *csParser) skipNewlines() {
	for p.cur().kind == csTokNewline {
		p.pos++
	}
}

func (p *csParser) diag(t csToken, severity, code, msg string) {
	p.diags = append(p.diags, Diagnostic{
		Line: t.line, Column: t.col, Severity: severity, Code: code, Message: msg,
	})
}

func (p *csParser) errorAt(t csToken, code, msg string) { p.diag(t, "error", code, msg) }
func (p *csParser) warnAt(t csToken, code, msg string)  { p.diag(t, "warning", code, msg) }

// parseStatements parses until EOF or a closing brace. closed is false when
// the block was left open at EOF.
func (p *csParser) parseStatements(inBlock bool) ([]*csStatement, bool) {
	var out []*csStatement
	for {
		p.skipNewlines()
		tok := p.cur()
		if tok.kind == csTokEOF {
			return out, false
		}
		if tok.kind == csTokRBrace {
			if !inBlock {
				p.errorAt(tok, "cs.unexpected_brace", "unexpected '}'")
				p.next()
				continue
			}
			p.next()
			return out, true
		}
		if tok.kind != csTokIdent {
			p.errorAt(tok, "cs.expected_statement", "expected a statement name")
			p.next()
			continue
		}
		nameTok := p.next()
		p.skipNewlines()
		if p.cur().kind == csTokLBrace {
			p.next()
			children, closed := p.parseStatements(true)
			if !closed {
				p.errorAt(nameTok, "cs.unclosed_block", fmt.Sprintf("block %q is not closed", nameTok.text))
			}
			out = append(out, &csStatement{
				name: strings.ToLower(nameTok.text), children: children,
				line: nameTok.line, col: nameTok.col,
			})
			continue
		}
		args := []string{}
		for {
			t := p.cur()
			if t.kind == csTokIdent || t.kind == csTokString {
				args = append(args, t.text)
				p.next()
				continue
			}
			break
		}
		if p.cur().kind == csTokLBrace {
			p.errorAt(p.cur(), "cs.unexpected_brace", "unexpected '{' after statement arguments")
			p.next()
		}
		switch p.cur().kind {
		case csTokSemicolon:
			p.next()
		case csTokNewline, csTokEOF, csTokRBrace:
			if len(args) > 0 {
				p.warnAt(nameTok, "cs.missing_semicolon", "missing ';' after statement")
			}
		default:
			p.errorAt(p.cur(), "cs.expected_semicolon", "expected ';' after statement")
			p.next()
		}
		out = append(out, &csStatement{
			name: strings.ToLower(nameTok.text), args: args, isSet: strings.EqualFold(nameTok.text, "set"),
			line: nameTok.line, col: nameTok.col,
		})
	}
}

func (p *csParser) parseFile() []*csStatement {
	out, _ := p.parseStatements(false)
	return out
}

// csHTTPBlock collects the fields of one http-get or http-post block.
type csHTTPBlock struct {
	present  bool
	uris     []string
	verb     string
	headers  map[string]string
	metadata []StepV2
	id       []StepV2
	output   []StepV2
	param    string
}

// ParseCSProfile parses a Cobalt Strike Malleable C2 profile into the v2
// canonical form. It accepts the full CS block grammar, maps the HTTP beacon
// subset ForgeC2 implements, and reports unsupported blocks as warnings while
// preserving them in the editor's source text.
func ParseCSProfile(name, text string) CSResult {
	toks, diags := csLex(text)
	p := &csParser{toks: toks, diags: diags}
	stmts := p.parseFile()
	b := &csBuilder{
		name:  strings.TrimSpace(name),
		v2:    &ProfileV2{Name: strings.TrimSpace(name)},
		diags: append([]Diagnostic{}, p.diags...),
		get:   &csHTTPBlock{headers: map[string]string{}},
		post:  &csHTTPBlock{headers: map[string]string{}},
	}
	if b.name == "" {
		b.name = "imported"
		b.v2.Name = b.name
	}
	b.walkTop(stmts)
	b.finish()
	if b.v2.Name == "" {
		b.v2.Name = b.name
	}
	if err := ValidateProfileV2(b.v2); err != nil {
		b.error(1, 1, "cs.invalid_profile", err.Error())
	}
	return CSResult{Profile: b.v2, Diagnostics: b.diags, Ignored: b.ignored}
}

type csBuilder struct {
	name    string
	v2      *ProfileV2
	diags   []Diagnostic
	ignored []string

	get  *csHTTPBlock
	post *csHTTPBlock

	sleepMS  int
	sleepSet bool
	jitter   int
	jitSet   bool

	extPrepend        string
	extAppend         string
	extRequestPrepend string
	extRequestAppend  string
	extRequestHeaders map[string]string
	extHeaderPool     []string
	extUserAgents     []string
	extParameterNames []string
	extPlacements     []PlacementV2
	extContentJitter  int
	extJitterURI      bool
	extJitterParam    bool
	extParameter      string
	extWorkStart      string
	extWorkEnd        string
	extWorkTZ         string

	// forgec2 { ... } fields that overlap HTTP block fields. Flags let an
	// explicit extension value win regardless of block order.
	extClientID        []StepV2
	extClientIDSet     bool
	extClientMetadata  []StepV2
	extClientMetaSet   bool
	extServerOutput    []StepV2
	extServerOutputSet bool
	extHeaders         map[string]string
	extMethod          string
	extMethodSet       bool
	extBeaconURI       string
	extBeaconURISet    bool
}

func (b *csBuilder) error(line, col int, code, msg string) {
	b.diags = append(b.diags, Diagnostic{Line: line, Column: col, Severity: "error", Code: code, Message: msg})
}

func (b *csBuilder) warn(line, col int, code, msg string) {
	b.diags = append(b.diags, Diagnostic{Line: line, Column: col, Severity: "warning", Code: code, Message: msg})
}

func (b *csBuilder) warnStmt(st *csStatement, code, msg string) {
	b.warn(st.line, st.col, code, msg)
}

func (b *csBuilder) ignoreBlock(st *csStatement, reason string) {
	b.ignored = append(b.ignored, st.name)
	b.warnStmt(st, "cs.unsupported_block", fmt.Sprintf("%s block is not applied by ForgeC2 (%s); it is preserved in the source", st.name, reason))
}

func (b *csBuilder) walkTop(stmts []*csStatement) {
	for _, st := range stmts {
		if st.isSet {
			b.topSet(st)
			continue
		}
		switch st.name {
		case "http-get":
			b.get.present = true
			b.httpBlock(st, b.get)
		case "http-post":
			b.post.present = true
			b.httpBlock(st, b.post)
		case "http-config":
			b.httpConfig(st)
		case "forgec2":
			b.forgec2Block(st)
		case "http-stager":
			b.ignoreBlock(st, "ForgeC2 uses its own stager/one-liner pipeline")
		case "post-ex", "process-inject", "stage", "ssl-certificate", "dns-beacon", "ssh", "tcp":
			b.ignoreBlock(st, "no equivalent in the ForgeC2 HTTP beacon")
		default:
			b.warnStmt(st, "cs.unknown_block", fmt.Sprintf("unknown block %q is ignored", st.name))
		}
	}
}

func (b *csBuilder) topSet(st *csStatement) {
	if len(st.args) == 0 {
		b.warnStmt(st, "cs.empty_set", "set statement has no key")
		return
	}
	key := strings.ToLower(st.args[0])
	vals := st.args[1:]
	first := ""
	if len(vals) > 0 {
		first = vals[0]
	}
	switch key {
	case "sleeptime":
		n, err := strconv.Atoi(strings.TrimSpace(first))
		if err != nil || n < 0 {
			b.warnStmt(st, "cs.bad_sleeptime", fmt.Sprintf("sleeptime %q is not a non-negative integer; ignored", first))
			return
		}
		b.sleepMS, b.sleepSet = n, true
	case "jitter":
		n, err := strconv.Atoi(strings.TrimSpace(first))
		if err != nil {
			b.warnStmt(st, "cs.bad_jitter", fmt.Sprintf("jitter %q is not an integer; ignored", first))
			return
		}
		b.jitter, b.jitSet = n, true
	case "useragent", "user_agent":
		b.v2.UserAgent = first
	case "description":
		b.v2.Description = first
	case "sample_name":
		// CS sample names are not ForgeC2 profile names; keep only as metadata.
		if b.v2.Description == "" {
			b.v2.Description = first
		}
	case "host_stage", "dns_idle", "dns_max_txt", "dns_sleep", "dns_ttl",
		"dns_stager_prepend", "dns_stager_subhost", "maxdns",
		"pipename", "smb_frame_header", "tcp_frame_header", "ssh_banner",
		"ssh_pipename", "callback_host", "http_proxy":
		b.warnStmt(st, "cs.unsupported_set", fmt.Sprintf("set %s is not applied by ForgeC2", key))
	default:
		b.warnStmt(st, "cs.unsupported_set", fmt.Sprintf("unknown set %q is ignored", key))
	}
}

func (b *csBuilder) httpBlock(st *csStatement, blk *csHTTPBlock) {
	for _, child := range st.children {
		if child.isSet {
			b.httpSet(child, blk)
			continue
		}
		switch child.name {
		case "client":
			b.httpClient(child, blk)
		case "server":
			b.httpServer(child, blk)
		case "header":
			b.addHeader(blk, child)
		default:
			b.warnStmt(child, "cs.unknown_http_statement", fmt.Sprintf("unknown %s statement %q is ignored", st.name, child.name))
		}
	}
}

func (b *csBuilder) httpSet(st *csStatement, blk *csHTTPBlock) {
	if len(st.args) == 0 {
		b.warnStmt(st, "cs.empty_set", "set statement has no key")
		return
	}
	key := strings.ToLower(st.args[0])
	vals := st.args[1:]
	first := ""
	if len(vals) > 0 {
		first = vals[0]
	}
	switch key {
	case "uri":
		for _, u := range vals {
			u = strings.TrimSpace(u)
			if u != "" {
				blk.uris = append(blk.uris, u)
			}
		}
	case "verb":
		blk.verb = first
	case "parameter", "idparameter":
		blk.param = first
	case "useragent", "user_agent":
		b.v2.UserAgent = first
	case "host", "ssl", "uri_x86", "uri_x64":
		b.warnStmt(st, "cs.unsupported_http_set", fmt.Sprintf("set %s is not applied to ForgeC2 beacons", key))
	default:
		b.warnStmt(st, "cs.unsupported_http_set", fmt.Sprintf("unknown http set %q is ignored", key))
	}
}

func (b *csBuilder) httpClient(st *csStatement, blk *csHTTPBlock) {
	for _, child := range st.children {
		if child.isSet {
			b.httpSet(child, blk)
			continue
		}
		switch child.name {
		case "header":
			b.addHeader(blk, child)
		case "metadata":
			blk.metadata = append(blk.metadata, b.parseSteps(child)...)
		case "id":
			blk.id = append(blk.id, b.parseSteps(child)...)
		case "parameter":
			if len(child.args) > 0 {
				blk.param = child.args[0]
			}
		default:
			b.warnStmt(child, "cs.unknown_client_statement", fmt.Sprintf("unknown client statement %q is ignored", child.name))
		}
	}
}

func (b *csBuilder) httpServer(st *csStatement, blk *csHTTPBlock) {
	for _, child := range st.children {
		if child.isSet {
			b.warnStmt(child, "cs.unsupported_server_set", fmt.Sprintf("unknown server set %q is ignored", strings.Join(child.args, " ")))
			continue
		}
		switch child.name {
		case "output":
			blk.output = append(blk.output, b.parseSteps(child)...)
		case "header":
			b.warnStmt(child, "cs.server_header", "server response headers are configured in Settings → Malleable, not on the agent profile")
		default:
			b.warnStmt(child, "cs.unknown_server_statement", fmt.Sprintf("unknown server statement %q is ignored", child.name))
		}
	}
}

func (b *csBuilder) addHeader(blk *csHTTPBlock, st *csStatement) {
	if len(st.args) == 0 {
		b.warnStmt(st, "cs.bad_header", "header statement needs a name")
		return
	}
	value := ""
	if len(st.args) > 1 {
		value = st.args[1]
	} else {
		b.warnStmt(st, "cs.header_without_value", fmt.Sprintf("header %q has no value", st.args[0]))
	}
	blk.headers[st.args[0]] = value
}

func (b *csBuilder) httpConfig(st *csStatement) {
	for _, child := range st.children {
		if child.isSet {
			if len(child.args) == 0 {
				continue
			}
			key := strings.ToLower(child.args[0])
			first := ""
			if len(child.args) > 1 {
				first = child.args[1]
			}
			switch key {
			case "useragent", "user_agent":
				b.v2.UserAgent = first
			case "headers", "trust_x_forwarded_for", "block_invoke", "block_retry", "block_timeout":
				b.warnStmt(child, "cs.server_config", fmt.Sprintf("http-config set %s is server-side; configure Settings → Malleable", key))
			default:
				b.warnStmt(child, "cs.unsupported_http_config", fmt.Sprintf("unknown http-config set %q is ignored", key))
			}
			continue
		}
		if child.name == "header" {
			b.warnStmt(child, "cs.server_config", "http-config headers are server-side; configure Settings → Malleable")
			continue
		}
		b.warnStmt(child, "cs.unknown_http_config", fmt.Sprintf("unknown http-config statement %q is ignored", child.name))
	}
}

func (b *csBuilder) parseSteps(block *csStatement) []StepV2 {
	var steps []StepV2
	for _, st := range block.children {
		if st.isSet {
			b.warnStmt(st, "cs.transform_set", fmt.Sprintf("set statement %q inside a transform block is ignored", strings.Join(st.args, " ")))
			continue
		}
		name := normalizeTransform(st.name)
		if name == "" {
			continue
		}
		if !allowedStepTypes[name] {
			b.warnStmt(st, "cs.unknown_transform", fmt.Sprintf("unknown transform %q is ignored", st.name))
			continue
		}
		value := ""
		switch name {
		case "strrep":
			if len(st.args) < 2 {
				b.warnStmt(st, "cs.bad_transform", "strrep requires two quoted values")
				continue
			}
			value = st.args[0] + ":" + st.args[1]
		case "append", "prepend", "xor", "mask", "header", "parameter", "case":
			if len(st.args) == 0 {
				b.warnStmt(st, "cs.bad_transform", fmt.Sprintf("%s requires a value", name))
				continue
			}
			value = st.args[0]
		default:
			if len(st.args) > 0 {
				b.warnStmt(st, "cs.extra_transform_arg", fmt.Sprintf("%s does not take a value; using %q", name, st.args[0]))
				value = st.args[0]
			}
		}
		steps = append(steps, StepV2{Type: name, Value: value})
	}
	return steps
}

func normalizeTransform(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.ReplaceAll(n, "-", "_")
	return n
}

func (b *csBuilder) forgec2Block(st *csStatement) {
	for _, child := range st.children {
		if !child.isSet {
			b.warnStmt(child, "cs.forgec2_block", fmt.Sprintf("nested block %q inside forgec2 is ignored", child.name))
			continue
		}
		if len(child.args) == 0 {
			continue
		}
		key := strings.ToLower(child.args[0])
		vals := child.args[1:]
		first := ""
		if len(vals) > 0 {
			first = vals[0]
		}
		switch key {
		case "prepend":
			b.extPrepend = first
		case "append":
			b.extAppend = first
		case "request_prepend":
			b.extRequestPrepend = first
		case "request_append":
			b.extRequestAppend = first
		case "content_length_jitter":
			n, err := strconv.Atoi(strings.TrimSpace(first))
			if err != nil {
				b.warnStmt(child, "cs.bad_extension", "content_length_jitter must be an integer")
				break
			}
			b.extContentJitter = n
		case "jitter_uri":
			b.extJitterURI = parseCSBool(first)
		case "jitter_parameter":
			b.extJitterParam = parseCSBool(first)
		case "parameter":
			b.extParameter = first
		case "work_start":
			b.extWorkStart = first
		case "work_end":
			b.extWorkEnd = first
		case "work_tz":
			b.extWorkTZ = first
		case "user_agents":
			b.extUserAgents = append(b.extUserAgents, vals...)
		case "parameter_names":
			b.extParameterNames = append(b.extParameterNames, vals...)
		case "request_header":
			if len(vals) < 2 {
				b.warnStmt(child, "cs.bad_extension", "request_header requires a name and value")
				break
			}
			if b.extRequestHeaders == nil {
				b.extRequestHeaders = map[string]string{}
			}
			b.extRequestHeaders[vals[0]] = vals[1]
		case "request_header_pool":
			b.extHeaderPool = append(b.extHeaderPool, vals...)
		case "placement":
			if len(vals) < 2 {
				b.warnStmt(child, "cs.bad_extension", "placement requires target and chain")
				break
			}
			b.extPlacements = append(b.extPlacements, PlacementV2{Target: vals[0], Chain: vals[1]})
		case "client_id":
			b.extClientID = ParseWire(first)
			b.extClientIDSet = true
		case "client_metadata":
			b.extClientMetadata = ParseWire(first)
			b.extClientMetaSet = true
		case "server_output":
			b.extServerOutput = ParseWire(first)
			b.extServerOutputSet = true
		case "header":
			if len(vals) < 2 {
				b.warnStmt(child, "cs.bad_extension", "header requires a name and value")
				break
			}
			if b.extHeaders == nil {
				b.extHeaders = map[string]string{}
			}
			b.extHeaders[vals[0]] = vals[1]
		case "method":
			b.extMethod = strings.ToUpper(first)
			b.extMethodSet = true
		case "beacon_uri":
			b.extBeaconURI = first
			b.extBeaconURISet = true
		case "description":
			b.v2.Description = first
		default:
			b.warnStmt(child, "cs.unknown_extension", fmt.Sprintf("unknown forgec2 set %q is ignored", key))
		}
	}
}

func parseCSBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (b *csBuilder) finish() {
	method := ""
	if b.post.present {
		method = strings.ToUpper(strings.TrimSpace(b.post.verb))
		if method == "" {
			method = "POST"
		}
	} else if b.get.present {
		method = strings.ToUpper(strings.TrimSpace(b.get.verb))
		if method == "" {
			method = "GET"
		}
	}
	if method == "" {
		b.warn(1, 1, "cs.no_http_block", "profile has no http-get or http-post block; ForgeC2 will use its default HTTP method")
	}
	if method != "" {
		b.v2.Method = method
		b.v2.Verb = method
	}
	verbSeen := map[string]bool{}
	for _, v := range []string{b.get.verb, b.post.verb, method} {
		v = strings.ToUpper(strings.TrimSpace(v))
		if v != "" && !verbSeen[v] {
			verbSeen[v] = true
			b.v2.Verbs = append(b.v2.Verbs, v)
		}
	}

	uris := make([]string, 0, len(b.get.uris)+len(b.post.uris))
	if method == "GET" {
		uris = append(uris, b.get.uris...)
		uris = append(uris, b.post.uris...)
	} else {
		uris = append(uris, b.post.uris...)
		uris = append(uris, b.get.uris...)
	}
	uris = dedupStrings(uris)
	if len(uris) > 0 {
		b.v2.URIs = uris
		b.v2.BeaconURIs = append([]string{}, uris...)
		b.v2.BeaconURI = uris[0]
	}

	// Merge headers with the method's block taking precedence.
	primary, secondary := b.get.headers, b.post.headers
	if method != "GET" {
		primary, secondary = b.post.headers, b.get.headers
	}
	if len(primary) > 0 || len(secondary) > 0 {
		b.v2.Headers = map[string]string{}
		for k, v := range secondary {
			b.v2.Headers[k] = v
		}
		for k, v := range primary {
			b.v2.Headers[k] = v
		}
	}
	for k, v := range b.extHeaders {
		if b.v2.Headers == nil {
			b.v2.Headers = map[string]string{}
		}
		b.v2.Headers[k] = v
	}

	if len(b.get.metadata) > 0 {
		b.v2.ClientMetadata = b.get.metadata
	}
	if len(b.post.id) > 0 {
		b.v2.ClientID = b.post.id
	}
	if len(b.post.output) > 0 {
		b.v2.ServerOutput = b.post.output
	} else if len(b.get.output) > 0 {
		b.v2.ServerOutput = b.get.output
	}
	if b.extClientMetaSet {
		b.v2.ClientMetadata = b.extClientMetadata
	}
	if b.extClientIDSet {
		b.v2.ClientID = b.extClientID
	}
	if b.extServerOutputSet {
		b.v2.ServerOutput = b.extServerOutput
	}
	if b.post.param != "" {
		b.v2.Parameter = b.post.param
	} else if b.get.param != "" {
		b.v2.Parameter = b.get.param
	}
	if b.v2.Parameter == "" {
		for _, s := range b.v2.ClientID {
			if s.Type == "parameter" && s.Value != "" {
				b.v2.Parameter = s.Value
				break
			}
		}
	}

	if b.sleepSet {
		if b.sleepMS == 0 {
			b.v2.Sleep = 0
		} else {
			secs := (b.sleepMS + 500) / 1000
			if secs < 1 {
				secs = 1
			}
			if b.sleepMS%1000 != 0 {
				b.warn(1, 1, "cs.sleeptime_rounded", fmt.Sprintf("sleeptime %dms is not a whole second; rounded to %ds", b.sleepMS, secs))
			}
			b.v2.Sleep = secs
		}
	}
	if b.jitSet {
		b.v2.Jitter = b.jitter
	}

	if b.extPrepend != "" {
		b.v2.Prepend = b.extPrepend
	}
	if b.extAppend != "" {
		b.v2.Append = b.extAppend
	}
	if b.extRequestPrepend != "" {
		b.v2.RequestPrepend = b.extRequestPrepend
	}
	if b.extRequestAppend != "" {
		b.v2.RequestAppend = b.extRequestAppend
	}
	if len(b.extRequestHeaders) > 0 {
		b.v2.RequestHeaders = b.extRequestHeaders
	}
	if len(b.extHeaderPool) > 0 {
		b.v2.RequestHeaderPool = b.extHeaderPool
	}
	if len(b.extUserAgents) > 0 {
		b.v2.UserAgents = b.extUserAgents
	}
	if len(b.extParameterNames) > 0 {
		b.v2.ParameterNames = b.extParameterNames
	}
	if len(b.extPlacements) > 0 {
		b.v2.Placements = b.extPlacements
	}
	if b.extContentJitter != 0 {
		b.v2.ContentLengthJitter = b.extContentJitter
	}
	if b.extJitterURI {
		b.v2.JitterURI = true
	}
	if b.extJitterParam {
		b.v2.JitterParameter = true
	}
	if b.extParameter != "" {
		b.v2.Parameter = b.extParameter
	}
	if b.extWorkStart != "" {
		b.v2.WorkStart = b.extWorkStart
	}
	if b.extWorkEnd != "" {
		b.v2.WorkEnd = b.extWorkEnd
	}
	if b.extWorkTZ != "" {
		b.v2.WorkTZ = b.extWorkTZ
	}
	if b.extMethodSet && b.extMethod != "" {
		b.v2.Method = b.extMethod
		b.v2.Verb = b.extMethod
	}
	if b.extBeaconURISet && b.extBeaconURI != "" {
		b.v2.BeaconURI = b.extBeaconURI
		b.v2.URIs = prependUniqueURI(b.extBeaconURI, b.v2.URIs)
		b.v2.BeaconURIs = prependUniqueURI(b.extBeaconURI, b.v2.BeaconURIs)
	}
}

func prependUniqueURI(uri string, list []string) []string {
	out := make([]string, 0, len(list)+1)
	out = append(out, uri)
	for _, item := range list {
		if item != uri {
			out = append(out, item)
		}
	}
	return out
}
