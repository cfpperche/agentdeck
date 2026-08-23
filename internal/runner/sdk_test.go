package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cfpperche/agentdeck/internal/agent"
	"github.com/cfpperche/agentdeck/internal/store"
)

// ADR-0005 integration: the claude adapter drives the Node shim, which
// (in fake mode) exercises memory across turns AND the permission
// round-trip — proving the whole chain through the Go runner without
// tokens: Go → shim → FakeSDK → control_request → Go Control() → allow.

func newSDKRunner(t *testing.T) (*Runner, *store.Store) {
	t.Helper()
	shimDir, err := filepath.Abs("../../agent-sdk-shim")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(shimDir, "node_modules", "@anthropic-ai", "claude-agent-sdk")); err == nil {
		t.Setenv("AGENTDECK_SDK_SHIM", filepath.Join(shimDir, "shim.mjs"))
	} else {
		t.Skip("agent-sdk-shim/node_modules not installed (run npm install in agent-sdk-shim)")
	}
	// force fake mode: run the shim itself as the "claude binary" won't
	// happen — instead we point the SDK detection at our shim and tell it
	// to use the FakeSDK.
	t.Setenv("AGENTDECK_SDK_FAKE", "1")
	t.Setenv("FAKE_ASK", "1")

	dataDir := t.TempDir()
	st, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	reg := agent.NewRegistry(agent.EnvWhich(nil))
	if a, ok := reg.Get("claude"); !ok || a.BuildLive == nil {
		t.Fatal("claude adapter missing or without live tier")
	}
	return New(reg, st, filepath.Join(dataDir, "ws")), st
}

func TestSDKShimMemoryAndPermission(t *testing.T) {
	os.Remove("/tmp/agentdeck-sdk-fake.txt")
	r, st := newSDKRunner(t)

	ss, _ := st.CreateSession("claude", "")

	// turn 1: plant fact (same shim process holds memory)
	if _, err := r.Send(ss.ID, "Remember: go sdk chain works"); err != nil {
		t.Fatal(err)
	}
	waitAssistant(t, st, ss.ID, 1)

	// turn 2: recall — proves the shim kept the conversation in-process
	if _, err := r.Send(ss.ID, "What do you remember?"); err != nil {
		t.Fatal(err)
	}
	msgs := waitAssistant(t, st, ss.ID, 2)
	if msgs[len(msgs)-1].Content != "go sdk chain works" {
		t.Fatalf("turn2 = %q", msgs[len(msgs)-1].Content)
	}

	// turn 3: permission round-trip through the whole stack
	ch, unsub := r.Subscribe(ss.ID)
	permDone := make(chan struct{})
	var reqID string
	go func() {
		for ev := range ch {
			if ev.Type == "permission" {
				reqID = ev.RequestID
				close(permDone)
				return
			}
		}
	}()
	if _, err := r.Send(ss.ID, "Write file now"); err != nil {
		t.Fatal(err)
	}
	<-permDone
	unsub()

	if err := r.Control(ss.ID, reqID, "allow", nil); err != nil {
		t.Fatalf("Control: %v", err)
	}
	msgs = waitAssistant(t, st, ss.ID, 3)
	if msgs[len(msgs)-1].Content != "File written" {
		t.Fatalf("turn3 = %q", msgs[len(msgs)-1].Content)
	}
	if _, err := os.Stat("/tmp/agentdeck-sdk-fake.txt"); err != nil {
		t.Fatal("file not created after allow")
	}

	// native ref captured (fake-sdk-42) for restart-with-ref
	got, _ := st.GetSession(ss.ID)
	if got.AgentRef == "" {
		t.Error("agent_ref not captured from the shim")
	}
}

func TestPermissionQueueAndEditedInput(t *testing.T) {
	os.Remove("/tmp/agentdeck-edited-input.txt")
	t.Setenv("FAKE_PARALLEL", "1")
	r, st := newSDKRunner(t)

	ss, _ := st.CreateSession("claude", "")
	ch, unsub := r.Subscribe(ss.ID)
	defer unsub()

	if _, err := r.Send(ss.ID, "Two permissions test"); err != nil {
		t.Fatal(err)
	}

	// collect BOTH permission events (queue order preserved);
	// keep the subscription alive — Control() must wake the turn
	var got []StreamEvent
	timeout := time.After(8 * time.Second)
collect:
	for {
		select {
		case ev := <-ch:
			if ev.Type == "permission" {
				got = append(got, ev)
				if len(got) == 2 {
					break collect
				}
			}
		case <-timeout:
			t.Fatalf("expected 2 permission events, got %d", len(got))
		}
	}
	if got[0].Tool != "Bash" || got[1].Tool != "Write" {
		t.Fatalf("queue order wrong: %+v", got)
	}
	// late-subscriber snapshot replays the whole queue
	if snap := r.PendingPermissions(ss.ID); len(snap) != 2 {
		t.Fatalf("snapshot queue = %d, want 2", len(snap))
	}

	// answer the FIRST with an EDITED input (allow with edits)
	edited := json.RawMessage(`{"command":"edited-cmd-42"}`)
	if err := r.Control(ss.ID, got[0].RequestID, "allow", edited); err != nil {
		t.Fatal(err)
	}
	// answer the second: deny
	if err := r.Control(ss.ID, got[1].RequestID, "deny", nil); err != nil {
		t.Fatal(err)
	}

	msgs := waitAssistant(t, st, ss.ID, 1)
	content := msgs[len(msgs)-1].Content
	if content != "executed: edited-cmd-42 (+second denied)" {
		t.Fatalf("turn content = %q", content)
	}
	// the edited input actually reached the fake (proof of updatedInput flow)
	b, err := os.ReadFile("/tmp/agentdeck-edited-input.txt")
	if err != nil || string(b) != "edited-cmd-42" {
		t.Fatalf("edited input not honored: %v %q", err, string(b))
	}
	// queue drained
	if q := r.PendingPermissions(ss.ID); len(q) != 0 {
		t.Fatalf("queue not drained: %d", len(q))
	}
}
