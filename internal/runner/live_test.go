package runner

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cfpperche/agentdeck/internal/agent"
	"github.com/cfpperche/agentdeck/internal/store"
)

// Live-session tests (ADR-0004, tier 1) against the bidirectional fake:
// memory inside ONE living process, permission round-trip via stdin,
// crash → restart.

func newLiveRunner(t *testing.T, fakeEnv map[string]string) (*Runner, *store.Store) {
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
	fake, err := filepath.Abs("../../tests/fakes/fake-claude")
	if err != nil {
		t.Fatal(err)
	}
	os.Chmod(fake, 0o755)
	t.Setenv("AGENTDECK_BIN_CLAUDE", fake)
	reg := agent.NewRegistry(agent.EnvWhich(nil))
	return New(reg, st, filepath.Join(dataDir, "ws")), st
}

// waitAssistant polls until the session has n assistant messages.
func waitAssistant(t *testing.T, st *store.Store, sid string, n int) []store.Message {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		msgs, _ := st.ListMessages(sid)
		assistants := 0
		for _, m := range msgs {
			if m.Role == "assistant" {
				assistants++
			}
		}
		if assistants >= n {
			return msgs
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout: %d assistant messages (want %d)", assistants, n)
		}
		time.Sleep(30 * time.Millisecond)
	}
}

func TestLiveMemoryInOneProcess(t *testing.T) {
	r, st := newLiveRunner(t, nil)

	ss, _ := st.CreateSession("claude", "")
	if _, err := r.Send(ss.ID, "Remember: secret is 777"); err != nil {
		t.Fatal(err)
	}
	waitAssistant(t, st, ss.ID, 1)

	pid1 := r.livePID(t, ss.ID)

	// second turn into the SAME living process — no resume dance
	if _, err := r.Send(ss.ID, "What do you remember?"); err != nil {
		t.Fatal(err)
	}
	msgs := waitAssistant(t, st, ss.ID, 2)

	last := msgs[len(msgs)-1]
	if last.Content != "secret is 777" {
		t.Errorf("turn 2 content = %q, want the in-process memory", last.Content)
	}
	pid2 := r.livePID(t, ss.ID)
	if pid1 != pid2 {
		t.Errorf("process restarted between turns (pid %d → %d)", pid1, pid2)
	}

	// native ref captured for restart-with-ref
	got, _ := st.GetSession(ss.ID)
	if got.AgentRef == "" {
		t.Error("agent_ref not captured")
	}
}

func TestLivePermissionRoundTrip(t *testing.T) {
	os.Remove("/tmp/agentdeck-live.txt")
	r, st := newLiveRunner(t, map[string]string{"FAKE_ASK": "1"})

	ss, _ := st.CreateSession("claude", "")

	var mu sync.Mutex
	var permEvt *StreamEvent
	ch, unsub := r.Subscribe(ss.ID)
	defer unsub()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ev := range ch {
			if ev.Type == "permission" {
				mu.Lock()
				e := ev
				permEvt = &e
				mu.Unlock()
				return
			}
		}
	}()

	if _, err := r.Send(ss.ID, "Write file now"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("permission event never arrived")
	}

	mu.Lock()
	ev := *permEvt
	mu.Unlock()
	if ev.RequestID == "" || ev.Tool != "Bash" {
		t.Fatalf("permission event = %+v", ev)
	}

	// deny first → agent reports denial, no file
	if err := r.Control(ss.ID, ev.RequestID, "deny"); err != nil {
		t.Fatal(err)
	}
	msgs := waitAssistant(t, st, ss.ID, 1)
	if msgs[len(msgs)-1].Content != "Permission denied by user" {
		t.Errorf("after deny = %q", msgs[len(msgs)-1].Content)
	}
	if _, err := os.Stat("/tmp/agentdeck-live.txt"); !os.IsNotExist(err) {
		t.Error("file created despite deny")
	}

	// ask again and allow → file exists
	if _, err := r.Send(ss.ID, "Write file now"); err != nil {
		t.Fatal(err)
	}
	for {
		var ok bool
		select {
		case ev := <-ch:
			if ev.Type == "permission" {
				if err := r.Control(ss.ID, ev.RequestID, "allow"); err != nil {
					t.Fatal(err)
				}
				ok = true
			}
		case <-time.After(5 * time.Second):
			t.Fatal("second permission never arrived")
		}
		if ok {
			break
		}
	}
	waitAssistant(t, st, ss.ID, 2)
	if _, err := os.Stat("/tmp/agentdeck-live.txt"); err != nil {
		t.Error("file NOT created after allow")
	}
}

func TestLiveCrashRestarts(t *testing.T) {
	r, st := newLiveRunner(t, map[string]string{"FAKE_CRASH_AFTER": "1"})

	ss, _ := st.CreateSession("claude", "")
	if _, err := r.Send(ss.ID, "hello one"); err != nil {
		t.Fatal(err)
	}
	waitAssistant(t, st, ss.ID, 1)
	pid1 := r.livePID(t, ss.ID)

	// fake dies right after turn 1 (FAKE_CRASH_AFTER=1); wait for exit
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !r.liveAlive(t, ss.ID) {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}

	// next Send must transparently restart the process
	if _, err := r.Send(ss.ID, "hello two"); err != nil {
		t.Fatalf("send after crash: %v", err)
	}
	msgs := waitAssistant(t, st, ss.ID, 2)
	pid2 := r.livePID(t, ss.ID)
	if pid1 == pid2 {
		t.Errorf("expected a NEW process after crash (pid %d)", pid1)
	}
	if msgs[len(msgs)-1].Content != "echo: hello two" {
		t.Errorf("restart turn content = %q", msgs[len(msgs)-1].Content)
	}
}

func TestControlNoLiveProcess(t *testing.T) {
	r, st := newLiveRunner(t, nil)
	ss, _ := st.CreateSession("claude", "")
	if err := r.Control(ss.ID, "x", "allow"); err == nil {
		t.Error("expected error controlling a session with no live process")
	}
}

// ---- helpers ----

func (r *Runner) livePID(t *testing.T, sid string) int {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	lp := r.live[sid]
	if lp == nil || lp.cmd == nil || lp.cmd.Process == nil {
		t.Fatalf("no live process for %s", sid)
	}
	return lp.cmd.Process.Pid
}

func (r *Runner) liveAlive(t *testing.T, sid string) bool {
	t.Helper()
	r.mu.Lock()
	lp := r.live[sid]
	r.mu.Unlock()
	return lp.alive()
}

// collectStatuses reads events until the predicate or timeout, recording
// every state/permission event seen.
func collectUntil(t *testing.T, ch <-chan StreamEvent, timeout time.Duration, done func([]string) bool) []string {
	t.Helper()
	var seq []string
	deadline := time.After(timeout)
	for {
		select {
		case ev := <-ch:
			switch ev.Type {
			case "state":
				seq = append(seq, "state:"+ev.Status)
			case "permission":
				seq = append(seq, "permission")
			case "message_end":
				seq = append(seq, "turn_end")
			}
			if done(seq) {
				return seq
			}
		case <-deadline:
			return seq
		}
	}
}

func TestWaitingStateTransitions(t *testing.T) {
	os.Remove("/tmp/agentdeck-live.txt")
	r, st := newLiveRunner(t, map[string]string{"FAKE_ASK": "1"})

	ss, _ := st.CreateSession("claude", "")
	ch, unsub := r.Subscribe(ss.ID)
	defer unsub()

	if _, err := r.Send(ss.ID, "Write file now"); err != nil {
		t.Fatal(err)
	}

	// running → permission → waiting (done only when BOTH arrived)
	seq := collectUntil(t, ch, 5*time.Second, func(s []string) bool {
		return contains(s, "state:waiting") && contains(s, "permission")
	})
	if !contains(seq, "state:running") || !contains(seq, "permission") || !contains(seq, "state:waiting") {
		t.Fatalf("sequence = %v", seq)
	}
	if r.Status(ss.ID) != StatusWaiting {
		t.Fatalf("Status = %s, want waiting", r.Status(ss.ID))
	}

	// late subscriber sees the pending permission replay (G1)
	if p := r.PendingPermission(ss.ID); p == nil || p.RequestID == "" {
		t.Fatal("PendingPermission missing during waiting")
	}

	// answer → running again → turn completes → idle
	if p := r.PendingPermission(ss.ID); p != nil {
		if err := r.Control(ss.ID, p.RequestID, "allow"); err != nil {
			t.Fatal(err)
		}
	}
	seq = collectUntil(t, ch, 5*time.Second, func(s []string) bool {
		return contains(s, "state:idle")
	})
	for _, want := range []string{"state:running", "turn_end", "state:idle"} {
		if !contains(seq, want) {
			t.Fatalf("after allow: %v (want %s)", seq, want)
		}
	}
}

func TestQueuedSteeringDelivered(t *testing.T) {
	r, st := newLiveRunner(t, nil)

	ss, _ := st.CreateSession("claude", "")
	ch, unsub := r.Subscribe(ss.ID)
	defer unsub()

	if _, err := r.Send(ss.ID, "Remember: steering works"); err != nil {
		t.Fatal(err)
	}
	// second message while turn 1 is in flight → queued, not rejected
	queued, err := r.Send(ss.ID, "What do you remember?")
	if err != nil || !queued {
		t.Fatalf("second send: queued=%v err=%v", queued, err)
	}
	if r.QueueLen(ss.ID) != 1 {
		t.Fatalf("QueueLen = %d", r.QueueLen(ss.ID))
	}

	// both turns complete; the second recalls the first (same process)
	deadline := time.Now().Add(15 * time.Second)
	for {
		msgs, _ := st.ListMessages(ss.ID)
		assistants := 0
		for _, m := range msgs {
			if m.Role == "assistant" {
				assistants++
			}
		}
		if assistants >= 2 {
			if msgs[len(msgs)-1].Content != "steering works" {
				t.Fatalf("second turn = %q", msgs[len(msgs)-1].Content)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("queued turn never delivered")
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = ch
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
