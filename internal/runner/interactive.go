package runner

import (
	"context"
	"fmt"
	"os"

	"github.com/cfpperche/agentdeck/internal/tmux"
)

// Tmux is the dock backend. Lazily constructed so tests that never
// open a TUI do not require the binary.
func (r *Runner) tmuxMgr() *tmux.Manager {
	return tmux.New()
}

// HasTUI reports whether this session currently owns an interactive TUI.
func (r *Runner) HasTUI(sid string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tui[sid] != ""
}

// TUIName is the tmux session name, or "".
func (r *Runner) TUIName(sid string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tui[sid]
}

// TUIAvailable is true when tmux is on PATH (dock can work).
func (r *Runner) TUIAvailable() bool { return r.tmuxMgr().Available() }

// StartTUI stops the protocol process (exclusive) and starts the
// agent's interactive TUI in tmux. Idempotent if already running.
func (r *Runner) StartTUI(sid string) (string, error) {
	ss, err := r.Store.GetSession(sid)
	if err != nil || ss == nil {
		return "", os.ErrNotExist
	}
	adapter, ok := r.Registry.Get(ss.Agent)
	if !ok || adapter.BuildTUI == nil {
		return "", fmt.Errorf("%s has no TUI", ss.Agent)
	}
	tm := r.tmuxMgr()
	if !tm.Available() {
		return "", fmt.Errorf("tmux not installed")
	}
	r.dropLive(sid)

	name := tmux.SessionName(sid)
	cwd := r.sessionDir(sid, ss.Cwd)
	argv := adapter.BuildTUI()
	if err := tm.NewSession(context.Background(), name, cwd, argv[0], argv[1:]...); err != nil {
		return "", err
	}
	r.mu.Lock()
	r.tui[sid] = name
	r.mu.Unlock()
	r.broadcast(sid, StreamEvent{Type: "state", Running: false, Status: "idle", Surface: "terminal"})
	return name, nil
}

// StopTUI kills the tmux session (used when chat takes over or the
// AgentDeck session is deleted). Detach-only is just closing the WS.
func (r *Runner) StopTUI(sid string) {
	r.mu.Lock()
	name := r.tui[sid]
	delete(r.tui, sid)
	r.mu.Unlock()
	if name == "" {
		name = tmux.SessionName(sid)
	}
	_ = r.tmuxMgr().KillSession(context.Background(), name)
	r.broadcast(sid, StreamEvent{Type: "state", Running: r.IsRunning(sid), Status: string(r.Status(sid)), Surface: "chat"})
}

// dropLive kills the protocol process so a TUI can take the session.
func (r *Runner) dropLive(sid string) {
	r.mu.Lock()
	lp := r.live[sid]
	delete(r.live, sid)
	r.mu.Unlock()
	if lp == nil || lp.cmd == nil || lp.cmd.Process == nil {
		return
	}
	lp.mu.Lock()
	lp.closed = true
	lp.mu.Unlock()
	_ = killGroup(lp.cmd.Process.Pid)
}

// Surface is "terminal" when a TUI owns the session, else "chat".
func (r *Runner) Surface(sid string) string {
	if r.HasTUI(sid) {
		return "terminal"
	}
	return "chat"
}
