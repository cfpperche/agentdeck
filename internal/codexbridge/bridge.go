// Package codexbridge drives Codex's experimental app-server
// (JSON-RPC NDJSON over stdio) and translates to AgentDeck's wire
// (ADR-0007 tier-1).
//
// Receipts: `codex app-server generate-json-schema` (0.149.0) +
// live handshake 2026-08-24 + tachyon poc-plano-interno-codex.
//
// Lifecycle: initialize → notify initialized → model/list →
// thread/start|resume → turn/start (ack only) → stream
// item/agentMessage/delta → turn/completed.
// Inbound replies omit the "jsonrpc" field — do not require it.
package codexbridge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

type line = map[string]any

type conn struct {
	r *bufio.Reader
	w *syncWriter

	mu      sync.Mutex
	next    int64
	replies map[int64]chan line
	Events  chan line
}

type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *syncWriter) WriteLine(b []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := w.w.Write(append(b, '\n'))
	return err
}

func newConn(r io.Reader, w io.Writer) *conn {
	return &conn{
		r: bufio.NewReaderSize(r, 1<<20),
		w: &syncWriter{w: w},
		replies: map[int64]chan line{},
		Events:  make(chan line, 256),
	}
}

func debugf(f string, a ...any) {
	if os.Getenv("CODEXASDEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[codexas] "+f+"\n", a...)
	}
}

func (c *conn) write(m line) error {
	if _, ok := m["jsonrpc"]; !ok {
		m["jsonrpc"] = "2.0"
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	debugf("out %s", b)
	return c.w.WriteLine(b)
}

func (c *conn) call(method string, params any) int64 {
	c.mu.Lock()
	c.next++
	id := c.next
	ch := make(chan line, 1)
	c.replies[id] = ch
	c.mu.Unlock()
	_ = c.write(line{"id": id, "method": method, "params": params})
	return id
}

func (c *conn) notify(method string, params any) {
	_ = c.write(line{"method": method, "params": params})
}

func (c *conn) respond(id any, result any) {
	_ = c.write(line{"id": id, "result": result})
}

func (c *conn) wait(id int64) line {
	c.mu.Lock()
	ch := c.replies[id]
	c.mu.Unlock()
	if ch == nil {
		return nil
	}
	return <-ch
}

// pump reads NDJSON. Replies (id, no method) go to the waiter;
// everything else (notifications + server requests) goes to Events.
// Codex omits "jsonrpc" on inbound — we do not require it.
func (c *conn) pump() {
	defer close(c.Events)
	for {
		b, err := c.r.ReadBytes('\n')
		if err != nil {
			return
		}
		var m line
		if json.Unmarshal(b, &m) != nil {
			continue
		}
		debugf("in %s", bytesTrim(b))
		if method, _ := m["method"].(string); method == "" {
			if id, ok := asInt(m["id"]); ok {
				c.mu.Lock()
				ch := c.replies[id]
				delete(c.replies, id)
				c.mu.Unlock()
				if ch != nil {
					ch <- m
					close(ch)
				}
				continue
			}
		}
		c.Events <- m
	}
}

func bytesTrim(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}

func asInt(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}

// Bridge speaks app-server towards Codex and our wire towards the runner.
type Bridge struct {
	c *conn

	mu      sync.Mutex
	thread  string
	turn    strings.Builder
	pending bool
	model   string
	effort  string
	models  []map[string]any
	perms   map[string]any // our request_id -> server request id
	nextP   int
}

func emit(v any) {
	b, _ := json.Marshal(v)
	fmt.Fprintln(os.Stdout, string(b))
}

// Run drives the bridge until stdin closes or the agent dies.
func Run(fromAgent io.Reader, toAgent io.Writer) error {
	c := newConn(fromAgent, toAgent)
	go c.pump()
	b := &Bridge{c: c, perms: map[string]any{}}
	return b.loop()
}

func (b *Bridge) loop() error {
	cwd, _ := os.Getwd()

	initID := b.c.call("initialize", map[string]any{
		"clientInfo": map[string]any{"name": "agentdeck", "title": "AgentDeck", "version": "dev"},
	})
	if rep := b.c.wait(initID); rep == nil || rep["error"] != nil {
		return fmt.Errorf("initialize: %v", rep)
	}
	b.c.notify("initialized", map[string]any{})

	b.ingestModels(b.c.wait(b.c.call("model/list", map[string]any{})))

	var threadID string
	if ref := os.Getenv("AGENTDECK_AGENT_REF"); ref != "" {
		rep := b.c.wait(b.c.call("thread/resume", map[string]any{"threadId": ref}))
		threadID = threadFrom(rep)
	}
	if threadID == "" {
		params := map[string]any{
			"cwd":            cwd,
			"sandbox":        "workspace-write",
			"approvalPolicy": "on-request",
		}
		if b.model != "" {
			params["model"] = b.model
		}
		rep := b.c.wait(b.c.call("thread/start", params))
		threadID = threadFrom(rep)
		if threadID == "" {
			return fmt.Errorf("thread/start: %v", rep)
		}
	}
	b.mu.Lock()
	b.thread = threadID
	b.mu.Unlock()

	emit(map[string]any{"type": "system", "subtype": "init", "session_id": threadID})
	emit(map[string]any{"type": "capabilities", "models": b.models, "modes": defaultModes()})

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
		case ev, ok := <-b.c.Events:
			if !ok {
				return fmt.Errorf("codex stream ended")
			}
			b.handleAgent(ev)
		case line, ok := <-stdinLines:
			if !ok {
				return nil
			}
			b.handleWire(line)
		}
	}
}

func threadFrom(rep line) string {
	if rep == nil {
		return ""
	}
	res, _ := rep["result"].(map[string]any)
	if res == nil {
		return ""
	}
	th, _ := res["thread"].(map[string]any)
	if th == nil {
		return ""
	}
	id, _ := th["id"].(string)
	return id
}

func (b *Bridge) ingestModels(rep line) {
	if rep == nil {
		return
	}
	res, _ := rep["result"].(map[string]any)
	if res == nil {
		return
	}
	raw, _ := res["data"].([]any)
	models := []map[string]any{}
	var def string
	for _, it := range raw {
		m, _ := it.(map[string]any)
		if m == nil {
			continue
		}
		if hidden, _ := m["hidden"].(bool); hidden {
			continue
		}
		id, _ := m["id"].(string)
		if id == "" {
			id, _ = m["model"].(string)
		}
		label, _ := m["displayName"].(string)
		if label == "" {
			label = id
		}
		isDef, _ := m["isDefault"].(bool)
		if isDef {
			def = id
		}
		entry := map[string]any{"id": id, "label": label, "is_default": isDef}
		if think := thinkingFrom(m); len(think) > 0 {
			entry["thinking_options"] = think
		}
		models = append(models, entry)
	}
	b.mu.Lock()
	b.models = models
	if b.model == "" {
		b.model = def
		if b.model == "" && len(models) > 0 {
			b.model, _ = models[0]["id"].(string)
		}
	}
	b.mu.Unlock()
}

func thinkingFrom(m map[string]any) []map[string]any {
	raw, _ := m["supportedReasoningEfforts"].([]any)
	out := []map[string]any{}
	def, _ := m["defaultReasoningEffort"].(string)
	for _, it := range raw {
		switch v := it.(type) {
		case string:
			out = append(out, map[string]any{"id": v, "label": v, "is_default": v == def})
		case map[string]any:
			id, _ := v["reasoningEffort"].(string)
			if id == "" {
				id, _ = v["id"].(string)
			}
			label, _ := v["description"].(string)
			if label == "" {
				label = id
			}
			out = append(out, map[string]any{"id": id, "label": shortEffort(id, label), "is_default": id == def})
		}
	}
	return out
}

func shortEffort(id, desc string) string {
	switch id {
	case "low":
		return "Low"
	case "medium":
		return "Medium"
	case "high":
		return "High"
	case "xhigh":
		return "Extra high"
	case "max":
		return "Max"
	case "ultra":
		return "Ultra"
	}
	if desc != "" && len(desc) < 24 {
		return desc
	}
	return id
}

func defaultModes() []map[string]any {
	return []map[string]any{
		{"id": "manual", "label": "Ask before edits", "description": "Sandbox: workspace-write"},
		{"id": "plan", "label": "Plan only", "description": "Sandbox: read-only"},
		{"id": "bypassPermissions", "label": "Full access", "description": "Sandbox: danger-full-access"},
	}
}

func (b *Bridge) handleAgent(m line) {
	method, _ := m["method"].(string)
	params, _ := m["params"].(map[string]any)
	if params == nil {
		params = map[string]any{}
	}
	switch method {
	case "item/agentMessage/delta":
		delta, _ := params["delta"].(string)
		if delta == "" {
			return
		}
		b.mu.Lock()
		b.turn.WriteString(delta)
		b.mu.Unlock()
		emit(map[string]any{"type": "assistant", "message": map[string]any{
			"content": []map[string]any{{"type": "text", "text": delta}}}})
	case "item/started":
		item, _ := params["item"].(map[string]any)
		if item == nil {
			return
		}
		typ, _ := item["type"].(string)
		if typ == "commandExecution" || typ == "fileChange" || typ == "mcpToolCall" || typ == "webSearch" {
			name, _ := item["command"].(string)
			if name == "" {
				name, _ = item["tool"].(string)
			}
			if name == "" {
				name = typ
			}
			raw, _ := json.Marshal(item)
			emit(map[string]any{"type": "assistant", "message": map[string]any{
				"content": []map[string]any{{"type": "tool_use", "name": name, "input": json.RawMessage(raw)}}}})
		}
	case "item/completed":
		item, _ := params["item"].(map[string]any)
		if item == nil {
			return
		}
		if typ, _ := item["type"].(string); typ == "agentMessage" {
			if text, _ := item["text"].(string); text != "" {
				b.mu.Lock()
				if b.turn.Len() == 0 {
					b.turn.WriteString(text)
					emit(map[string]any{"type": "assistant", "message": map[string]any{
						"content": []map[string]any{{"type": "text", "text": text}}}})
				}
				b.mu.Unlock()
			}
		}
	case "turn/completed":
		turn, _ := params["turn"].(map[string]any)
		status, _ := turn["status"].(string)
		b.mu.Lock()
		final := strings.TrimSpace(b.turn.String())
		b.turn.Reset()
		was := b.pending
		b.pending = false
		sid := b.thread
		b.mu.Unlock()
		if !was {
			return
		}
		if status == "failed" || status == "interrupted" {
			msg := "codex turn " + status
			if errObj, _ := turn["error"].(map[string]any); errObj != nil {
				if m, _ := errObj["message"].(string); m != "" {
					msg = extractErr(m)
				}
			}
			emit(map[string]any{"type": "result", "subtype": "error", "session_id": sid, "result": msg})
			return
		}
		if final == "" {
			final = "done"
		}
		emit(map[string]any{"type": "result", "subtype": "success", "session_id": sid, "result": final})
	case "error":
		// surface mid-turn provider errors in the transcript; turn/completed follows
		if errObj, _ := params["error"].(map[string]any); errObj != nil {
			if msg, _ := errObj["message"].(string); msg != "" {
				debugf("error %s", extractErr(msg))
			}
		}
	case "execCommandApproval", "applyPatchApproval",
		"item/commandExecution/requestApproval", "item/fileChange/requestApproval",
		"item/permissions/requestApproval":
		b.askPermission(m)
	default:
		// unknown server request: answer so Codex never blocks
		if _, ok := m["id"]; ok && method != "" {
			b.c.respond(m["id"], map[string]any{})
		}
	}
}

func extractErr(raw string) string {
	var wrap struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal([]byte(raw), &wrap) == nil {
		if wrap.Error.Message != "" {
			return wrap.Error.Message
		}
		if wrap.Message != "" {
			return wrap.Message
		}
	}
	return raw
}

func (b *Bridge) askPermission(m line) {
	params, _ := m["params"].(map[string]any)
	if params == nil {
		params = map[string]any{}
	}
	name, _ := params["command"].(string)
	if name == "" {
		name, _ = params["reason"].(string)
	}
	if name == "" {
		name, _ = m["method"].(string)
	}
	input, _ := json.Marshal(params)
	b.mu.Lock()
	b.nextP++
	reqID := fmt.Sprintf("codex-%d", b.nextP)
	b.perms[reqID] = m["id"]
	b.mu.Unlock()
	emit(map[string]any{"type": "control_request", "request_id": reqID,
		"tool_name": name, "input": string(input)})
}

func (b *Bridge) handleWire(lineStr string) {
	var msg struct {
		Type     string `json:"type"`
		Message  json.RawMessage `json:"message"`
		Request  string `json:"request_id"`
		Response struct {
			Behavior string `json:"behavior"`
		} `json:"response"`
		Controls struct {
			Model    string `json:"model"`
			Thinking string `json:"thinking"`
			Mode     string `json:"mode"`
		} `json:"controls"`
	}
	if json.Unmarshal([]byte(lineStr), &msg) != nil {
		return
	}
	switch msg.Type {
	case "set_controls":
		b.mu.Lock()
		if msg.Controls.Model != "" {
			b.model = msg.Controls.Model
		}
		if msg.Controls.Thinking != "" {
			b.effort = msg.Controls.Thinking
		}
		b.mu.Unlock()
	case "control_response":
		b.mu.Lock()
		id := b.perms[msg.Request]
		delete(b.perms, msg.Request)
		b.mu.Unlock()
		if id == nil {
			return
		}
		var decision any = map[string]any{"denied": map[string]any{"rejection": "user denied"}}
		if msg.Response.Behavior == "allow" {
			decision = "approved"
		}
		b.c.respond(id, map[string]any{"decision": decision})
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
		b.startTurn(text)
	}
}

func (b *Bridge) startTurn(text string) {
	b.mu.Lock()
	b.pending = true
	b.turn.Reset()
	params := map[string]any{
		"threadId": b.thread,
		"input":    []map[string]any{{"type": "text", "text": text}},
	}
	if b.model != "" {
		params["model"] = b.model
	}
	if b.effort != "" {
		params["effort"] = b.effort
	}
	b.mu.Unlock()
	// ack only — completion is turn/completed
	go b.c.wait(b.c.call("turn/start", params))
}
