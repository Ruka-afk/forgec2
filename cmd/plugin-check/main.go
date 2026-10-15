// Command plugin-check validates a ForgeC2 plugin package the way the server
// loads it: manifest, entry, package digest/signature, tier-specific structure
// (the WASM ABI surface is inspected with the server's own runtime) and,
// with -run, one real execution through the server's executor followed by the
// output-contract check for the plugin's type.
//
// Every finding has a stable code (manifest.invalid, wasm.bad_abi,
// result.bad_json, …) and -json prints the whole report as JSON, so the tool
// is as usable from an agent loop as from a terminal. The codes are documented
// in docs/PLUGIN_AUTHORING.md.
//
//	go run ./cmd/plugin-check ./my-plugin            # static checks
//	go run ./cmd/plugin-check ./my-plugin -run       # + execute it once
//	go run ./cmd/plugin-check ./my-plugin -json      # machine-readable report
//
// Exit code 0 means no error-level findings; warnings do not fail the check.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/forgec2/forgec2/internal/plugin"
)

func main() {
	run := flag.Bool("run", false, "execute the plugin once with a sample input after the static checks (only for packages you trust)")
	input := flag.String("input", "", "raw JSON input for -run (default: the manifest's own defaults)")
	timeout := flag.Int("timeout", 0, "run timeout in seconds (default: the manifest's)")
	root := flag.String("root", "", "package tree root anchoring manifest.includes (default: the package directory)")
	keys := flag.String("key", "", "comma-separated hex Ed25519 public keys for signature verification")
	requireSigned := flag.Bool("require-signed", false, "fail unsigned packages (mirrors plugins.require_signed)")
	asJSON := flag.Bool("json", false, "print the machine-readable report instead of text")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		usage()
		os.Exit(2)
	}
	dir := strings.TrimSpace(args[0])
	// Go's flag package stops at the first positional argument; accept the
	// natural `plugin-check ./dir -run` reading order by parsing the rest.
	if rest := args[1:]; len(rest) > 0 {
		_ = flag.CommandLine.Parse(rest)
	}

	opts := plugin.CheckOptions{
		Dir:           dir,
		Root:          strings.TrimSpace(*root),
		Run:           *run,
		Input:         *input,
		Timeout:       *timeout,
		RequireSigned: *requireSigned,
	}
	if *keys != "" {
		opts.TrustedKeys = strings.Split(*keys, ",")
	}

	report := plugin.CheckPlugin(opts)
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintln(os.Stderr, "plugin-check: "+err.Error())
			os.Exit(1)
		}
	} else {
		printText(report)
	}
	if !report.OK() {
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: plugin-check [flags] <plugin-directory>")
	fmt.Fprintln(os.Stderr, "flags:")
	flag.PrintDefaults()
}

func printText(report *plugin.CheckReport) {
	if report.Name == "" {
		fmt.Println(report.Dir)
	} else {
		fmt.Printf("%s: name=%s version=%s type=%s interpreter=%s entry=%s\n",
			report.Dir, report.Name, report.Version, report.Type, report.Interpreter, report.Entry)
	}
	for _, d := range report.Diagnostics {
		fmt.Printf("  [%-7s] %-26s %s\n", d.Level, d.Code, d.Message)
	}
	if report.Ran && (report.Stdout != "" || report.Stderr != "") {
		fmt.Println("  --- run output ---")
		if report.Stdout != "" {
			fmt.Println("  stdout: " + firstLines(report.Stdout, 8))
		}
		if report.Stderr != "" {
			fmt.Println("  stderr: " + firstLines(report.Stderr, 8))
		}
	}
	summary := fmt.Sprintf("%d error(s), %d warning(s)", report.Errors, report.Warnings)
	if report.OK() {
		fmt.Println("OK: " + summary)
		return
	}
	fmt.Println("FAIL: " + summary)
}

// firstLines keeps the terminal honest for plugins that print a wall of text.
func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		return strings.Join(lines[:n], "\n         ") + fmt.Sprintf("\n         … (%d more lines)", len(lines)-n)
	}
	return strings.Join(lines, "\n         ")
}
