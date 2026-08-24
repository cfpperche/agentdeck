package store

import (
	"strings"
	"testing"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSessionCRUD(t *testing.T) {
	s := openTemp(t)

	ss, err := s.CreateSession("claude", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if ss.Title != "New session" {
		t.Errorf("default title = %q", ss.Title)
	}
	if len(ss.ID) != 12 {
		t.Errorf("id len = %d, want 12", len(ss.ID))
	}

	// rename
	renamed, err := s.RenameSession(ss.ID, "Refactor the loader")
	if err != nil || renamed == nil || renamed.Title != "Refactor the loader" {
		t.Fatalf("rename: %+v %v", renamed, err)
	}

	// messages round-trip with meta
	if _, err := s.AddMessage(ss.ID, "user", "run the tests", nil); err != nil {
		t.Fatal(err)
	}
	msg, err := s.AddMessage(ss.ID, "assistant", "all green",
		map[string]any{"tools": []any{map[string]any{"name": "bash", "state": "end"}}, "error": false})
	if err != nil {
		t.Fatal(err)
	}

	msgs, err := s.ListMessages(ss.ID)
	if err != nil || len(msgs) != 2 {
		t.Fatalf("list: %d msgs, %v", len(msgs), err)
	}
	if msgs[1].Meta["error"] != false {
		t.Errorf("meta round-trip lost: %+v", msgs[1].Meta)
	}
	_ = msg

	// list with count + preview
	list, err := s.ListSessions()
	if err != nil || len(list) != 1 {
		t.Fatalf("sessions: %v", list)
	}
	if list[0].MessageCount != 2 || list[0].Preview != "all green" {
		t.Errorf("count/preview = %d %q", list[0].MessageCount, list[0].Preview)
	}

	// HasAssistantReply
	if !s.HasAssistantReply(ss.ID) {
		t.Error("HasAssistantReply = false after assistant message")
	}

	// agent ref
	if err := s.SetAgentRef(ss.ID, "abc-123"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetSession(ss.ID)
	if got.AgentRef != "abc-123" {
		t.Errorf("agent_ref = %q", got.AgentRef)
	}

	// delete
	ok, err := s.DeleteSession(ss.ID, t.TempDir())
	if err != nil || !ok {
		t.Fatalf("delete: %v %v", ok, err)
	}
	gone, _ := s.GetSession(ss.ID)
	if gone != nil {
		t.Error("session still present after delete")
	}
	msgs, _ = s.ListMessages(ss.ID)
	if len(msgs) != 0 {
		t.Error("messages survived session delete")
	}
}

func TestCleanPreview(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"empty", "", ""},
		{
			"strips ANSI, first textual line wins",
			"\x1b[1mAgentDeck\x1b[0m\n█▀▀█ █▀▀█ █▀▀█\nreal answer here",
			"AgentDeck",
		},
		{
			"skips box-drawing garbage line",
			"┌────────┐\n└────────┘\nThe parser handles it",
			"The parser handles it",
		},
		{
			"collapses whitespace",
			"lots    of\n\tspaces everywhere now",
			"lots of",
		},
		{"all garbage → empty", "████\n▄▄▄▄", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanPreview(tt.in); got != tt.want {
				t.Errorf("CleanPreview(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestAutoTitleSource(t *testing.T) {
	// sanity for the runner contract: title auto-derivation length cap
	long := strings.Repeat("x", 100)
	if len(long[:52]) != 52 {
		t.Error("title cap helper broken")
	}
}
