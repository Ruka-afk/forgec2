package main

// Task registry v2: single-file command registration.
//
// RegisterTask binds a handler to a protocol.TaskSpec, expanding the spec's
// aliases automatically. New commands should declare their TaskSpec in
// pkg/protocol (taskspec_data.go) and bind the handler here via
// RegisterTask — metadata, aliases, server-side validation and help output
// then all follow from that one declaration.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/forgec2/forgec2/pkg/protocol"
)

// RegisterTask binds a handler under the spec's canonical type plus every
// alias declared on it. Later registrations for the same name overwrite —
// init-time duplicates are already prevented at the spec layer.
func RegisterTask(spec protocol.TaskSpec, h taskHandler) {
	taskHandlers[spec.Type] = h
	for _, alias := range spec.Aliases {
		taskHandlers[alias] = h
	}
}

// handleHelp renders the full command catalogue from the shared spec
// registry: canonical name, aliases, parameters and description. Metadata
// only — no capability details beyond what the server's own API exposes.
func handleHelp(task Task, res *TaskResult) {
	specs := protocol.AllSpecs()
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%d commands available:\n\n", len(specs)))
	for _, sp := range specs {
		line := fmt.Sprintf("  %-28s", sp.Type)
		if len(sp.Aliases) > 0 {
			line += " (alias: " + strings.Join(sp.Aliases, ", ") + ")"
		}
		b.WriteString(line + "\n")
		if sp.Description != "" {
			b.WriteString("      " + sp.Description + "\n")
		}
		if len(sp.Parameters) > 0 {
			params := make([]string, 0, len(sp.Parameters))
			for _, p := range sp.Parameters {
				mark := ""
				if p.Required {
					mark = "*"
				}
				params = append(params, p.Name+mark+"("+p.Type+")")
			}
			b.WriteString("      params: " + strings.Join(params, ", ") + "\n")
		}
	}
	res.Output = strings.TrimRight(b.String(), "\n")
}

// sortedSpecTypes is a deterministic ordering helper used by tests.
func sortedSpecTypes() []string {
	specs := protocol.AllSpecs()
	out := make([]string, 0, len(specs))
	for _, sp := range specs {
		out = append(out, sp.Type)
	}
	sort.Strings(out)
	return out
}

// init binds the v2-style commands. Each pair below is the complete
// registration story: the spec (declared in pkg/protocol/taskspec_data.go)
// carries metadata + aliases for BOTH ends; this side only supplies the
// handler. Adding command #208 means one spec line here plus one
// RegisterTask line — nothing else.
func init() {
	RegisterTask(protocol.MustSpec(protocol.TaskTypeHostInfo), handleHostInfo)
	RegisterTask(protocol.MustSpec("mkdir"), handleMkdir)
	RegisterTask(protocol.MustSpec("rename"), handleRename)
	RegisterTask(protocol.MustSpec("chmod"), handleChmod)
	RegisterTask(protocol.MustSpec("help"), handleHelp)
}
