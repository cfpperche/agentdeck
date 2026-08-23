// Package runner manages agent process lifecycles: spawn, stream-parse,
// broadcast to subscribers, persist results. One process per session.
package runner

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cfpperche/agentdeck/internal/agent"
	"github.com/cfpperche/agentdeck/internal/store"
)

// ErrBusy mirrors the legacy 409: one running process per session.
var ErrBusy = errors.New("agent already running for this session")

// StreamEvent is the client-facing event (SSE payload shape, parity with
// the Phase-0 server).
type StreamEvent struct {
	Type    string         `json:"type"` // state | text | tool | message_end
	Running bool           `json:"running"`
	Content string         `json:"content,omitempty"`
	Name    string         `json:"name,omitempty"`
	State   string         `json:"state,omitempty"`
	Detail  string         `json:"detail,omitempty"`
	Message *store.Message `json:"message,omitempty"`
}

const taskTimeout = 10 * time.Minute

type Runner struct {
	Registry   *agent.Registry
	Store      *store.Store
	Workspaces string

	mu      sync.Mutex
	running map[string]context.CancelFunc
	subs    map[string]map[chan StreamEvent]struct{}
}

func New(reg *agent.Registry, st *store.Store, workspaces string) *Runner {
	os.MkdirAll(workspaces, 0o755)
	return &Runner{
		Registry: reg, Store: st, Workspaces: workspaces,
		running: map[string]context.CancelFunc{},
		subs:    map[string]map[chan StreamEvent]struct{}{},
	}
}

func (r *Runner) IsRunning(sid string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.running[sid]
	return ok
}

// Subscribe returns a channel of events for this session and a cancel func.
func (r *Runner) Subscribe(sid string) (<-chan StreamEvent, func()) {
	ch := make(chan StreamEvent, 64)
	r.mu.Lock()
	if r.subs[sid] == nil {
		r.subs[sid] = map[chan StreamEvent]struct{}{}
	}
	r.subs[sid][ch] = struct{}{}
	r.mu.Unlock()
	return ch, func() {
		r.mu.Lock()
		delete(r.subs[sid], ch)
		r.mu.Unlock()
	}
}

func (r *Runner) broadcast(sid string, ev StreamEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for ch := range r.subs[sid] {
		select {
		case ch <- ev:
		default: // slow consumer: drop rather than block the pump
		}
	}
}

// Send runs the agent for this session and returns immediately; results
// arrive via Subscribe / ListMessages.
func (r *Runner) Send(sid, text string) error {
	ss, err := r.Store.GetSession(sid)
	if err != nil {
		return err
	}
	if ss == nil {
		return os.ErrNotExist
	}
	adapter, ok := r.Registry.Get(ss.Agent)
	if !ok {
		return fmt.Errorf("agent %q unavailable", ss.Agent)
	}

	r.mu.Lock()
	if _, busy := r.running[sid]; busy {
		r.mu.Unlock()
		return ErrBusy
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.running[sid] = cancel
	r.mu.Unlock()

	// persist user message + auto-title (ChatGPT-style, parity with legacy)
	if _, err := r.Store.AddMessage(sid, "user", text, nil); err != nil {
		cancel()
		r.clearRunning(sid)
		return err
	}
	if ss.Title == "" || ss.Title == "New session" || ss.Title == "Nova sessão" {
		title := strings.Join(strings.Fields(text), " ")
		if len(title) > 52 {
			title = title[:52]
		}
		if title != "" {
			r.Store.RenameSession(sid, title)
		}
	}

	cwd := filepath.Join(r.Workspaces, sid)
	os.MkdirAll(cwd, 0o755)
	hasHistory := r.Store.HasAssistantReply(sid)
	argv := adapter.Build(text, ss.AgentRef, cwd, hasHistory)

	r.broadcast(sid, StreamEvent{Type: "state", Running: true})
	go r.pump(sid, adapter, argv, cwd, ctx)
	return nil
}

func (r *Runner) pump(sid string, adapter agent.Adapter, argv []string, cwd string, ctx context.Context) {
	defer r.clearRunning(sid)
	var cancel context.CancelFunc = func() {}
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, taskTimeout)
	}
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	// explicit pipe: merge stderr into the same stream (agents print
	// diagnostics like 'No session found' on stderr)
	pr, pw, err := os.Pipe()
	if err != nil {
		r.finish(sid, "", nil, err.Error(), argv)
		return
	}
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		r.finish(sid, "", nil, err.Error(), argv)
		return
	}
	pw.Close() // parent holds only the reader end

	var (
		textAcc []string
		tools   []map[string]any
		final   string
		errMsg  string
	)
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		for _, ev := range adapter.Parse(line) {
			switch ev.Kind {
			case agent.KindRef:
				r.Store.SetAgentRef(sid, ev.Ref)
			case agent.KindText:
				textAcc = append(textAcc, ev.Content)
				r.broadcast(sid, StreamEvent{Type: "text", Content: ev.Content})
			case agent.KindTool:
				tools = append(tools, map[string]any{
					"name": ev.Name, "state": ev.State, "detail": ev.Detail})
				r.broadcast(sid, StreamEvent{Type: "tool",
					Name: ev.Name, State: ev.State, Detail: ev.Detail})
			case agent.KindFinal:
				final = ev.Content
			case agent.KindError:
				errMsg = ev.Content
			}
		}
	}
	waitErr := cmd.Wait()
	pr.Close()

	content := final
	if content == "" {
		content = strings.Join(textAcc, "")
		content = strings.TrimSpace(content)
	}
	if content == "" && errMsg == "" {
		if waitErr != nil {
			errMsg = waitErr.Error()
		} else {
			errMsg = "(no output)"
		}
	}
	if content == "" {
		content = errMsg
	}
	r.finish(sid, content, tools, errMsg, argv)
}

func (r *Runner) finish(sid, content string, tools []map[string]any, errMsg string, argv []string) {
	if tools == nil {
		tools = []map[string]any{}
	}
	meta := map[string]any{
		"agent": r.sessionAgent(sid),
		"tools": tools,
		"error": errMsg != "",
	}
	if errMsg != "" {
		meta["error_message"] = errMsg
	}
	msg, err := r.Store.AddMessage(sid, "assistant", content, meta)
	if err != nil {
		msg = &store.Message{Role: "assistant", Content: content, Meta: meta}
	}
	r.broadcast(sid, StreamEvent{Type: "message_end", Message: msg})
	r.broadcast(sid, StreamEvent{Type: "state", Running: false})
}

func (r *Runner) sessionAgent(sid string) string {
	ss, _ := r.Store.GetSession(sid)
	if ss == nil {
		return ""
	}
	return ss.Agent
}

func (r *Runner) clearRunning(sid string) {
	r.mu.Lock()
	if cancel, ok := r.running[sid]; ok {
		cancel()
		delete(r.running, sid)
	}
	r.mu.Unlock()
}

// Stop terminates the running process for a session (if any).
func (r *Runner) Stop(sid string) bool {
	r.mu.Lock()
	cancel, ok := r.running[sid]
	r.mu.Unlock()
	if !ok {
		return false
	}
	cancel()
	return true
}
