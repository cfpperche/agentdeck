// Live sessions (ADR-0004, tier 1): persistent bidirectional agent
// processes — the web UI as a native client of the agent's protocol.
// The TUI dock (ADR-0008) is a separate exclusive process; never run
// both against the same session.
package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/cfpperche/agentdeck/internal/agent"
)

// liveProc is one persistent agent process owned by a session.
type liveProc struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  *json.Encoder
	closed bool
	// pid observed at start — tests assert persistence across turns
	startedAt time.Time
}

func (lp *liveProc) alive() bool {
	if lp == nil || lp.cmd == nil || lp.cmd.Process == nil {
		return false
	}
	lp.mu.Lock()
	closed := lp.closed
	lp.mu.Unlock()
	if closed {
		return false // stream ended — pump observed EOF/crash
	}
	return lp.cmd.ProcessState == nil || lp.cmd.ProcessState.ExitCode() == -1
}

func (lp *liveProc) write(v any) error {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	if lp.closed {
		return fmt.Errorf("live process closed")
	}
	return lp.stdin.Encode(v)
}

// ensureLive returns the session's live process, spawning or
// restarting-with-ref it as needed. Caller holds no locks.
func (r *Runner) ensureLive(sid string, adapter agent.Adapter) (*liveProc, error) {
	r.mu.Lock()
	lp := r.live[sid]
	r.mu.Unlock()

	if lp.alive() {
		return lp, nil
	}
	// dead or absent → (re)start, resuming the native session if we have one
	ss, err := r.Store.GetSession(sid)
	if err != nil || ss == nil {
		return nil, os.ErrNotExist
	}
	cwd := r.sessionDir(sid, ss.Cwd)
	argv := adapter.BuildLive(ss.AgentRef, cwd)

	cmd := exec.CommandContext(context.Background(), argv[0], argv[1:]...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	if c := r.LastControls(sid); c != nil && c.OpMode == "readonly" {
		cmd.Env = append(cmd.Env, "AGENTDECK_PI_TOOLS=read,grep,find,ls")
	}
	// SDK shim (ADR-0005): pass the native ref so a restarted shim resumes
	if ss.AgentRef != "" && argv[0] == "node" {
		cmd.Env = append(cmd.Env, "AGENTDECK_SDK_RESUME="+ss.AgentRef)
	}
	// ACP bridge (ADR-0007): native session ref for loadSession
	if ss.AgentRef != "" {
		cmd.Env = append(cmd.Env, "AGENTDECK_AGENT_REF="+ss.AgentRef)
	}
	if pm := os.Getenv("AGENTDECK_SDK_PERMISSION_MODE"); pm != "" {
		cmd.Env = append(cmd.Env, "AGENTDECK_SDK_PERMISSION_MODE="+pm)
	}
	// process group (war story #1 in HANDOFF): kill children too
	setProcAttr(cmd)
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return killGroup(cmd.Process.Pid)
		}
		return os.ErrProcessDone
	}
	cmd.WaitDelay = 3 * time.Second

	// explicit pipe: stderr merged into the parse stream (war story #2)
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	cmd.Stdout = pw
	cmd.Stderr = pw
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		pr.Close()
		pw.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return nil, err
	}
	pw.Close()

	lp = &liveProc{cmd: cmd, stdin: json.NewEncoder(stdinPipe), startedAt: time.Now()}
	r.mu.Lock()
	r.live[sid] = lp
	r.mu.Unlock()

	go r.pumpLive(sid, adapter, lp, pr, stdinPipe)
	return lp, nil
}

// pumpLive reads the persistent stream, broadcasts events and finishes
// turns on KindFinal (one assistant message per turn, chat semantics).
func (r *Runner) pumpLive(sid string, adapter agent.Adapter, lp *liveProc, pr *os.File, stdinPipe interface{ Close() error }) {
	var textAcc []string
	var tools []map[string]any
	var final string
	var errMsg string

	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if os.Getenv("SHIMDEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[shim-raw] %s\n", line)
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		parse := adapter.Parse
		if adapter.ParseLive != nil {
			parse = adapter.ParseLive // bridge dialect (ADR-0007)
		}
		for _, ev := range parse(line) {
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
			case agent.KindControl:
				// ask the user; turn stays open until Control() answers.
				// G1: session enters 'waiting'; snapshot for late subscribers.
				pev := StreamEvent{Type: "permission",
					RequestID: ev.Ref, Tool: ev.Name, Input: ev.Detail}
				r.mu.Lock()
				r.pending[sid] = append(r.pending[sid], pev)
				r.mu.Unlock()
				r.setStatus(sid, StatusWaiting)
				r.broadcast(sid, pev)
			case agent.KindUsage:
				if ev.Usage != nil {
					r.mu.Lock()
					r.usage[sid] = ev.Usage
					r.mu.Unlock()
				}
			case agent.KindCaps:
				// composer surface: remember for late subscribers and push.
				r.mu.Lock()
				r.caps[sid] = ev.Caps
				r.mu.Unlock()
				r.broadcast(sid, capsEvent(ev.Caps))
			case agent.KindFinal:
				final = ev.Content
			case agent.KindError:
				// a result-level failure ends the turn (upstream provider
				// errors arrive as result subtype:error via bridges)
				errMsg = ev.Content
				final = ev.Content
			}
		}
		if final != "" {
			// turn complete → persist + close (chat message per turn)
			r.finish(sid, final, tools, errMsg, nil)
			final = ""
			errMsg = ""
			textAcc = nil
			tools = nil
		}
	}
	// stream ended: process died. If a turn was in flight, persist what
	// we have and mark idle; next Send restarts with ref.
	if len(textAcc) > 0 || len(tools) > 0 {
		content := strings.Join(textAcc, "")
		if content == "" {
			content = "(agent process exited unexpectedly)"
		}
		r.finish(sid, content, tools, "", nil)
	} else if r.IsRunning(sid) {
		r.clearRunning(sid)
		r.setStatus(sid, StatusIdle)
	}
	lp.mu.Lock()
	lp.closed = true
	lp.mu.Unlock()
	stdinPipe.Close()
	pr.Close()
}

// sendLive writes a user message into the living process.
func (r *Runner) sendLive(sid string, adapter agent.Adapter, text string) error {
	lp, err := r.ensureLive(sid, adapter)
	if err != nil {
		return err
	}
	// composer controls (ADR-0006): push the current selection before the
	// message whenever one exists — the shim merges it into the next turn.
	r.mu.Lock()
	c := r.ctrls[sid]
	r.mu.Unlock()
	if c != nil && (c.Model != "" || c.Thinking != "" || c.Mode != "" || c.Provider != "" || c.Kind != "" || c.OpMode != "") {
		if err := lp.write(map[string]any{
			"type": "set_controls", "model": c.Model, "thinking": c.Thinking,
			"permission_mode": c.Mode, "provider": c.Provider, "kind": c.Kind, "op_mode": c.OpMode,
			"controls": c,
		}); err != nil {
			return err
		}
	}
	msg := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": text},
			},
		},
	}
	if c != nil {
		msg["controls"] = c
	}
	return lp.write(msg)
}

// Control answers a permission request (allow/deny, with optional
// edited input) on a live session. G7: updatedInput flows to the agent.
func (r *Runner) Control(sid string, requestID, behavior string, updatedInput json.RawMessage) error {
	r.mu.Lock()
	lp := r.live[sid]
	r.mu.Unlock()
	if !lp.alive() {
		return fmt.Errorf("no live process for session")
	}
	if behavior != "allow" && behavior != "deny" {
		return fmt.Errorf("behavior must be allow or deny")
	}
	resp := map[string]any{"behavior": behavior}
	if behavior == "allow" && len(updatedInput) > 0 {
		var parsed any
		if json.Unmarshal(updatedInput, &parsed) == nil {
			resp["updatedInput"] = parsed
		}
	}
	err := lp.write(map[string]any{
		"type":       "control_response",
		"request_id": requestID,
		"response":   resp,
	})
	if err == nil {
		r.mu.Lock()
		q := r.pending[sid]
		out := q[:0]
		for _, p := range q {
			if p.RequestID != requestID {
				out = append(out, p)
			}
		}
		r.pending[sid] = out
		r.mu.Unlock()
		r.setStatus(sid, StatusRunning) // G1: back to work
	}
	return err
}
