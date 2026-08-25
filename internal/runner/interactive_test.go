package runner

import (
	"strings"
	"testing"

	"github.com/cfpperche/agentdeck/internal/agent"
)

func TestApplyTUIControlsClaude(t *testing.T) {
	got := applyTUIControls("claude", []string{"claude"}, &agent.Controls{
		Model: "haiku", Mode: "bypassPermissions", Thinking: "off",
	})
	want := "claude --model haiku --permission-mode bypassPermissions --effort low"
	if strings.Join(got, " ") != want {
		t.Fatalf("got %q", strings.Join(got, " "))
	}
}

func TestApplyTUIControlsPi(t *testing.T) {
	got := applyTUIControls("pi", []string{"pi"}, &agent.Controls{
		Provider: "openrouter", Model: "ox-alpha", Thinking: "high", OpMode: "readonly",
	})
	s := strings.Join(got, " ")
	for _, part := range []string{"--provider openrouter", "--model ox-alpha", "--thinking high", "--tools read,grep,find,ls"} {
		if !strings.Contains(s, part) {
			t.Fatalf("missing %q in %q", part, s)
		}
	}
}
