package tmux

import "testing"

func TestSessionName(t *testing.T) {
	got := SessionName("AbC.def:xyz")
	if got != "agentdeck-abc-def-xyz" {
		t.Fatalf("got %s", got)
	}
	if !Owned(got) {
		t.Fatal("owned")
	}
	if Owned("random") || Owned("") {
		t.Fatal("stranger")
	}
}

func TestHasSessionMissing(t *testing.T) {
	m := New()
	if !m.Available() {
		t.Skip("tmux not installed")
	}
	ok, err := m.HasSession(nil, "agentdeck-does-not-exist-xyz")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("ghost session")
	}
}
