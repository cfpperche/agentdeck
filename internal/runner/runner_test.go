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
	return newTestRunnerMode(t, fakeEnv, false) // one-shot fallback tier
}

func newTestRunnerMode(t *testing.T, fakeEnv map[string]string, live bool) (*Runner, *store.Store) {
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
	if !live {
		// strip the live tier: these tests exercise the one-shot fallback
		reg.DisableLive("claude")
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

	ss, _ := st.CreateSession("claude", "", "")
	ch, unsub := r.Subscribe(ss.ID)
	defer unsub()

	if _, err := r.Send(ss.ID, "run echo hello"); err != nil {
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

func TestQueueFullAndDrain(t *testing.T) {
	// fallback tier, one-shot held busy by FAKE_SLEEP
	r, st := newTestRunner(t, map[string]string{"FAKE_SLEEP": "2"})

	ss, _ := st.CreateSession("claude", "", "")
	if _, err := r.Send(ss.ID, "first"); err != nil {
		t.Fatal(err)
	}

	// fill the queue to the cap
	for i := 0; i < runnerQueueCap(); i++ {
		queued, err := r.Send(ss.ID, "queued-"+string(rune('a'+i)))
		if err != nil || !queued {
			t.Fatalf("enqueue %d: queued=%v err=%v", i, queued, err)
		}
	}
	// one beyond the cap → ErrBusy
	if _, err := r.Send(ss.ID, "overflow"); err != ErrBusy {
		t.Fatalf("overflow err = %v, want ErrBusy", err)
	}

	// cancel everything queued, then let the turn finish
	if n := r.ClearQueue(ss.ID); n != runnerQueueCap() {
		t.Fatalf("ClearQueue = %d", n)
	}
	waitAssistant(t, st, ss.ID, 1)
	deadline := time.Now().Add(3 * time.Second)
	for r.IsRunning(ss.ID) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if r.IsRunning(ss.ID) {
		t.Fatal("still running after sleep turn")
	}
	// queue was cancelled: no further turns fire. Queued (cancelled)
	// user messages remain in history by design — assert assistant count
	// stays at exactly 1 and nothing is delivered afterwards.
	time.Sleep(700 * time.Millisecond) // drain window would fire here
	msgs, _ := st.ListMessages(ss.ID)
	assistants := 0
	for _, m := range msgs {
		if m.Role == "assistant" {
			assistants++
		}
	}
	if assistants != 1 {
		t.Fatalf("assistants after cancel = %d, want 1", assistants)
	}
	if r.QueueLen(ss.ID) != 0 {
		t.Fatalf("queue not empty: %d", r.QueueLen(ss.ID))
	}
}

func TestStopPersistsPartial(t *testing.T) {
	r, st := newTestRunner(t, map[string]string{"FAKE_SLEEP": "30"})

	ss, _ := st.CreateSession("claude", "", "")
	ch, unsub := r.Subscribe(ss.ID)
	defer unsub()

	if _, err := r.Send(ss.ID, "long task"); err != nil {
		t.Fatal(err)
	}
	// wait for the streamed lines, then kill mid-sleep
	collect(t, ch, []string{"state", "tool", "text"})
	r.Stop(ss.ID)

	// Stop marks idle immediately; the partial result lands a beat later —
	// wait for the persisted assistant message, not for the state
	deadline := time.Now().Add(5 * time.Second)
	var msgs []store.Message
	for time.Now().Before(deadline) {
		msgs, _ = st.ListMessages(ss.ID)
		if len(msgs) >= 2 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
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
	if _, err := r.Send("nope", "hi"); err != os.ErrNotExist {
		t.Fatalf("err = %v, want ErrNotExist", err)
	}
}

func runnerQueueCap() int { return 5 }
