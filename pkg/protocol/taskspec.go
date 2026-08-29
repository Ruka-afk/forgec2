package protocol

// TaskSpec is the single point of declaration for a task type. Both the
// teamserver (metadata API, validation, approval gating) and the agent
// (dispatch, aliases, help) derive their views from this registry, so adding
// a command means writing ONE declaration instead of keeping three copies in
// sync.
//
// Registration happens at init time from taskspec_data.go; duplicates or
// alias collisions panic loudly — a broken build is better than a silently
// shadowed command.

import (
	"fmt"
	"sort"
	"sync"
)

// TaskParam describes one operator-supplied parameter.
type TaskParam struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
}

// TaskSpec declares everything the framework knows about a task type.
type TaskSpec struct {
	Type             string      `json:"type"`
	Name             string      `json:"name"`
	Description      string      `json:"description,omitempty"`
	Category         string      `json:"category,omitempty"`
	RequiresShell    bool        `json:"requires_shell,omitempty"`
	RequiresElev     bool        `json:"requires_elevation,omitempty"`
	RequiresApproval bool        `json:"requires_approval,omitempty"`
	Aliases          []string    `json:"aliases,omitempty"`
	Help             string      `json:"help,omitempty"`
	Parameters       []TaskParam `json:"parameters,omitempty"`
}

var (
	taskSpecsMu     sync.RWMutex
	taskSpecsByType = map[string]*TaskSpec{}
	taskAliasIndex  = map[string]string{} // alias -> canonical Type
)

// RegisterTaskSpec adds one declaration. Panics on duplicate Type, on an
// alias that collides with another alias, and on an alias that shadows an
// existing canonical Type — all three would silently misroute tasks.
func RegisterTaskSpec(spec TaskSpec) {
	if spec.Type == "" {
		panic("protocol: RegisterTaskSpec with empty Type")
	}
	taskSpecsMu.Lock()
	defer taskSpecsMu.Unlock()

	if prev, ok := taskSpecsByType[spec.Type]; ok {
		panic(fmt.Sprintf("protocol: task type %q registered twice (%q and %q)",
			spec.Type, prev.Name, spec.Name))
	}
	for _, alias := range spec.Aliases {
		if alias == "" {
			panic(fmt.Sprintf("protocol: task %q has empty alias", spec.Type))
		}
		if _, clash := taskSpecsByType[alias]; clash {
			panic(fmt.Sprintf("protocol: alias %q (task %q) shadows a canonical type", alias, spec.Type))
		}
		if prevOwner, clash := taskAliasIndex[alias]; clash {
			panic(fmt.Sprintf("protocol: alias %q claimed by both %q and %q",
				alias, prevOwner, spec.Type))
		}
		taskAliasIndex[alias] = spec.Type
	}
	copied := spec
	taskSpecsByType[spec.Type] = &copied
}

// AllSpecs returns every registered declaration sorted by Type so callers
// (help output, metadata APIs) get deterministic ordering.
func AllSpecs() []TaskSpec {
	taskSpecsMu.RLock()
	defer taskSpecsMu.RUnlock()
	out := make([]TaskSpec, 0, len(taskSpecsByType))
	for _, sp := range taskSpecsByType {
		out = append(out, *sp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

// SpecOf looks up the declaration for a canonical task type.
func SpecOf(taskType string) (TaskSpec, bool) {
	taskSpecsMu.RLock()
	defer taskSpecsMu.RUnlock()
	sp, ok := taskSpecsByType[taskType]
	if !ok {
		return TaskSpec{}, false
	}
	return *sp, true
}

// ResolveAlias maps an alias to its canonical task type. Unknown names are
// returned unchanged so callers can treat the result uniformly.
func ResolveAlias(name string) (string, bool) {
	taskSpecsMu.RLock()
	defer taskSpecsMu.RUnlock()
	if canon, ok := taskAliasIndex[name]; ok {
		return canon, true
	}
	return name, false
}

// AliasCount exposes the size of the alias index for consistency tests.
func AliasCount() int {
	taskSpecsMu.RLock()
	defer taskSpecsMu.RUnlock()
	return len(taskAliasIndex)
}

// MustSpec returns the registered declaration for taskType, panicking when
// absent. Meant for init-time handler binding: a typo in a command name must
// fail loudly at startup rather than resurface as runtime "unknown task".
func MustSpec(taskType string) TaskSpec {
	sp, ok := SpecOf(taskType)
	if !ok {
		panic(fmt.Sprintf("protocol: no spec registered for task %q", taskType))
	}
	return sp
}
