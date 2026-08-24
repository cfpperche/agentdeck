// Package acp implements a minimal Agent Client Protocol client
// (ADR-0007): NDJSON JSON-RPC over stdio, as spoken by `opencode acp`
// (verified against opencode 1.18.20 on 2026-08-24) and `grok agent
// stdio` (grok 1.0.5, verified 2026-08-24).
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

// Conn is a framed ACP connection. Safe for concurrent use.
type Conn struct {
	w  io.Writer
	wc *sync.Mutex
	r  *bufio.Reader

	mu   sync.Mutex
	next int64
	pend map[int64]chan rpcMsg
}

type rpcMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

// ErrField is the wire error (named Err to dodge json tag clash).
func (m *rpcMsg) ErrField() json.RawMessage { return m.Error }

func NewConn(r io.Reader, w io.Writer) *Conn {
	return &Conn{w: w, wc: &sync.Mutex{}, r: bufio.NewReaderSize(r, 1<<20), pend: map[int64]chan rpcMsg{}}
}

func (c *Conn) write(m rpcMsg) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if os.Getenv("ACPDEBUG") != "" {
		fmt.Fprintln(os.Stderr, "[acp-out] "+string(b))
	}
	c.wc.Lock()
	defer c.wc.Unlock()
	_, err = c.w.Write(append(b, '\n'))
	return err
}

// Call sends a request and returns its result. The reply is delivered
// to whichever of Wait()/Next() consumes it first.
func (c *Conn) Call(method string, params any) (int64, error) {
	c.mu.Lock()
	c.next++
	id := c.next
	ch := make(chan rpcMsg, 1)
	c.pend[id] = ch
	c.mu.Unlock()
	p, _ := json.Marshal(params)
	return id, c.write(rpcMsg{JSONRPC: "2.0", ID: id, Method: method, Params: p})
}

func (c *Conn) respond(id int64, result any) error {
	r, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return c.write(rpcMsg{JSONRPC: "2.0", ID: id, Result: r})
}

// Notify sends a notification (no id, no reply expected).
func (c *Conn) Notify(method string, params any) error {
	p, _ := json.Marshal(params)
	return c.write(rpcMsg{JSONRPC: "2.0", Method: method, Params: p})
}

type Next struct {
	Method string
	Params json.RawMessage
	ID     int64
	Result json.RawMessage // set when this completes a Call
	Err    json.RawMessage
}

// Next reads the next message. Requests from the agent surface with
// Method set (caller must Respond); replies to our calls surface with
// Result/Err set.
func (c *Conn) Next() (*Next, error) {
	for {
		line, err := c.r.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		var m rpcMsg
		if json.Unmarshal(line, &m) != nil || m.JSONRPC != "2.0" {
			continue // tolerate junk lines (agents may print banners)
		}
		if m.Method != "" && m.ID == 0 {
			return &Next{Method: m.Method, Params: m.Params}, nil // notification
		}
		if m.Method != "" { // request FROM the agent — must answer
			return &Next{Method: m.Method, Params: m.Params, ID: m.ID}, nil
		}
		if m.ID == 0 && m.Result == nil {
			continue
		}
		// reply to one of our calls
		c.mu.Lock()
		ch := c.pend[m.ID]
		delete(c.pend, m.ID)
		c.mu.Unlock()
		if ch != nil {
			ch <- m
			close(ch)
		}
		continue
	}
}

// Wait blocks until the reply for call id arrives or the stream ends.
func (c *Conn) Wait(id int64) (json.RawMessage, json.RawMessage, error) {
	c.mu.Lock()
	ch := c.pend[id]
	c.mu.Unlock()
	if ch == nil {
		return nil, nil, io.ErrClosedPipe
	}
	m, ok := <-ch
	if !ok {
		return nil, nil, io.ErrClosedPipe
	}
	return m.Result, m.ErrField(), nil
}

// Respond answers a request that came from the agent.
func (c *Conn) Respond(id int64, result any) error { return c.respond(id, result) }

// ---- typed helpers over the verified opencode surface ----

type InitializeResult struct {
	ProtocolVersion int             `json:"protocolVersion"`
	AgentInfo       json.RawMessage `json:"agentInfo,omitempty"`
}

func (c *Conn) Initialize(protocolVersion int) (*InitializeResult, error) {
	id, err := c.Call("initialize", map[string]any{"protocolVersion": protocolVersion})
	if err != nil {
		return nil, err
	}
	res, errRes, err := c.Wait(id)
	if err != nil {
		return nil, err
	}
	if errRes != nil {
		return nil, ErrFrom(errRes)
	}
	var out InitializeResult
	return &out, json.Unmarshal(res, &out)
}

type SessionConfigOption struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	CurrentValue string `json:"currentValue"`
	Options      []struct {
		Value string `json:"value"`
		Name  string `json:"name"`
	} `json:"options,omitempty"`
	Category string `json:"category,omitempty"`
}

type SessionNewResult struct {
	SessionID     string                `json:"sessionId"`
	ConfigOptions []SessionConfigOption `json:"configOptions,omitempty"`
	// grok 1.0.5+ (ACP session/new) — not configOptions
	Models *SessionModels `json:"models,omitempty"`
	Meta   json.RawMessage `json:"_meta,omitempty"`
}

type SessionModels struct {
	CurrentModelID  string `json:"currentModelId"`
	AvailableModels []struct {
		ModelID string `json:"modelId"`
		Name    string `json:"name"`
	} `json:"availableModels"`
}

func (c *Conn) NewSession(cwd string) (*SessionNewResult, error) {
	id, err := c.Call("session/new", map[string]any{"cwd": cwd, "mcpServers": []any{}})
	if err != nil {
		return nil, err
	}
	res, errRes, err := c.Wait(id)
	if err != nil {
		return nil, err
	}
	if errRes != nil {
		return nil, ErrFrom(errRes)
	}
	var out SessionNewResult
	return &out, json.Unmarshal(res, &out)
}

func (c *Conn) SetModel(sessionID, modelID string) (*SessionNewResult, error) {
	// grok: session/set_model; opencode: session/set_config_option
	res, errRes, err := c.callWait("session/set_model",
		map[string]any{"sessionId": sessionID, "modelId": modelID})
	if isMethodMissing(errRes) {
		res, errRes, err = c.callWait("session/set_config_option",
			map[string]any{"sessionId": sessionID, "configId": "model", "value": modelID})
	}
	if err != nil {
		return nil, err
	}
	if errRes != nil {
		return nil, ErrFrom(errRes)
	}
	var out SessionNewResult
	if res != nil {
		json.Unmarshal(res, &out)
	}
	return &out, nil
}

// SetMode maps composer thinking → grok session/set_mode (modeId =
// low|medium|high|xhigh). opencode has no equivalent; method-missing is ok.
func (c *Conn) SetMode(sessionID, modeID string) error {
	_, errRes, err := c.callWait("session/set_mode",
		map[string]any{"sessionId": sessionID, "modeId": modeID})
	if err != nil {
		return err
	}
	if isMethodMissing(errRes) {
		return nil
	}
	if errRes != nil {
		return ErrFrom(errRes)
	}
	return nil
}

func (c *Conn) callWait(method string, params any) (json.RawMessage, json.RawMessage, error) {
	id, err := c.Call(method, params)
	if err != nil {
		return nil, nil, err
	}
	return c.Wait(id)
}

func isMethodMissing(errRes json.RawMessage) bool {
	if errRes == nil {
		return false
	}
	var e struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	json.Unmarshal(errRes, &e)
	return e.Code == -32601 || strings.Contains(strings.ToLower(e.Message), "not found")
}

// Prompt sends one user turn. Streaming updates arrive via Next()
// (session/update notifications) until the caller sees the reply.
// Prompt sends one user turn and returns the call id; streaming
// updates arrive via Next() until the caller sees the reply for id.
func (c *Conn) Prompt(sessionID, text string) (int64, error) {
	return c.Call("session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": text}},
	})
}

// ---- permission request shape (from the ACP spec / observed fields) ----

type PermissionRequest struct {
	SessionID string `json:"sessionId"`
	ToolCall  struct {
		ToolCallID string `json:"toolCallId"`
		Title      string `json:"title,omitempty"`
		RawInput   any    `json:"rawInput,omitempty"`
	} `json:"toolCall"`
	Options []struct {
		OptionID string `json:"optionId"`
		Kind     string `json:"kind"` // allow_once | allow_always | reject_once | reject_always
		Name     string `json:"name"`
	} `json:"options"`
}

// PickOutcome maps an AgentDeck behavior onto the agent's offered
// option ids (never invents an id the agent didn't offer).
func PickOutcome(pr *PermissionRequest, behavior string) string {
	wantAllow := behavior == "allow"
	fallback := ""
	for _, o := range pr.Options {
		isAllow := o.Kind == "allow_once" || o.Kind == "allow_always"
		if wantAllow == isAllow {
			return o.OptionID
		}
		fallback = o.OptionID
	}
	return fallback
}

func ErrFrom(raw json.RawMessage) error {
	var e struct {
		Message string `json:"message"`
	}
	json.Unmarshal(raw, &e)
	if e.Message == "" {
		e.Message = "acp error"
	}
	return &RPCError{Message: e.Message}
}

type RPCError struct{ Message string }

func (e *RPCError) Error() string { return "acp: " + e.Message }
