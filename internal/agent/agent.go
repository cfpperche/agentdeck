// Package agent defines the adapter interface that every agent CLI
// implements (hexagonal boundary), plus the registry that discovers
// installed agents. The server never talks to subprocesses directly —
// only through an Adapter.
package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Event kinds emitted by parsers, normalized across agents.
const (
	KindRef     = "ref"   // native session reference captured
	KindText    = "text"  // assistant text chunk
	KindTool    = "tool"  // tool call lifecycle
	KindFinal   = "final" // authoritative final text (turn end)
	KindError   = "error"
	KindControl = "control" // agent asks the user (permission request)
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
	// Build returns the argv to run the agent headless (one-shot).
	//   text       user message
	//   ref        native session reference ("" on first turn)
	//   cwd        per-session working directory
	//   hasHistory true when the agent already answered once (fallback
	//              chains like grok --continue rely on this)
	Build func(text, ref, cwd string, hasHistory bool) []string
	// Parse converts one stdout JSONL line into zero or more events.
	Parse func(line string) []Event
	// BuildLive, when non-nil, marks a tier-1 agent (ADR-0004): argv for
	// a PERSISTENT bidirectional process (stdin/stdout JSONL). ref resumes
	// the native session after a crash/restart.
	BuildLive func(ref, cwd string) []string
}

// Registry holds discovered agents, preserving stable order.
type Registry struct {
	order []string
	m     map[string]Adapter
}

// Which resolves a binary name to a path. Tests inject fakes through it.
type Which func(name string) (string, error)

// EnvWhich checks AGENTDECK_BIN_<id> overrides first, then resolves
// via PATH plus user-local dirs (userAwareLookup). The override enables
// deterministic tests with fake agent binaries.
func EnvWhich(which Which) Which {
	base := which
	if base == nil {
		base = userAwareLookup
	}
	return func(name string) (string, error) {
		key := "AGENTDECK_BIN_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
		if p := os.Getenv(key); p != "" {
			return p, nil
		}
		return base(name)
	}
}

// NewRegistry discovers agents present on this machine.
func NewRegistry(which Which) *Registry {
	if which == nil {
		which = userAwareLookup
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

// userAwareLookup resolves a binary via PATH plus the usual user-local
// install dirs (~/.local/bin, ~/.bun/bin) — systemd services get a clean
// PATH that misses CLIs installed by npm/bun user prefixes (found live:
// claude/grok in ~/.local/bin were invisible to the service).
func userAwareLookup(name string) (string, error) {
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	for _, dir := range []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".bun", "bin"),
	} {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s: not found", name)
}

// Get returns the adapter by id.
func (r *Registry) Get(id string) (Adapter, bool) {
	a, ok := r.m[id]
	return a, ok
}

// DisableLive strips the live tier from an adapter (forces one-shot
// fallback). Used by tests and available for configuration.
func (r *Registry) DisableLive(id string) {
	if a, ok := r.m[id]; ok {
		a.BuildLive = nil
		r.m[id] = a
	}
}

// List returns adapters in registry order, for the UI.
func (r *Registry) List() []Adapter {
	out := make([]Adapter, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.m[id])
	}
	return out
}
