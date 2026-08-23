// Package runner manages agent process lifecycles: spawn, stream-parse,
// broadcast to subscribers, persist results. One process per session
// (live tier) or per turn (fallback tier), with a session state machine
// and a message queue for steering (benchmark study G1/G3).
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
	"syscall"
	"time"

	"github.com/cfpperche/agentdeck/internal/agent"
	"github.com/cfpperche/agentdeck/internal/store"
)

// SessionStatus is the session state machine (benchmark study G1,
// after t3code's RuntimeSessionState, simplified):
//
//	idle    — no turn in flight
//	running — a turn is in flight
//	waiting — a turn is in flight AND the agent asked the user
//	          something (permission); answering returns to running
type SessionStatus string

const (
	StatusIdle    SessionStatus = "idle"
	StatusRunning SessionStatus = "running"
	StatusWaiting SessionStatus = "waiting"
)

// QueueCap bounds how many messages may wait while a turn is in flight.
const QueueCap = 5

var (
	// ErrBusy is returned when the queue is full (turn in flight).
	ErrBusy = errors.New("queue full: too many messages waiting")
)

// StreamEvent is the client-facing event (SSE payload shape).
// `running` is derived (status != idle) and kept for compatibility;
// `status` is the authoritative field.
type StreamEvent struct {
	Type      string         `json:"type"` // state | text | tool | message_end | permission | queue
	Running   bool           `json:"running"`
	Status    string         `json:"status,omitempty"`
	Content   string         `json:"content,omitempty"`
	Name      string         `json:"name,omitempty"`
	State     string         `json:"state,omitempty"`
	Detail    string         `json:"detail,omitempty"`
	Message   *store.Message `json:"message,omitempty"`
	RequestID string         `json:"request_id,omitempty"` // permission events (ADR-0004)
	Tool      string         `json:"tool,omitempty"`
	Input     string         `json:"input,omitempty"`
	Count     int            `json:"count,omitempty"` // queue events
}

const taskTimeout = 10 * time.Minute

type Runner struct {
	Registry   *agent.Registry
	Store      *store.Store
	Workspaces string

	mu      sync.Mutex
	running map[string]context.CancelFunc // fallback cancels + live busy marker
	live    map[string]*liveProc          // persistent tier-1 processes (ADR-0004)
	state   map[string]SessionStatus
	pending map[string][]StreamEvent // unanswered permission requests, in order
	queues  map[string][]string    // messages waiting for the current turn
	subs    map[string]map[chan StreamEvent]struct{}
}

func New(reg *agent.Registry, st *store.Store, workspaces string) *Runner {
	os.MkdirAll(workspaces, 0o755)
	return &Runner{
		Registry: reg, Store: st, Workspaces: workspaces,
		running: map[string]context.CancelFunc{},
		live:    map[string]*liveProc{},
		state:   map[string]SessionStatus{},
		pending: map[string][]StreamEvent{},
		queues:  map[string][]string{},
		subs:    map[string]map[chan StreamEvent]struct{}{},
	}
}

// Status returns the current session status (idle when unknown).
func (r *Runner) Status(sid string) SessionStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	if st, ok := r.state[sid]; ok {
		return st
	}
	return StatusIdle
}

// IsRunning reports whether a turn is in flight (running or waiting).
func (r *Runner) IsRunning(sid string) bool {
	return r.Status(sid) != StatusIdle
}

// setStatus mutates and broadcasts the new state.
func (r *Runner) setStatus(sid string, st SessionStatus) {
	r.mu.Lock()
	r.state[sid] = st
	r.mu.Unlock()
	r.broadcast(sid, StreamEvent{
		Type: "state", Status: string(st), Running: st != StatusIdle,
	})
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

// PendingPermissions returns all unanswered permission events for a
// session (for late SSE subscribers), in order.
func (r *Runner) PendingPermissions(sid string) []StreamEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]StreamEvent, len(r.pending[sid]))
	copy(out, r.pending[sid])
	return out
}

// Send delivers a message or queues it while a turn is in flight
// (steering). The user message is persisted immediately either way;
// queued messages are delivered automatically when the turn ends.
// Returns (queued, error): queued=true means accepted into the queue.
func (r *Runner) Send(sid, text string) (bool, error) {
	ss, err := r.Store.GetSession(sid)
	if err != nil {
		return false, err
	}
	if ss == nil {
		return false, os.ErrNotExist
	}
	adapter, ok := r.Registry.Get(ss.Agent)
	if !ok {
		return false, fmt.Errorf("agent %q unavailable", ss.Agent)
	}

	// turn in flight? queue for delivery at turn end (G3).
	// NOTE: nothing is persisted unless accepted (cap check first).
	if r.IsRunning(sid) {
		r.mu.Lock()
		if len(r.queues[sid]) >= QueueCap {
			r.mu.Unlock()
			return false, ErrBusy
		}
		if _, err := r.Store.AddMessage(sid, "user", text, nil); err != nil {
			r.mu.Unlock()
			return false, err
		}
		r.queues[sid] = append(r.queues[sid], text)
		n := len(r.queues[sid])
		r.mu.Unlock()
		r.broadcast(sid, StreamEvent{Type: "queue", Count: n})
		return true, nil
	}

	// idle: persist user message + auto-title, then deliver
	if _, err := r.Store.AddMessage(sid, "user", text, nil); err != nil {
		return false, err
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

	return false, r.deliver(sid, ss, adapter, text)
}

// deliver starts a turn with a (already persisted) user message.
func (r *Runner) deliver(sid string, ss *store.Session, adapter agent.Adapter, text string) error {
	// tier-1 (ADR-0004): persistent bidirectional process — no spawn
	if adapter.BuildLive != nil {
		if err := r.sendLive(sid, adapter, text); err != nil {
			return err
		}
		r.mu.Lock()
		r.running[sid] = func() {} // busy marker; Stop()/finish clear it
		r.mu.Unlock()
		r.setStatus(sid, StatusRunning)
		return nil
	}

	// fallback: one-shot spawn per turn
	ctx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.running[sid] = cancel
	r.mu.Unlock()

	cwd := filepath.Join(r.Workspaces, sid)
	os.MkdirAll(cwd, 0o755)
	hasHistory := r.Store.HasAssistantReply(sid)
	argv := adapter.Build(text, ss.AgentRef, cwd, hasHistory)

	r.setStatus(sid, StatusRunning)
	go r.pump(sid, adapter, argv, cwd, ctx)
	return nil
}

// drain delivers queued messages after a turn ends; stops at the first
// delivery so each turn gets its own lifecycle.
func (r *Runner) drain(sid string) {
	r.mu.Lock()
	q := r.queues[sid]
	if len(q) == 0 {
		r.mu.Unlock()
		return
	}
	text := q[0]
	r.queues[sid] = q[1:]
	r.mu.Unlock()

	ss, err := r.Store.GetSession(sid)
	if err != nil || ss == nil {
		return // session deleted mid-queue: drop
	}
	adapter, ok := r.Registry.Get(ss.Agent)
	if !ok {
		return
	}
	if err := r.deliver(sid, ss, adapter, text); err != nil {
		// deliver failed: drop the rest quietly (message is in history)
		r.ClearQueue(sid)
	}
}

// ClearQueue discards all queued messages; returns how many were dropped.
func (r *Runner) ClearQueue(sid string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := len(r.queues[sid])
	delete(r.queues, sid)
	return n
}

// QueueLen reports how many messages are waiting.
func (r *Runner) QueueLen(sid string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.queues[sid])
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
	// own process group + group-kill: agents spawn children that inherit
	// the pipe; killing only the parent leaves the pipe open (scanner
	// blocks until the child exits — caught by CI on TestStopPersistsPartial)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return os.ErrProcessDone
	}
	cmd.WaitDelay = 3 * time.Second
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
	r.clearRunning(sid)
	r.broadcast(sid, StreamEvent{Type: "message_end", Message: msg})
	r.setStatus(sid, StatusIdle)
	// any queued message? deliver the next turn now (G3)
	r.drain(sid)
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

// Stop cancels the current turn (fallback tier: kills the process;
// live tier: lets it run to completion) and clears the message queue.
// Returns true if anything was active.
func (r *Runner) Stop(sid string) bool {
	active := r.IsRunning(sid)
	r.ClearQueue(sid)
	r.clearRunning(sid)
	if active {
		r.setStatus(sid, StatusIdle)
	}
	return active
}
