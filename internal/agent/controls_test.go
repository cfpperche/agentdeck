package agent

import (
	"reflect"
	"strings"
	"testing"
)

// The curated catalogs must map composer controls to REAL flags
// verified against each CLI's --help on 2026-08-24.
func TestApplyControls(t *testing.T) {
	// CI has no agent CLIs: build adapters straight from the specs with
	// a dummy bin path — we only exercise ApplyControls, not exec.
	find := func(id string) Adapter {
		for _, sp := range adapterSpecs {
			if sp.bin == id {
				return sp.build(id)
			}
		}
		t.Fatalf("agent %q not found", id)
		return Adapter{}
	}

	codex := find("codex")
	argv := codex.ApplyControls(
		[]string{"codex", "exec", "--json", "-s", "workspace-write", "PROMPT"},
		&Controls{Model: "gpt-5-codex", Thinking: "high", Mode: "plan"})
	got := strings.Join(argv, " ")
	for _, want := range []string{"-m gpt-5-codex", "model_reasoning_effort=", "-s read-only"} {
		if !strings.Contains(got, want) {
			t.Errorf("codex argv missing %q: %s", want, got)
		}
	}

	grok := find("grok")
	argv = grok.ApplyControls([]string{"grok", "--output-format", "streaming-json", "-p", "PROMPT"},
		&Controls{Model: "grok-4.6", Thinking: "high"})
	if got = strings.Join(argv, " "); !strings.Contains(got, "-m grok-4.6") ||
		!strings.Contains(got, "--reasoning-effort high") ||
		!strings.HasSuffix(got, "PROMPT") {
		t.Errorf("grok argv wrong: %s", got)
	}

	pi := find("pi")
	argv = pi.ApplyControls([]string{"pi", "-p", "--mode", "json", "PROMPT"},
		&Controls{Thinking: "high"})
	if got = strings.Join(argv, " "); !strings.Contains(got, "--thinking high") || !strings.HasSuffix(got, "PROMPT") {
		t.Errorf("pi argv wrong: %s", got)
	}

	opencode := find("opencode")
	argv = opencode.ApplyControls([]string{"opencode", "run", "--format", "json", "PROMPT"},
		&Controls{Model: "anthropic/claude-sonnet-4", Thinking: "max"})
	if got = strings.Join(argv, " "); !strings.Contains(got, "-m anthropic/claude-sonnet-4") ||
		!strings.Contains(got, "--variant max") {
		t.Errorf("opencode argv wrong: %s", got)
	}

	claude := find("claude")
	base := []string{"claude", "-p", "PROMPT"}
	argv = claude.ApplyControls(base, &Controls{Model: "opus", Mode: "acceptEdits"})
	if !reflect.DeepEqual(argv, []string{"claude", "-p", "PROMPT", "--model", "opus", "--permission-mode", "acceptEdits"}) {
		t.Errorf("claude argv wrong: %v", argv)
	}
}
