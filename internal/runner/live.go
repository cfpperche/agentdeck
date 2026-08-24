// Live sessions (ADR-0004, tier 1): persistent bidirectional agent
// processes — the web UI as a native client of the agent's protocol,
// like the TUIs are. No tmux anywhere.
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
	"syscall"
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
	// SDK shim (ADR-0005): pass the native ref so a restarted shim resumes
	if ss.AgentRef != "" && argv[0] == "node" {
		cmd.Env = append(cmd.Env, "AGENTDECK_SDK_RESUME="+ss.AgentRef)
	}
	if pm := os.Getenv("AGENTDECK_SDK_PERMISSION_MODE"); pm != "" {
		cmd.Env = append(cmd.Env, "AGENTDECK_SDK_PERMISSION_MODE="+pm)
	}
	// process group (war story #1 in HANDOFF): kill children too
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
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
			case agent.KindFinal:
				final = ev.Content
			case agent.KindError:
				textAcc = append(textAcc, ev.Content)
			}
		}
		if final != "" {
			// turn complete → persist + close (chat message per turn)
			r.finish(sid, final, tools, "", nil)
			final = ""
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
	return lp.write(map[string]any{
		"type": "user",
		"message": map[string]any{
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": text},
			},
		},
	})
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
