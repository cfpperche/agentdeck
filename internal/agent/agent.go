// Package agent defines the adapter interface that every agent CLI
// implements (hexagonal boundary), plus the registry that discovers
// installed agents. The server never talks to subprocesses directly —
// only through an Adapter.
package agent

import (
	"log"
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
	KindControl = "control"      // agent asks the user (permission request)
	KindCaps    = "capabilities" // composer surface reported at startup
)

// Event is the normalized stream unit. Kind discriminates the payload.
type Event struct {
	Kind    string        `json:"type"`
	Ref     string        `json:"ref,omitempty"`
	Content string        `json:"content,omitempty"`
	Name    string        `json:"name,omitempty"`  // tool name
	State   string        `json:"state,omitempty"` // start | end
	Detail  string        `json:"detail,omitempty"`
	Caps    *Capabilities `json:"capabilities,omitempty"`
}

// Capabilities is the composer surface an agent reports (ADR-0006):
// which models, reasoning variants and named modes it supports. The
// shim/fake emit it once at startup; parsers pass it through.
type Capabilities struct {
	Models []ModelDef `json:"models"`
	Modes  []ModeDef  `json:"modes"`
}

// ModelDef mirrors paseo's AgentModelDefinition: a selectable model
// with optional nested thinking/reasoning variants.
type ModelDef struct {
	ID                      string         `json:"id"`
	Label                   string         `json:"label"`
	IsDefault               bool           `json:"is_default,omitempty"`
	ThinkingOptions         []SelectOption `json:"thinking_options,omitempty"`
	DefaultThinkingOptionID string         `json:"default_thinking_option_id,omitempty"`
}

// ModeDef is a named permission mode (human label, not jargon).
type ModeDef struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// Controls is the composer selection sent with a message (ADR-0006):
// which model/thinking variant/mode the turn should run with. Empty
// fields mean "keep the previous value" — it is per-session state.
type Controls struct {
	Model    string `json:"model,omitempty"`
	Thinking string `json:"thinking,omitempty"`
	Mode     string `json:"mode,omitempty"`
}

// SelectOption is one variant inside ModelDef.ThinkingOptions.
type SelectOption struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	IsDefault bool   `json:"is_default,omitempty"`
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
	// ApplyControls, when non-nil, injects composer controls (ADR-0006)
	// into a fallback-tier argv. Convention: the positional prompt is
	// argv's last element unless the implementation says otherwise.
	ApplyControls func(argv []string, c *Controls) []string
	// ParseLive overrides Parse for the live tier (ADR-0007): protocol
	// bridges emit the claude wire dialect regardless of the agent's
	// own CLI format, so their streams must be read with parseClaude.
	ParseLive func(line string) []Event
	// BuildTUI, when non-nil, is the interactive TUI argv (ADR-0008).
	// Exclusive with the protocol process — never run both.
	BuildTUI func() []string
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
			ad := spec.build(p)
			if os.Getenv("AGENTDECK_DEBUG_REGISTRY") != "" {
				log.Printf("[registry] %s: BuildLive=%v ParseLive=%v", ad.ID, ad.BuildLive != nil, ad.ParseLive != nil)
			}
			add(ad, p)
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

// DefaultCaps returns the composer surface an agent offers even before
// its process first spawns (the UI must show controls immediately).
// Dynamic "capabilities" events override these once they arrive.
func DefaultCaps(agentID string) *Capabilities {
	switch agentID {
	case "claude":
		think := []SelectOption{{ID: "off", Label: "Standard"}, {ID: "on", Label: "Extended thinking"}}
		return &Capabilities{
			Models: []ModelDef{
				{ID: "sonnet", Label: "Sonnet", IsDefault: true, ThinkingOptions: think},
				{ID: "opus", Label: "Opus", ThinkingOptions: think},
				{ID: "haiku", Label: "Haiku"},
			},
			Modes: []ModeDef{
				{ID: "manual", Label: "Ask before edits", Description: "Every tool call asks (default)"},
				{ID: "acceptEdits", Label: "Auto-accept edits", Description: "File edits apply without asking"},
				{ID: "plan", Label: "Plan only", Description: "Reads and research, no changes"},
				{ID: "bypassPermissions", Label: "Full access", Description: "No permission prompts at all"},
			},
		}
	case "codex":
		think := []SelectOption{
			{ID: "low", Label: "Low", IsDefault: true}, {ID: "medium", Label: "Medium"},
			{ID: "high", Label: "High"}, {ID: "xhigh", Label: "Extra high"},
		}
		return &Capabilities{
			Models: []ModelDef{
				{ID: "gpt-5.6-terra", Label: "GPT-5.6 Terra", IsDefault: true, ThinkingOptions: think},
				{ID: "gpt-5.5", Label: "GPT-5.5", ThinkingOptions: think},
				{ID: "gpt-5.4-mini", Label: "GPT-5.4 Mini", ThinkingOptions: think},
			},
			Modes: []ModeDef{
				{ID: "manual", Label: "Ask before edits", Description: "Sandbox: workspace-write (default)"},
				{ID: "plan", Label: "Plan only", Description: "Sandbox: read-only"},
				{ID: "bypassPermissions", Label: "Full access", Description: "Sandbox: danger-full-access"},
			},
		}
	case "grok":
		think := []SelectOption{
			{ID: "low", Label: "Low"}, {ID: "medium", Label: "Medium"},
			{ID: "high", Label: "High", IsDefault: true}, {ID: "xhigh", Label: "Extra high"},
		}
		return &Capabilities{
			Models: []ModelDef{
				{ID: "grok-4.6", Label: "Grok 4.6", IsDefault: true, ThinkingOptions: think},
				{ID: "grok-4.5", Label: "Grok 4.5", ThinkingOptions: think},
			},
			Modes: []ModeDef{}, // permission prompts arrive via ACP request_permission
		}
	case "pi":
		return &Capabilities{
			Models: []ModelDef{
				{ID: "", Label: "Default model", IsDefault: true,
					ThinkingOptions: []SelectOption{
						{ID: "off", Label: "Off"}, {ID: "minimal", Label: "Minimal"},
						{ID: "low", Label: "Low"}, {ID: "medium", Label: "Medium"},
						{ID: "high", Label: "High"}, {ID: "xhigh", Label: "Extra high"},
					}},
			},
		}
	case "opencode":
		return &Capabilities{
			Models: []ModelDef{
				{ID: "", Label: "Default model", IsDefault: true,
					ThinkingOptions: []SelectOption{
						{ID: "minimal", Label: "Minimal"}, {ID: "high", Label: "High"}, {ID: "max", Label: "Max"},
					}},
			},
		}
	default:
		return nil // unknown surface until the agent reports one
	}
}
