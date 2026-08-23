package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cfpperche/agentdeck/internal/agent"
	"github.com/cfpperche/agentdeck/internal/runner"
	"github.com/cfpperche/agentdeck/internal/store"
)

// End-to-end API test with the fake claude agent: exercises the exact
// HTTP contract the React app consumes (parity with the Python server).

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	fake, err := filepath.Abs("../../tests/fakes/fake-claude")
	if err != nil {
		t.Fatal(err)
	}
	os.Chmod(fake, 0o755)
	t.Setenv("AGENTDECK_BIN_CLAUDE", fake)

	dataDir := t.TempDir()
	st, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	reg := agent.NewRegistry(agent.EnvWhich(nil))
	r := runner.New(reg, st, filepath.Join(dataDir, "ws"))
	srv := &Server{Registry: reg, Store: st, Runner: r}
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)
	return ts
}

func TestAPIContract(t *testing.T) {
	ts := newTestServer(t)

	// agents
	resp, _ := http.Get(ts.URL + "/api/agents")
	if resp.StatusCode != 200 {
		t.Fatalf("agents status = %d", resp.StatusCode)
	}
	var agents []map[string]string
	json.NewDecoder(resp.Body).Decode(&agents)
	if len(agents) == 0 || agents[0]["label"] == "" {
		t.Fatalf("agents = %v", agents)
	}

	// create session
	resp, _ = http.Post(ts.URL+"/api/sessions", "application/json",
		strings.NewReader(`{"agent":"claude"}`))
	if resp.StatusCode != 200 {
		t.Fatalf("create status = %d", resp.StatusCode)
	}
	var ss store.Session
	json.NewDecoder(resp.Body).Decode(&ss)
	if ss.ID == "" || ss.Agent != "claude" {
		t.Fatalf("session = %+v", ss)
	}

	// unknown agent → 400
	resp, _ = http.Post(ts.URL+"/api/sessions", "application/json",
		strings.NewReader(`{"agent":"nope"}`))
	if resp.StatusCode != 400 {
		t.Errorf("unknown agent status = %d, want 400", resp.StatusCode)
	}

	// send message (fake agent answers)
	resp, _ = http.Post(ts.URL+"/api/sessions/"+ss.ID+"/messages",
		"application/json", strings.NewReader(`{"text":"run the thing"}`))
	if resp.StatusCode != 200 {
		t.Fatalf("send status = %d", resp.StatusCode)
	}

	// wait for the agent to finish
	deadline := time.Now().Add(10 * time.Second)
	for {
		msgs := listMessages(t, ts, ss.ID)
		if len(msgs) >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("assistant message never persisted")
		}
		time.Sleep(50 * time.Millisecond)
	}

	msgs := listMessages(t, ts, ss.ID)
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" ||
		msgs[1].Content != "Ran echo hello" {
		t.Fatalf("messages = %+v %+v", msgs[0], msgs[1])
	}

	// auto-title applied
	sessions := listSessions(t, ts)
	if sessions[0].Title != "run the thing" {
		t.Errorf("title = %q", sessions[0].Title)
	}
	if sessions[0].Preview != "Ran echo hello" {
		t.Errorf("preview = %q", sessions[0].Preview)
	}

	// rename via PATCH
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/sessions/"+ss.ID,
		strings.NewReader(`{"title":"renamed"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("rename: %v %d", err, resp.StatusCode)
	}
	var renamed store.Session
	json.NewDecoder(resp.Body).Decode(&renamed)
	if renamed.Title != "renamed" {
		t.Errorf("renamed = %+v", renamed)
	}

	// 404s
	resp, _ = http.Get(ts.URL + "/api/sessions/zzz/messages")
	if resp.StatusCode != 404 {
		t.Errorf("messages 404 = %d", resp.StatusCode)
	}

	// delete
	req, _ = http.NewRequest("DELETE", ts.URL+"/api/sessions/"+ss.ID, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("delete: %v", resp.StatusCode)
	}
	sessions = listSessions(t, ts)
	if len(sessions) != 0 {
		t.Errorf("sessions after delete = %d", len(sessions))
	}
}

func TestSSEStream(t *testing.T) {
	ts := newTestServer(t)

	var ss store.Session
	resp, _ := http.Post(ts.URL+"/api/sessions", "application/json",
		strings.NewReader(`{"agent":"claude"}`))
	json.NewDecoder(resp.Body).Decode(&ss)

	// subscribe BEFORE sending so we catch every event
	req, _ := http.NewRequest("GET", ts.URL+"/api/sessions/"+ss.ID+"/events", nil)
	sseResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer sseResp.Body.Close()
	if sseResp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("sse content-type = %q", sseResp.Header.Get("Content-Type"))
	}

	http.Post(ts.URL+"/api/sessions/"+ss.ID+"/messages",
		"application/json", strings.NewReader(`{"text":"hi"}`))

	// read SSE lines until we see message_end + state false
	br := bufio.NewReader(sseResp.Body)
	seen := map[string]bool{}
	deadline := time.After(10 * time.Second)
	for !(seen["message_end"] && seen["state_off"]) {
		select {
		case <-deadline:
			t.Fatalf("timeout; seen=%v", seen)
		default:
		}
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev map[string]any
		if json.Unmarshal([]byte(line[6:]), &ev) != nil {
			continue
		}
		switch ev["type"] {
		case "message_end":
			seen["message_end"] = true
		case "state":
			if run, _ := ev["running"].(bool); !run {
				seen["state_off"] = true
			}
		}
	}
}

// ---- small helpers ----

func listMessages(t *testing.T, ts *httptest.Server, sid string) []store.Message {
	t.Helper()
	resp, err := http.Get(ts.URL + "/api/sessions/" + sid + "/messages")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("list messages: %v %d", err, resp.StatusCode)
	}
	var msgs []store.Message
	json.NewDecoder(resp.Body).Decode(&msgs)
	return msgs
}

func listSessions(t *testing.T, ts *httptest.Server) []store.Session {
	t.Helper()
	resp, err := http.Get(ts.URL + "/api/sessions")
	if err != nil {
		t.Fatal(err)
	}
	var ss []store.Session
	json.NewDecoder(resp.Body).Decode(&ss)
	return ss
}
