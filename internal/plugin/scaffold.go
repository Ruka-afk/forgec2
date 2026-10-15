package plugin

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Plugin authoring: this file generates plugin packages that are compatible
// with the server *by construction*. cmd/plugin-init is the CLI front end;
// cmd/plugin-check (check.go) is the validator to iterate against.
//
// The generated code documents the runtime contract in the language the author
// (human or AI) is about to edit, because that is where the contract is needed:
//
//	stdin   one JSON object: {"params": …, "config": …} (plus "agent_id" for
//	        command plugins, plus "event" for hook plugins)
//	stdout  one JSON object: Result (command/hook) or Report (report)
//	cwd     the package directory; HOME/TMP* point at the OS temp dir
//
// Every template is a valid plugin on its own: `plugin-check -run` passes on a
// fresh scaffold, so the author starts from green and changes one thing at a
// time.

// ScaffoldOptions describes the package to generate. Name, Type, Interpreter
// and Dir are required.
type ScaffoldOptions struct {
	// Name is the plugin name used in the manifest, the UI and `requires`
	// lists. Lowercase letters, digits, '-', '_' and '.'.
	Name string
	// Type is one of ValidPluginTypes: command, hook or report.
	Type string
	// Interpreter is one of ValidInterpreters; "wasm" selects the sandboxed
	// tier and adds a C skeleton plus a build script.
	Interpreter string
	// Dir is the package directory to create.
	Dir string
	// Description is a one-line summary shown in the UI.
	Description string
	// Timeout is the per-run wall-clock limit in seconds (0 = a type default).
	Timeout int
	// Force writes into a non-empty directory.
	Force bool
}

// ScaffoldResult reports what a scaffold wrote.
type ScaffoldResult struct {
	Dir   string
	Files []string
	// BuildHint is the command that produces the manifest entry when the tier
	// needs a build step (wasm), or empty.
	BuildHint string
	// Notes are advisories about the generated package (for example which
	// interpreter auto-detection picked, and why that matters for portability).
	Notes []string
	// NextSteps are the commands worth running next (check, sign, install).
	NextSteps []string
}

// pluginNamePattern keeps generated packages portable: the name ends up in
// file paths, URLs and shell-quoted commands on every supported platform.
var pluginNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// defaultScaffoldTimeout mirrors the bundled plugins: hooks fire inside event
// delivery and must be quick, reports scan the database.
func defaultScaffoldTimeout(typ string) int {
	switch typ {
	case "hook":
		return 30
	case "report":
		return 120
	default:
		return 60
	}
}

// scaffoldInterpreterCandidates returns the interpreters to try when the
// caller did not name one, most-wanted first.
func scaffoldInterpreterCandidates() []string {
	if runtime.GOOS == "windows" {
		return []string{"python", "python3", "py", "pwsh", "powershell", "go", "sh", "bash"}
	}
	return []string{"python3", "python", "pwsh", "powershell", "bash", "sh", "go"}
}

// detectInterpreter picks the first interpreter that actually runs on this
// host, so a scaffold works where it was generated instead of producing a
// package whose manifest names an interpreter this machine cannot execute.
func detectInterpreter() string {
	for _, candidate := range scaffoldInterpreterCandidates() {
		if !isValidValue(candidate, ValidInterpreters) {
			continue
		}
		if err := probeInterpreter(candidate); err == nil {
			return candidate
		}
	}
	return "python"
}

// entryName maps an interpreter to the file the manifest will point at.
func entryName(interpreter string) string {
	switch {
	case isWasmInterpreter(interpreter):
		return "plugin.wasm"
	case strings.HasPrefix(interpreter, "python"):
		return "main.py"
	case interpreter == "bash" || interpreter == "sh":
		return "main.sh"
	case interpreter == "powershell" || interpreter == "pwsh":
		return "main.ps1"
	case interpreter == "go":
		return "main.go"
	}
	return ""
}

// ScaffoldPlugin writes a plugin package. The result is valid by construction:
// CheckPlugin reports no errors for it (the wasm tier still needs its build
// step, which BuildHint names).
func ScaffoldPlugin(opts ScaffoldOptions) (*ScaffoldResult, error) {
	opts.Name = strings.TrimSpace(opts.Name)
	opts.Type = strings.TrimSpace(strings.ToLower(opts.Type))
	opts.Interpreter = strings.TrimSpace(strings.ToLower(opts.Interpreter))
	opts.Dir = strings.TrimSpace(opts.Dir)
	opts.Description = strings.TrimSpace(opts.Description)

	if opts.Name == "" {
		return nil, fmt.Errorf("plugin name is required (-name)")
	}
	if !pluginNamePattern.MatchString(opts.Name) {
		return nil, fmt.Errorf("plugin name %q must be lowercase letters, digits, '-', '_' or '.', at most 64 chars, starting with a letter or digit", opts.Name)
	}
	if !isValidValue(opts.Type, ValidPluginTypes) {
		return nil, fmt.Errorf("plugin type %q must be one of %v (-type)", opts.Type, ValidPluginTypes)
	}
	autoInterpreter := opts.Interpreter == ""
	if autoInterpreter {
		opts.Interpreter = detectInterpreter()
	}
	if !isValidValue(opts.Interpreter, ValidInterpreters) {
		return nil, fmt.Errorf("interpreter %q must be one of %v (-interpreter)", opts.Interpreter, ValidInterpreters)
	}
	if opts.Dir == "" {
		return nil, fmt.Errorf("destination directory is required (-dir)")
	}
	if opts.Description == "" {
		opts.Description = "TODO: one-line description of what this plugin does"
	}
	if opts.Timeout <= 0 {
		opts.Timeout = defaultScaffoldTimeout(opts.Type)
	}
	if entries, err := os.ReadDir(opts.Dir); err == nil && len(entries) > 0 && !opts.Force {
		return nil, fmt.Errorf("directory %s is not empty; pass -force to write into it", opts.Dir)
	}

	files, buildHint, err := scaffoldFiles(opts)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(opts.Dir, 0750); err != nil {
		return nil, fmt.Errorf("create %s: %w", opts.Dir, err)
	}
	written := make([]string, 0, len(files))
	// manifest.yaml first: it is the file the author and the server read.
	order := []string{"manifest.yaml"}
	for name := range files {
		if name != "manifest.yaml" {
			order = append(order, name)
		}
	}
	for _, name := range order {
		path := filepath.Join(opts.Dir, filepath.FromSlash(name))
		//nolint:gosec // 0644 matches what Manifest.Save writes for the rest of the plugin tree
		if err := os.WriteFile(path, []byte(files[name]), 0644); err != nil {
			return nil, fmt.Errorf("write %s: %w", path, err)
		}
		written = append(written, path)
	}

	steps := []string{
		fmt.Sprintf("validate:  go run ./cmd/plugin-check %s -run", opts.Dir),
		fmt.Sprintf("sign:      PLUGIN_SIGNING_KEY=<seed> go run ./cmd/sign-plugin %s   (keys: go run ./cmd/sign-plugin -gen)", opts.Dir),
		fmt.Sprintf("install:   copy %s into the server's plugins tree (or upload it in the UI); it loads on start and refresh", opts.Dir),
	}
	var notes []string
	if autoInterpreter {
		notes = append(notes, fmt.Sprintf("interpreter auto-detected as %q: the first one that actually runs on this host; pass -interpreter to name another (the manifest must name an interpreter that runs on the server host, not this one)", opts.Interpreter))
	}
	return &ScaffoldResult{Dir: opts.Dir, Files: written, BuildHint: buildHint, Notes: notes, NextSteps: steps}, nil
}

// scaffoldFiles builds the package contents: manifest first, then the entry
// source, then tier extras (the wasm build script).
func scaffoldFiles(opts ScaffoldOptions) (map[string]string, string, error) {
	entry := entryName(opts.Interpreter)
	if entry == "" {
		return nil, "", fmt.Errorf("no scaffold template for interpreter %q", opts.Interpreter)
	}
	files := map[string]string{
		"manifest.yaml": scaffoldManifest(opts, entry),
	}
	buildHint := ""
	if isWasmInterpreter(opts.Interpreter) {
		files["plugin.c"] = wasmCSource(opts)
		files["build.sh"] = wasmBuildScript()
		buildHint = fmt.Sprintf("sh %s/build.sh", filepath.ToSlash(strings.TrimSuffix(opts.Dir, "/")))
	} else {
		switch {
		case strings.HasPrefix(opts.Interpreter, "python"):
			files[entry] = pythonSource(opts)
		case opts.Interpreter == "bash" || opts.Interpreter == "sh":
			files[entry] = shellSource(opts)
		case opts.Interpreter == "powershell" || opts.Interpreter == "pwsh":
			files[entry] = powershellSource(opts)
		case opts.Interpreter == "go":
			files[entry] = goSource(opts)
		default:
			return nil, "", fmt.Errorf("no scaffold template for interpreter %q", opts.Interpreter)
		}
	}
	return files, buildHint, nil
}

// scaffoldManifest renders a manifest that satisfies Manifest.Validate and the
// conventions of the bundled plugins.
func scaffoldManifest(opts ScaffoldOptions, entry string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# ForgeC2 plugin manifest. Contract: docs/PLUGIN_AUTHORING.md\n")
	fmt.Fprintf(&b, "# Machine-readable schema: docs/schemas/plugin-manifest.schema.json\n")
	fmt.Fprintf(&b, "# Validate: go run ./cmd/plugin-check %s -run\n", opts.Dir)
	fmt.Fprintf(&b, "name: %s\n", opts.Name)
	fmt.Fprintf(&b, "version: 0.1.0\n")
	fmt.Fprintf(&b, "type: %s\n", opts.Type)
	fmt.Fprintf(&b, "description: %s\n", yamlScalar(opts.Description))
	fmt.Fprintf(&b, "interpreter: %s\n", opts.Interpreter)
	fmt.Fprintf(&b, "entry: %s\n", entry)
	fmt.Fprintf(&b, "timeout: %d\n", opts.Timeout)
	if opts.Type == "hook" {
		fmt.Fprintf(&b, "params: []\n")
		fmt.Fprintf(&b, "# Events this hook subscribes to; an empty list means it never fires.\n")
		fmt.Fprintf(&b, "events:\n  - agent.connect\n")
	} else {
		fmt.Fprintf(&b, "# Parameters the operator fills in before a run; read from params.\n")
		fmt.Fprintf(&b, "params:\n")
		fmt.Fprintf(&b, "  - name: target\n")
		fmt.Fprintf(&b, "    type: string\n")
		fmt.Fprintf(&b, "    required: false\n")
		fmt.Fprintf(&b, "    default: localhost\n")
		fmt.Fprintf(&b, "    description: Example parameter - rename it and read it from params\n")
	}
	fmt.Fprintf(&b, "# Default configuration handed to the plugin as `config` (optional).\n")
	fmt.Fprintf(&b, "# config:\n")
	fmt.Fprintf(&b, "#   example_key: example value\n")
	// The digest/signature fields are written by cmd/sign-plugin; note it here
	// so the author does not hand-craft them.
	fmt.Fprintf(&b, "# digest/signature: written by cmd/sign-plugin, never by hand.\n")
	return b.String()
}

// yamlScalar quotes a description that would otherwise change the YAML shape.
func yamlScalar(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, ":#'\"\n\t{}[]&*!|>%@`") {
		return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
	}
	return s
}

// scaffoldDocs is the inline contract comment every template starts with.
func scaffoldHeader(opts ScaffoldOptions) string {
	return fmt.Sprintf(`%s - %s

ForgeC2 plugin contract (https://github.com/forgec2/forgec2, docs/PLUGIN_AUTHORING.md):
  * the plugin runs as a child process with its own directory as the working
    directory; HOME/TMP point at the OS temp directory, not the operator profile
  * stdin  carries ONE JSON object: {"params": {...}, "config": {...}} for
    command/report plugins, {"event": {...}, "config": {...}} for hooks; a
    command plugin also receives "agent_id"
  * stdout must carry ONE JSON object: a Result
    {"success": true, "output": "...", "data": {...}} for command plugins and
    hooks, or a Report {"title": ..., "format": "markdown|html|json",
    "content": <base64>} for report plugins
  * anything printed to stdout that is not that object is a bug: diagnostics go
    to stderr, where the server captures them and shows them on failure
  * the run is bounded by the manifest timeout plus the server's CPU and
    write budgets; a plugin that spins or floods is killed
`, opts.Name, opts.Description)
}

// commentLines prefixes every line of s (shell, PowerShell and Go embed the
// shared contract as line comments).
func commentLines(prefix, s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = strings.TrimRight(prefix, " ")
			continue
		}
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

// ── python ──────────────────────────────────────────────────────────────────

func pythonSource(opts ScaffoldOptions) string {
	var body string
	switch opts.Type {
	case "hook":
		body = fmt.Sprintf(`    payload = json.load(sys.stdin)
    event = payload.get("event") or {}
    config = payload.get("config") or {}

    event_type = str(event.get("type", "unknown"))
    # TODO: your logic here. Keep it fast - hooks fire inside event delivery,
    # and the server trips a breaker on repeated failures.
    result = {
        "success": True,
        "output": f"%s: saw {event_type}",
        "data": {"event": event_type, "config_keys": sorted(config)},
    }
    print(json.dumps(result))
    return 0`, opts.Name)
	case "report":
		body = fmt.Sprintf(`    payload = json.load(sys.stdin)
    params = payload.get("params") or {}

    title = str(params.get("title") or "%s")
    # TODO: your logic here. Gather the data, then render it.
    markdown = "# " + title + "\n\nReplace this scaffold with the real report.\n"

    report = {
        "title": title,
        "format": "markdown",
        # Report content is raw bytes in the wire format, so JSON carries it
        # base64-encoded. markdown/html/json are all fine as the format.
        "content": base64.b64encode(markdown.encode("utf-8")).decode("ascii"),
    }
    print(json.dumps(report))
    return 0`, opts.Name)
	default:
		body = fmt.Sprintf(`    payload = json.load(sys.stdin)
    params = payload.get("params") or {}
    config = payload.get("config") or {}

    target = str(params.get("target") or "localhost")

    # TODO: your logic here. Keep stdout small: the whole output is the
    # operator-visible result and is capped by the server.
    result = {
        "success": True,
        "output": f"%s: hello from {target}",
        "data": {"target": target, "config_keys": sorted(config)},
    }
    print(json.dumps(result))
    return 0`, opts.Name)
	}
	return fmt.Sprintf(`#!/usr/bin/env python3
"""%s"""
import base64
import json
import sys


def main() -> int:
%s


if __name__ == "__main__":
    sys.exit(main())
`, scaffoldHeader(opts), body)
}

// ── shell ───────────────────────────────────────────────────────────────────

func shellSource(opts ScaffoldOptions) string {
	content := base64.StdEncoding.EncodeToString([]byte("# " + opts.Name + "\n\nReplace this scaffold with the real report.\n"))
	var body string
	switch opts.Type {
	case "hook":
		body = fmt.Sprintf(`# Hooks receive {"event": {...}, "config": {...}} on stdin. Parsing JSON in
# shell is a trap: this scaffold ignores the payload on purpose. Switch the
# manifest to python/go if you need event fields.
input="$(cat)"
printf '{"success": true, "output": "%s: hook scaffold (payload ignored)"}\n'`, opts.Name)
	case "report":
		// The JSON is baked in as a literal: no shell printf directives, no
		// quoting rules to get wrong. Single quotes are safe here because the
		// name pattern and base64 exclude them.
		json := fmt.Sprintf(`{"title": "%s", "format": "markdown", "content": "%s"}`, opts.Name, content)
		body = fmt.Sprintf(`# A report plugin must print {"title", "format", "content"} where content is
# base64-encoded bytes. The literal below is the scaffold text, base64ed.
input="$(cat)"
printf '%%s\n' '%s'`, json)
	default:
		body = fmt.Sprintf(`# The payload is one JSON object on stdin: {"agent_id", "params", "config"}.
input="$(cat)"

# Parsing JSON in pure shell is not worth it for anything non-trivial: the
# python3 fallback below reads params.target, and stays a no-op if python3 is
# missing. For real logic, switch the manifest to interpreter: python or go.
target="localhost"
if command -v python3 >/dev/null 2>&1; then
    target="$(printf '%%s' "$input" | python3 -c 'import json,sys; print((json.load(sys.stdin).get("params") or {}).get("target") or "localhost")' 2>/dev/null || printf 'localhost')"
fi

# TODO: your logic here. Diagnostics belong on stderr (>&2).
printf '{"success": true, "output": "%%s: hello from %%s"}\n' "${PLUGIN_NAME:-%s}" "$target"`, opts.Name)
	}
	return fmt.Sprintf(`#!/usr/bin/env sh
%s

set -eu

%s
`, commentLines("# ", scaffoldHeader(opts)), body)
}

// ── powershell ──────────────────────────────────────────────────────────────

func powershellSource(opts ScaffoldOptions) string {
	content := base64.StdEncoding.EncodeToString([]byte("# " + opts.Name + "\n\nReplace this scaffold with the real report.\n"))
	var body string
	switch opts.Type {
	case "hook":
		body = fmt.Sprintf(`$payload = [Console]::In.ReadToEnd() | ConvertFrom-Json
$eventType = 'unknown'
if ($payload.PSObject.Properties.Name -contains 'event' -and $payload.event) { $eventType = [string]$payload.event.type }

# TODO: your logic here. Keep it fast: hooks fire inside event delivery.
@{ success = $true; output = "%s: saw $eventType"; data = @{ event = $eventType } } |
    ConvertTo-Json -Compress -Depth 6`, opts.Name)
	case "report":
		body = fmt.Sprintf(`$payload = [Console]::In.ReadToEnd() | ConvertFrom-Json

# Report content is bytes on the wire, so JSON carries it base64-encoded.
# TODO: render the real report instead of the scaffold text.
$content = "%s"

@{ title = "%s"; format = 'markdown'; content = $content } |
    ConvertTo-Json -Compress -Depth 6`, content, opts.Name)
	default:
		body = fmt.Sprintf(`$payload = [Console]::In.ReadToEnd() | ConvertFrom-Json
$target = 'localhost'
if ($payload.PSObject.Properties.Name -contains 'params' -and $payload.params -and
    $payload.params.PSObject.Properties.Name -contains 'target' -and $payload.params.target) {
    $target = [string]$payload.params.target
}

# TODO: your logic here. Write diagnostics with Write-Error; stdout must stay
# a single JSON object.
@{ success = $true; output = "%s: hello from $target"; data = @{ target = $target } } |
    ConvertTo-Json -Compress -Depth 6`, opts.Name)
	}
	return fmt.Sprintf(`%s

# Fail fast: an unhandled cmdlet error must not fall through to a half-formed
# JSON object on stdout.
$ErrorActionPreference = 'Stop'

%s
`, commentLines("# ", scaffoldHeader(opts)), body)
}

// ── go ──────────────────────────────────────────────────────────────────────

func goSource(opts ScaffoldOptions) string {
	var main string
	switch opts.Type {
	case "hook":
		main = fmt.Sprintf(`	var in struct {
		Event  event                  `+"`json:\"event\"`"+`
		Config map[string]interface{} `+"`json:\"config\"`"+`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&in); err != nil {
		emit(result{Success: false, Error: "bad input: " + err.Error()})
	}
	// TODO: your logic here. Keep it fast: hooks fire inside event delivery.
	emit(result{Success: true, Output: "%s: saw " + in.Event.Type, Data: map[string]interface{}{"event": in.Event.Type}})`, opts.Name)
	case "report":
		main = fmt.Sprintf(`	var in struct {
		Params map[string]interface{} `+"`json:\"params\"`"+`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&in); err != nil {
		fmt.Fprintln(os.Stderr, "bad input:", err)
		os.Exit(1)
	}
	title := "%s"
	// TODO: render the real report instead of the scaffold text.
	markdown := "# " + title + "\n\nReplace this scaffold with the real report.\n"
	emit(report{
		Title:   title,
		Format:  "markdown",
		Content: []byte(markdown), // []byte marshals as base64 in JSON
	})`, opts.Name)
	default:
		main = fmt.Sprintf(`	var in struct {
		AgentID string                 `+"`json:\"agent_id\"`"+`
		Params  map[string]interface{} `+"`json:\"params\"`"+`
		Config  map[string]interface{} `+"`json:\"config\"`"+`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&in); err != nil {
		emit(result{Success: false, Error: "bad input: " + err.Error()})
	}
	target, _ := in.Params["target"].(string)
	if target == "" {
		target = "localhost"
	}
	// TODO: your logic here.
	emit(result{Success: true, Output: "%s: hello from " + target, Data: map[string]interface{}{"target": target}})`, opts.Name)
	}
	return fmt.Sprintf(`%s
//
// The server runs this with `+"`go run main.go`"+` from the package directory, so
// the file is a complete program: no module, no dependencies.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type event struct {
	Type string `+"`json:\"type\"`"+`
}

// result is the command/hook contract.
type result struct {
	Success bool                   `+"`json:\"success\"`"+`
	Output  string                 `+"`json:\"output,omitempty\"`"+`
	Error   string                 `+"`json:\"error,omitempty\"`"+`
	Data    map[string]interface{} `+"`json:\"data,omitempty\"`"+`
}

// report is the report contract; Content marshals as base64.
type report struct {
	Title   string `+"`json:\"title\"`"+`
	Format  string `+"`json:\"format\"`"+`
	Content []byte `+"`json:\"content\"`"+`
}

func main() {
%s
}

// emit writes the one JSON object the server reads from stdout.
func emit(v interface{}) {
	if err := json.NewEncoder(os.Stdout).Encode(v); err != nil {
		fmt.Fprintln(os.Stderr, "encode result:", err)
		os.Exit(1)
	}
}
`, commentLines("// ", scaffoldHeader(opts)), main)
}

// ── wasm (ABI v1) ───────────────────────────────────────────────────────────

// wasmCSource renders the C skeleton. The reply body is a compile-time
// constant: parsing JSON in freestanding C is possible but is exactly the part
// the author is meant to write.
func wasmCSource(opts ScaffoldOptions) string {
	body := ""
	switch opts.Type {
	case "hook":
		body = fmt.Sprintf(`{"success": true, "output": "%s: hook scaffold (payload ignored)"}`, opts.Name)
	case "report":
		content := base64.StdEncoding.EncodeToString([]byte("# " + opts.Name + "\n\nReplace this scaffold with the real report.\n"))
		body = fmt.Sprintf(`{"title": "%s", "format": "markdown", "content": "%s"}`, opts.Name, content)
	default:
		body = fmt.Sprintf(`{"success": true, "output": "%s: replace me"}`, opts.Name)
	}
	return fmt.Sprintf(`/* %s
 *
 * ForgeC2 WASM tier, ABI v1 (docs/PLUGIN_TIERS.md):
 *   run(ptr, len) -> i64   the host wrote the input JSON at in[0..len).
 *                          Return the reply location packed into one i64:
 *                          ptr | (len << 32); return 0 for "no result".
 *                          The reply must be the same JSON object a native
 *                          plugin would print on stdout.
 *   log(ptr, len)          optional host import (module "forgec2"): one
 *                          diagnostic line per call, capped by the host.
 *
 * There is no libc and no WASI: no files, sockets, environment or clock.
 * Build with either toolchain:  sh build.sh
 */
typedef unsigned char u8;
typedef unsigned int u32;
typedef unsigned long long u64;

__attribute__((import_module("forgec2"), import_name("log")))
extern void host_log(const char *ptr, u32 len);

/* The reply lives in the module's data segment. The host writes its input
 * past the initial memory on purpose, so static data can never be clobbered. */
static unsigned char reply[4096];

__attribute__((export_name("run")))
u64 run(const u8 *in, u32 len) {
    (void)in;
    (void)len;
    /* TODO: read the input JSON at in[0..len), then build the reply. */

    static const char body[] = %s;
    static const char logline[] = "%s ran";

    u32 n = (u32)(sizeof(body) - 1);
    for (u32 i = 0; i < n; i++) {
        reply[i] = (unsigned char)body[i];
    }
    host_log(logline, (u32)(sizeof(logline) - 1));

    return ((u64)n << 32) | (u64)(unsigned long)reply;
}
`, scaffoldHeader(opts), cStringLiteral(body), opts.Name)
}

// cStringLiteral quotes s as a C string literal (no newlines in our bodies).
func cStringLiteral(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func wasmBuildScript() string {
	return `#!/usr/bin/env sh
# Build plugin.wasm for the ForgeC2 WASM tier (ABI v1).
#
# Any clang with a wasm32 backend works, with or without a sysroot: the module
# is freestanding (-nostdlib) and declares its own host import.
#
#   sh build.sh                               # clang from PATH
#   CC="zig cc" TARGET=wasm32-freestanding sh build.sh
#   CC=/opt/wasi-sdk/bin/clang sh build.sh    # wasi-sdk also works
set -eu

CC="${CC:-clang}"
TARGET="${TARGET:-wasm32-unknown-unknown}"

if ! command -v "${CC%% *}" >/dev/null 2>&1; then
    echo "build.sh: ${CC%% *} not found; use zig (CC=\"zig cc\" TARGET=wasm32-freestanding)" >&2
    exit 1
fi

# CC may be a wrapper such as "zig cc", so it is deliberately unquoted.
# shellcheck disable=SC2086
$CC \
    --target="$TARGET" \
    -Os -nostdlib \
    -Wl,--no-entry \
    -Wl,--export=run \
    -Wl,--export-memory \
    -Wl,--strip-all \
    -Wl,-z,stack-size=65536 \
    -o plugin.wasm \
    plugin.c

echo "built plugin.wasm:"
ls -l plugin.wasm
`
}
