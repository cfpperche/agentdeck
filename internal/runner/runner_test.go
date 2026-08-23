package runner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cfpperche/agentdeck/internal/agent"
	"github.com/cfpperche/agentdeck/internal/store"
)

// The fake agent binary emits the recorded Claude JSONL (see
// tests/fakes/fake-claude). The registry is wired via the same env
// override mechanism (AGENTDECK_BIN_*) used in production.

func newTestRunner(t *testing.T, fakeEnv map[string]string) (*Runner, *store.Store) {
	t.Helper()
	for k, v := range fakeEnv {
		t.Setenv(k, v)
	}
	dataDir := t.TempDir()
	st, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	fakePath, err := filepath.Abs("../../tests/fakes/fake-claude")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(fakePath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTDECK_BIN_CLAUDE", fakePath)

	reg := agent.NewRegistry(agent.EnvWhich(nil))
	if _, ok := reg.Get("claude"); !ok {
		t.Fatal("fake claude not registered")
	}
	return New(reg, st, filepath.Join(dataDir, "workspaces")), st
}

func collect(t *testing.T, ch <-chan StreamEvent, wantTypes []string) []StreamEvent {
	t.Helper()
	var got []StreamEvent
	deadline := time.After(10 * time.Second)
	for len(got) < len(wantTypes) {
		select {
		case ev := <-ch:
			got = append(got, ev)
		case <-deadline:
			t.Fatalf("timeout waiting for %v (got %d)", wantTypes, len(got))
		}
	}
	return got
}

func TestSendHappyPath(t *testing.T) {
	r, st := newTestRunner(t, nil)

	ss, _ := st.CreateSession("claude", "")
	ch, unsub := r.Subscribe(ss.ID)
	defer unsub()

	if err := r.Send(ss.ID, "run echo hello"); err != nil {
		t.Fatalf("send: %v", err)
	}

	evs := collect(t, ch, []string{"state", "tool", "text", "message_end", "state"})
	if evs[0].Type != "state" || !evs[0].Running {
		t.Errorf("first event = %+v, want state running", evs[0])
	}
	if evs[1].Type != "tool" || evs[1].Name != "Bash" {
		t.Errorf("tool event = %+v", evs[1])
	}
	if evs[2].Type != "text" || evs[2].Content != "Ran echo hello" {
		t.Errorf("text event = %+v", evs[2])
	}
	if evs[3].Type != "message_end" || evs[3].Message.Content != "Ran echo hello" {
		t.Errorf("message_end = %+v", evs[3])
	}
	if evs[4].Type != "state" || evs[4].Running {
		t.Errorf("last event = %+v, want state stopped", evs[4])
	}

	// persisted: user + assistant, meta carries tools
	msgs, _ := st.ListMessages(ss.ID)
	if len(msgs) != 2 {
		t.Fatalf("persisted %d messages", len(msgs))
	}
	meta := msgs[1].Meta
	if tools, ok := meta["tools"].([]any); !ok || len(tools) != 1 {
		t.Errorf("meta.tools = %#v", meta["tools"])
	}
	if meta["error"] != false {
		t.Errorf("meta.error = %v", meta["error"])
	}

	// native ref captured for resume
	got, _ := st.GetSession(ss.ID)
	if got.AgentRef != "fake-session-001" {
		t.Errorf("agent_ref = %q", got.AgentRef)
	}

	// auto-title from first message
	if got.Title != "run echo hello" {
		t.Errorf("auto title = %q", got.Title)
	}

	if r.IsRunning(ss.ID) {
		t.Error("still running after completion")
	}
}

func TestSendBusy(t *testing.T) {
	r, st := newTestRunner(t, map[string]string{"FAKE_SLEEP": "3"})

	ss, _ := st.CreateSession("claude", "")
	if err := r.Send(ss.ID, "first"); err != nil {
		t.Fatal(err)
	}
	// give the process a moment to actually be running
	time.Sleep(200 * time.Millisecond)

	if err := r.Send(ss.ID, "second"); err != ErrBusy {
		t.Fatalf("second send err = %v, want ErrBusy", err)
	}

	r.Stop(ss.ID)
	deadline := time.Now().Add(5 * time.Second)
	for r.IsRunning(ss.ID) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if r.IsRunning(ss.ID) {
		t.Fatal("stop did not terminate the run")
	}
}

func TestStopPersistsPartial(t *testing.T) {
	r, st := newTestRunner(t, map[string]string{"FAKE_SLEEP": "30"})

	ss, _ := st.CreateSession("claude", "")
	ch, unsub := r.Subscribe(ss.ID)
	defer unsub()

	if err := r.Send(ss.ID, "long task"); err != nil {
		t.Fatal(err)
	}
	// wait for the streamed lines, then kill mid-sleep
	collect(t, ch, []string{"state", "tool", "text"})
	r.Stop(ss.ID)

	deadline := time.Now().Add(5 * time.Second)
	for r.IsRunning(ss.ID) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	msgs, _ := st.ListMessages(ss.ID)
	if len(msgs) != 2 {
		t.Fatalf("messages after stop = %d, want 2 (user + partial assistant)", len(msgs))
	}
	// partial content was already streamed before the kill
	if msgs[1].Content != "Ran echo hello" {
		t.Errorf("partial content = %q", msgs[1].Content)
	}
}

func TestUnknownSession(t *testing.T) {
	r, _ := newTestRunner(t, nil)
	if err := r.Send("nope", "hi"); err != os.ErrNotExist {
		t.Fatalf("err = %v, want ErrNotExist", err)
	}
}
