// Command plugin-init writes a ForgeC2 plugin package skeleton that is valid
// by construction: a manifest, an entry template that documents the runtime
// contract in the language being edited, and (for the WASM tier) a C skeleton
// plus its build script.
//
// The intended loop, for a human or an AI agent:
//
//	go run ./cmd/plugin-init -name my-report -type report -interpreter python -dir ./my-report
//	# edit the generated files
//	go run ./cmd/plugin-check ./my-report -run
//
// docs/PLUGIN_AUTHORING.md carries the contract and the diagnostic codes
// plugin-check emits; docs/PLUGIN_TIERS.md describes what each tier guarantees.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/forgec2/forgec2/internal/plugin"
)

func main() {
	name := flag.String("name", "", "plugin name (lowercase letters, digits, '-', '_' or '.')")
	typ := flag.String("type", "command", "plugin type: "+strings.Join(plugin.ValidPluginTypes, ", "))
	interpreter := flag.String("interpreter", "", "plugin interpreter: "+strings.Join(plugin.ValidInterpreters, ", ")+
		" (default: the first one that actually runs on this host)")
	dir := flag.String("dir", "", "destination directory (default: ./<name>)")
	desc := flag.String("description", "", "one-line description shown in the UI")
	timeout := flag.Int("timeout", 0, "per-run timeout in seconds (0 = type default: command 60, report 120, hook 30)")
	force := flag.Bool("force", false, "write into a non-empty directory")
	list := flag.Bool("list", false, "print the supported types and interpreters, then exit")
	flag.Parse()

	if *list {
		fmt.Println("types:        " + strings.Join(plugin.ValidPluginTypes, ", "))
		fmt.Println("interpreters: " + strings.Join(plugin.ValidInterpreters, ", "))
		fmt.Println()
		fmt.Println("wasm runs sandboxed in-process (no child process, no WASI); every other")
		fmt.Println("interpreter runs as a real child process with the server account's reach.")
		return
	}

	destination := strings.TrimSpace(*dir)
	if destination == "" && strings.TrimSpace(*name) != "" {
		destination = "./" + strings.TrimSpace(*name)
	}
	res, err := plugin.ScaffoldPlugin(plugin.ScaffoldOptions{
		Name:        *name,
		Type:        *typ,
		Interpreter: *interpreter,
		Dir:         destination,
		Description: *desc,
		Timeout:     *timeout,
		Force:       *force,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "plugin-init: "+err.Error())
		os.Exit(1)
	}

	fmt.Printf("wrote %d files in %s:\n", len(res.Files), res.Dir)
	for _, f := range res.Files {
		fmt.Println("  " + f)
	}
	if len(res.Notes) > 0 {
		fmt.Println()
		for _, note := range res.Notes {
			fmt.Println("note: " + note)
		}
	}
	if res.BuildHint != "" {
		fmt.Println()
		fmt.Println("build the module before checking it: " + res.BuildHint)
	}
	fmt.Println()
	fmt.Println("next steps:")
	for _, step := range res.NextSteps {
		fmt.Println("  " + step)
	}
	fmt.Println()
	fmt.Println("contract and diagnostic codes: docs/PLUGIN_AUTHORING.md")
}
