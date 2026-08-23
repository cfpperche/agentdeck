// Package agent defines the adapter interface that every agent CLI
// implements (hexagonal boundary), plus the registry that discovers
// installed agents. The server never talks to subprocesses directly —
// only through an Adapter.
package agent

import (
	"os"
	"os/exec"
	"strings"
)

// Event kinds emitted by parsers, normalized across agents.
const (
	KindRef   = "ref"   // native session reference captured
	KindText  = "text"  // assistant text chunk
	KindTool  = "tool"  // tool call lifecycle
	KindFinal = "final" // authoritative final text
	KindError = "error"
)

// Event is the normalized stream unit. Kind discriminates the payload.
type Event struct {
	Kind    string `json:"type"`
	Ref     string `json:"ref,omitempty"`
	Content string `json:"content,omitempty"`
	Name    string `json:"name,omitempty"`  // tool name
	State   string `json:"state,omitempty"` // start | end
	Detail  string `json:"detail,omitempty"`
}

// Adapter couples a CLI agent binary with its wire format.
type Adapter struct {
	ID    string
	Label string
	Color string
	// Build returns the argv to run the agent headless.
	//   text       user message
	//   ref        native session reference ("" on first turn)
	//   cwd        per-session working directory
	//   hasHistory true when the agent already answered once (fallback
	//              chains like grok --continue rely on this)
	Build func(text, ref, cwd string, hasHistory bool) []string
	// Parse converts one stdout JSONL line into zero or more events.
	Parse func(line string) []Event
}

// Registry holds discovered agents, preserving stable order.
type Registry struct {
	order []string
	m     map[string]Adapter
}

// Which resolves a binary name to a path. Tests inject fakes through it.
type Which func(name string) (string, error)

// EnvWhich checks AGENTDECK_BIN_<id> overrides first, then PATH.
// The override enables deterministic tests with fake agent binaries.
func EnvWhich(which Which) Which {
	return func(name string) (string, error) {
		key := "AGENTDECK_BIN_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
		if p := os.Getenv(key); p != "" {
			return p, nil
		}
		if which == nil {
			return exec.LookPath(name)
		}
		return which(name)
	}
}

// NewRegistry discovers agents present on this machine.
func NewRegistry(which Which) *Registry {
	if which == nil {
		which = exec.LookPath
	}
	r := &Registry{m: map[string]Adapter{}}
	add := func(a Adapter, bin string) {
		if bin == "" {
			return
		}
		r.order = append(r.order, a.ID)
		r.m[a.ID] = a
	}
	for _, spec := range adapterSpecs {
		if p, err := which(spec.bin); err == nil {
			add(spec.build(p), p)
		}
	}
	return r
}

// Get returns the adapter by id.
func (r *Registry) Get(id string) (Adapter, bool) {
	a, ok := r.m[id]
	return a, ok
}

// List returns adapters in registry order, for the UI.
func (r *Registry) List() []Adapter {
	out := make([]Adapter, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.m[id])
	}
	return out
}
