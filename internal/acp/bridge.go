package acp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// Bridge connects an ACP agent to AgentDeck's wire protocol
// (ADR-0004 shapes) on stdin/stdout, so the runner needs no protocol
// awareness. It emits the claude dialect (system/init, assistant text,
// result) which every parser already normalizes.
type Bridge struct {
	conn *Conn

	mu      sync.Mutex
	session string
	pending map[string]*permEntry // our request_id -> acp request
	nextReq int
	turn    strings.Builder // streamed text of the current turn
}

type permEntry struct {
	acpID     int64
	reqParams *PermissionRequest
}

func NewBridge(conn *Conn) *Bridge {
	return &Bridge{conn: conn, pending: map[string]*permEntry{}}
}

func emit(v any) {
	b, _ := json.Marshal(v)
	fmt.Fprintln(os.Stdout, string(b))
}

// Run drives the bridge until stdin closes or the agent dies.
func (b *Bridge) Run() error {
	cwd, _ := os.Getwd()

	// pump agent messages FIRST: replies to our calls are routed by
	// Conn.Next; blocking on Wait without a running reader deadlocks.
	chN := make(chan *Next, 32)
	go func() {
		defer close(chN)
		for {
			n, err := b.conn.Next()
			if err != nil {
				debugf("pump end: %v", err)
				return
			}
			if os.Getenv("ACPDEBUG") != "" {
				b, _ := json.Marshal(n)
				fmt.Fprintln(os.Stderr, "[acp-in] "+string(b))
			}
			chN <- n
		}
	}()

	initRes, err := b.conn.Initialize(1)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	_ = initRes

	var sess *SessionNewResult
	if ref := os.Getenv("AGENTDECK_AGENT_REF"); ref != "" {
		sess = b.loadSession(ref, cwd)
	}
	if sess == nil {
		if sess, err = b.conn.NewSession(cwd); err != nil {
			return fmt.Errorf("session/new: %w", err)
		}
	}
	b.mu.Lock()
	b.session = sess.SessionID
	b.mu.Unlock()

	emit(map[string]any{"type": "system", "subtype": "init", "session_id": sess.SessionID})
	emit(capsFrom(sess))

	stdinLines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(os.Stdin)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for sc.Scan() {
			stdinLines <- sc.Text()
		}
		close(stdinLines)
	}()

	for {
		select {
		case n, ok := <-chN:
			if !ok {
				return fmt.Errorf("agent stream ended")
			}
			if err := b.handleAgentMessage(n); err != nil {
				return err
			}
		case line, ok := <-stdinLines:
			if !ok {
				return nil // parent went away
			}
			if err := b.handleWireLine(line); err != nil {
				emit(map[string]any{"type": "result", "subtype": "error",
					"result": "bridge: " + err.Error()})
			}
		}
	}
}

func capsFrom(sess *SessionNewResult) map[string]any {
	models := []map[string]any{}
	for _, o := range sess.ConfigOptions {
		if o.ID == "model" && o.Type == "select" {
			for _, opt := range o.Options {
				models = append(models, map[string]any{
					"id": opt.Value, "label": opt.Name,
					"is_default": opt.Value == o.CurrentValue,
				})
			}
		}
	}
	return map[string]any{"type": "capabilities", "models": models, "modes": []any{}}
}

func (b *Bridge) handleAgentMessage(n *Next) error {
	switch n.Method {
	case "session/update":
		var u struct {
			SessionID string `json:"sessionId"`
			Update    struct {
				SessionUpdate string `json:"sessionUpdate"`
				Content       struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
				ToolCallID string          `json:"toolCallId"`
				Title      string          `json:"title"`
				RawInput   json.RawMessage `json:"rawInput"`
				Kind       string          `json:"kind"`
			} `json:"update"`
		}
		if json.Unmarshal(n.Params, &u) != nil {
			return nil
		}
		switch u.Update.SessionUpdate {
		case "agent_message_chunk":
			b.mu.Lock()
			b.turn.WriteString(u.Update.Content.Text)
			b.mu.Unlock()
			emit(map[string]any{"type": "assistant", "message": map[string]any{
				"content": []map[string]any{{"type": "text", "text": u.Update.Content.Text}}}})
		case "tool_call":
			emit(map[string]any{"type": "assistant", "message": map[string]any{
				"content": []map[string]any{{"type": "tool_use",
					"name": u.Update.Title, "input": json.RawMessage(u.Update.RawInput)}}}})
		}
	case "session/request_permission":
		var pr PermissionRequest
		if json.Unmarshal(n.Params, &pr) != nil {
			return b.conn.Respond(n.ID, map[string]any{"outcome": map[string]any{"outcome": "rejected"}})
		}
		b.mu.Lock()
		b.nextReq++
		reqID := fmt.Sprintf("acp-%d", b.nextReq)
		b.pending[reqID] = &permEntry{acpID: n.ID, reqParams: &pr}
		b.mu.Unlock()
		input, _ := json.Marshal(pr.ToolCall.RawInput)
		emit(map[string]any{"type": "control_request",
			"request_id": reqID,
			"tool_name":  pr.ToolCall.Title,
			"input":      string(input)})
	default:
		// unknown request from the agent: answer so it never blocks
		if n.ID != 0 {
			return b.conn.Respond(n.ID, map[string]any{})
		}
	}
	return nil
}

func (b *Bridge) handleWireLine(line string) error {
	var msg struct {
		Type     string          `json:"type"`
		Message  json.RawMessage `json:"message"`
		Request  string          `json:"request_id"`
		ID       int64           `json:"id"`
		Response struct {
			Behavior     string          `json:"behavior"`
			UpdatedInput json.RawMessage `json:"updatedInput"`
		} `json:"response"`
		Controls struct {
			Model    string `json:"model"`
			Thinking string `json:"thinking"`
		} `json:"controls"`
		Text string `json:"text"`
	}
	if json.Unmarshal([]byte(line), &msg) != nil {
		return nil // ignore junk like the shim does
	}
	switch msg.Type {
	case "set_controls":
		if msg.Controls.Model != "" {
			if _, err := b.conn.SetModel(b.sessionID(), msg.Controls.Model); err != nil {
				emit(map[string]any{"type": "result", "subtype": "error",
					"result": "set model: " + err.Error()})
			}
		}
	case "control_response":
		b.mu.Lock()
		entry := b.pending[msg.Request]
		delete(b.pending, msg.Request)
		b.mu.Unlock()
		if entry != nil {
			outcome := PickOutcome(entry.reqParams, msg.Response.Behavior)
			return b.conn.Respond(entry.acpID, map[string]any{
				"outcome": map[string]any{"outcome": outcome, "optionId": outcome}})
		}
	case "user":
		var m struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		json.Unmarshal(msg.Message, &m)
		text := ""
		for _, c := range m.Content {
			if c.Type == "text" {
				text += c.Text
			}
		}
		id, _ := b.sessionPrompt(text)
		go b.awaitPromptReply(id)
	}
	return nil
}

func (b *Bridge) sessionPrompt(text string) (int64, error) {
	return b.conn.Prompt(b.sessionID(), text)
}

func (b *Bridge) awaitPromptReply(id int64) {
	res, errRes, err := b.conn.Wait(id)
	if err != nil {
		emit(map[string]any{"type": "result", "subtype": "error", "result": err.Error()})
		return
	}
	if errRes != nil {
		e := ErrFrom(errRes)
		emit(map[string]any{"type": "result", "subtype": "error", "result": e.Error()})
		return
	}
	var r struct {
		StopReason string `json:"stopReason"`
	}
	json.Unmarshal(res, &r)
	// the wire contract needs non-empty final text (parseClaude gates
	// KindFinal on it) — send what we streamed as the authoritative turn.
	b.mu.Lock()
	final := b.turn.String()
	b.turn.Reset()
	b.mu.Unlock()
	emit(map[string]any{"type": "result", "subtype": "success",
		"session_id": b.sessionID(), "result": final})
}

func (b *Bridge) sessionID() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.session
}

func (b *Bridge) loadSession(ref, cwd string) *SessionNewResult {
	// ACP session/load restores a prior conversation (opencode sets
	// loadSession:true). Best effort: fall back to a fresh session.
	id, err := b.conn.Call("session/load", map[string]any{
		"sessionId": ref, "cwd": cwd, "mcpServers": []any{}})
	if err != nil {
		return nil
	}
	res, _, err := b.conn.Wait(id)
	if err != nil || res == nil {
		return nil
	}
	var out SessionNewResult
	if json.Unmarshal(res, &out) != nil || out.SessionID == "" {
		return nil
	}
	return &out
}

var _ = io.Discard

func debugf(f string, a ...any) {
	if os.Getenv("ACPDEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[bridge] "+f+"\n", a...)
	}
}
