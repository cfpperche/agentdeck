// Package pibridge drives pi's native RPC mode (`pi --mode rpc`) and
// translates to AgentDeck's wire protocol (ADR-0007 tier-1 native).
// Protocol receipts: paseo providers/pi/{rpc-types,cli-runtime}.ts,
// verified against the installed CLI on 2026-08-24: NDJSON lines;
// requests carry {id,type,...}; responses {type:"response",id,success};
// streaming arrives as notifications (message_update etc.), a turn ends
// on agent_end. The response to `prompt` is only an ack.
package pibridge

import (
	"bufio"
	"io"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

type piline = map[string]any

// piConn is an NDJSON connection that routes responses to per-call
// channels and everything else (events/notifications) to Events.
type piConn struct {
	r *bufio.Reader
	w *syncWriter

	mu      sync.Mutex
	next    int64
	replies map[int64]chan piline

	Events chan piline // notifications/events from pi
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

func newPiConn(r io.Reader, w io.Writer) *piConn {
	return &piConn{
		r: bufio.NewReaderSize(r, 1<<20),
		w: &syncWriter{w: w},
		replies: map[int64]chan piline{},
		Events:  make(chan piline, 256),
	}
}

func debugf(f string, a ...any) {
	if os.Getenv("PIRPCDEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[pibridge] "+f+"\n", a...)
	}
}

func (c *piConn) writeLine(m piline) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	debugf("out %s", b)
	return c.w.WriteLine(b)
}

func (c *piConn) call(m piline) int64 {
	c.mu.Lock()
	c.next++
	id := c.next
	ch := make(chan piline, 1)
	c.replies[id] = ch
	c.mu.Unlock()
	m["id"] = id
	if err := c.writeLine(m); err != nil {
		c.mu.Lock()
		delete(c.replies, id)
		c.mu.Unlock()
		close(ch)
	}
	return id
}

// Reader runs the read loop forever, routing as described above.
func (c *piConn) Reader() {
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			debugf("reader end: %v", err)
			close(c.Events)
			return
		}
		var m piline
		if json.Unmarshal([]byte(line), &m) != nil {
			continue // tolerate banners/junk
		}
		if os.Getenv("PIRPCDEBUG") != "" {
			dbg, _ := json.Marshal(m)
			fmt.Fprintln(os.Stderr, "[pi-in] "+string(dbg))
		}
		t, _ := m["type"].(string)
		idf, hasID := m["id"]
		if t == "response" && hasID {
			id, _ := toInt(idf)
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
		c.Events <- m
	}
}

func toInt(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	}
	return 0, false
}

// ---- Bridge over AgentDeck's wire ----

type Bridge struct {
	c *piConn

	mu     sync.Mutex
	models []map[string]any // live catalog entries
	turn   strings.Builder
	pending bool // a prompt is in flight
}

func emit(v any) {
	b, _ := json.Marshal(v)
	fmt.Fprintln(os.Stdout, string(b))
}

func (b *Bridge) snapshotModels() []map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]map[string]any, len(b.models))
	copy(out, b.models)
	return out
}

func (b *Bridge) ingestModels(data json.RawMessage) {
	var d struct {
		Models []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Provider string `json:"provider"`
		} `json:"models"`
	}
	if json.Unmarshal(data, &d) != nil {
		return
	}
	out := []map[string]any{}
	for _, m := range d.Models {
		full := m.ID
		if m.Provider != "" && !strings.Contains(full, "/") {
			full = m.Provider + "/" + full
		}
		out = append(out, map[string]any{"id": full, "label": m.Name})
	}
	b.mu.Lock()
	b.models = out
	b.mu.Unlock()
}

func (b *Bridge) sessionRef() string {
	return os.Getenv("AGENTDECK_AGENT_REF")
}

// Run serves our wire on os.Stdin/os.Stdout while driving the pi
// process through piOut/piIn.
func Run(piOut io.Reader, piIn io.Writer) error {
	b := &Bridge{c: newPiConn(piOut, piIn)}
	go b.c.Reader()

	// live catalog → capabilities
	id := b.c.call(piline{"type": "get_available_models"})
	replyCh := waitReply(b.c, id)
	got := false
	for !got {
		select {
		case m, ok := <-b.c.Events:
			if !ok {
				return fmt.Errorf("pi closed before catalog")
			}
			b.handleEvent(m)
		case rep, ok := <-replyCh:
			if ok {
				b.ingestModels(dataField(rep))
			}
			got = true
		}
	}
	emit(map[string]any{"type": "capabilities", "models": b.snapshotModels(), "modes": []any{}})
	emit(map[string]any{"type": "system", "subtype": "init", "session_id": b.sessionRef()})

	// forward pi events to the wire forever
	go func() {
		for m := range b.c.Events {
			b.handleEvent(m)
		}
	}()

	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		b.handleWire(sc.Text())
	}
	return nil
}

// handleEvent translates pi notifications into wire lines.
func (b *Bridge) handleEvent(m piline) {
	t, _ := m["type"].(string)
	switch t {
	case "message_update":
		var u struct {
			Event struct {
				Type  string `json:"type"`
				Delta string `json:"delta"`
			} `json:"assistantMessageEvent"`
		}
		if json.Unmarshal(mustBytes(m), &u) == nil &&
			u.Event.Type == "text_delta" && u.Event.Delta != "" {
			b.mu.Lock()
			b.turn.WriteString(u.Event.Delta)
			b.mu.Unlock()
			emit(map[string]any{"type": "assistant", "message": map[string]any{
				"content": []map[string]any{{"type": "text", "text": u.Event.Delta}}}})
		}
	case "tool_execution_start":
		name, _ := m["toolName"].(string)
		args, _ := json.Marshal(m["args"])
		emit(map[string]any{"type": "assistant", "message": map[string]any{
			"content": []map[string]any{{"type": "tool_use",
				"name": name, "input": json.RawMessage(args)}}}})
	case "agent_end", "agent_settled":
		// both signal turn completion depending on pi version/state;
		// emit exactly one result per prompt (dedup via b.pending).
		b.mu.Lock()
		final := strings.TrimSpace(b.turn.String())
		b.turn.Reset()
		wasPending := b.pending
		b.pending = false
		b.mu.Unlock()
		if !wasPending {
			return
		}
		if final == "" {
			// provider failed silently (free models do this) — finish the
			// turn as an error instead of leaving it running forever
			emit(map[string]any{"type": "result", "subtype": "error",
				"session_id": b.sessionRef(),
				"result":     "pi returned an empty response (provider error?)"})
			return
		}
		emit(map[string]any{"type": "result", "subtype": "success",
			"session_id": b.sessionRef(), "result": final})
	}
}

func (b *Bridge) handleWire(lineStr string) {
	var in struct {
		Type    string `json:"type"`
		Message struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
		Controls struct {
			Model    string `json:"model"`
			Thinking string `json:"thinking"`
		} `json:"controls"`
	}
	if json.Unmarshal([]byte(lineStr), &in) != nil {
		return
	}
	switch in.Type {
	case "set_controls":
		if in.Controls.Model != "" {
			req := piline{"type": "set_model"}
			if parts := strings.SplitN(in.Controls.Model, "/", 2); len(parts) == 2 {
				req["provider"] = parts[0]
				req["modelId"] = parts[1]
			} else {
				req["modelId"] = parts[0]
			}
			waitReply(b.c, b.c.call(req))
		}
		if in.Controls.Thinking != "" {
			waitReply(b.c, b.c.call(piline{"type": "set_thinking_level", "level": in.Controls.Thinking}))
		}
	case "user":
		text := ""
		for _, c := range in.Message.Content {
			if c.Type == "text" {
				text += c.Text
			}
		}
		// ack-only reply; real completion arrives as agent_end/settled
		b.mu.Lock()
		b.pending = true
		b.mu.Unlock()
		waitReply(b.c, b.c.call(piline{"type": "prompt", "message": text}))
	}
}

// waitReply returns a channel delivering the reply line for id.
func waitReply(c *piConn, id int64) <-chan piline {
	out := make(chan piline, 1)
	c.mu.Lock()
	ch := c.replies[id]
	c.mu.Unlock()
	if ch == nil {
		close(out)
		return out
	}
	go func() { out <- <-ch; close(out) }()
	return out
}

func dataField(m piline) json.RawMessage {
	d, _ := json.Marshal(m["data"])
	return d
}

func mustBytes(m piline) []byte { b, _ := json.Marshal(m); return b }
