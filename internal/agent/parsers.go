package agent

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

// adapterSpecs: how each CLI is launched and parsed. Commands were
// calibrated against the real binaries; see docs/adr/0001.
type spec struct {
	bin   string
	build func(p string) Adapter
}

var adapterSpecs = []spec{
	{"claude", func(p string) Adapter {
		return Adapter{
			ID: "claude", Label: "Claude", Color: "#E07856",
			Build: func(text, ref, cwd string, hasHistory bool) []string {
				argv := []string{p, "-p", text, "--output-format", "stream-json",
					"--verbose", "--add-dir", homeDir()}
				if ref != "" {
					argv = append(argv, "--resume", ref)
				}
				return argv
			},
			ApplyControls: func(argv []string, c *Controls) []string {
				if c.Model != "" {
					argv = append(argv, "--model", c.Model)
				}
				if c.Mode != "" {
					argv = append(argv, "--permission-mode", c.Mode)
				}
				return argv // thinking: SDK-only today
			},
			BuildLive: buildClaudeLive(p),
			BuildTUI:  func() []string { return []string{p} },
			Parse:     parseClaude,
		}
	}},
	{"codex", func(p string) Adapter {
		return Adapter{
			ID: "codex", Label: "Codex", Color: "#33B08C",
			BuildLive: buildCodexLive(selfExe(), p),
			ParseLive: parseClaude, // bridge emits the claude dialect
			BuildTUI:  func() []string { return []string{p} },
			Build: func(text, ref, cwd string, hasHistory bool) []string {
				if ref != "" {
					return []string{p, "exec", "resume", ref, "--json",
						"--skip-git-repo-check", "-s", "workspace-write", text}
				}
				return []string{p, "exec", "--json", "--skip-git-repo-check",
					"-s", "workspace-write", text}
			},
			ApplyControls: func(argv []string, c *Controls) []string {
				if c.Model != "" {
					// insert before the positional prompt
					argv = append([]string{argv[0], "-m", c.Model}, argv[1:]...)
				}
				if c.Thinking != "" {
					newArgv := append([]string{}, argv[:len(argv)-1]...)
					newArgv = append(newArgv, "-c", "model_reasoning_effort=\""+c.Thinking+"\"")
					argv = append(newArgv, argv[len(argv)-1])
				}
				if c.Mode != "" {
					// named composer mode -> codex sandbox policy
					sandbox := map[string]string{
						"plan":              "read-only",
						"manual":            "workspace-write",
						"acceptEdits":       "workspace-write",
						"bypassPermissions": "danger-full-access",
					}[c.Mode]
					if sandbox != "" && sandbox != "workspace-write" {
						for i, a := range argv {
							if a == "-s" {
								argv[i+1] = sandbox
							}
						}
					}
				}
				return argv
			},
			Parse: parseCodex,
		}
	}},
	{"grok", func(p string) Adapter {
		return Adapter{
			ID: "grok", Label: "Grok", Color: "#C9CEDC",
			BuildLive: buildGrokLive(selfExe(), p),
			ParseLive: parseClaude, // ACP bridge emits the claude dialect
			BuildTUI:  func() []string { return []string{p} },
			Build: func(text, ref, cwd string, hasHistory bool) []string {
				argv := []string{p, "--output-format", "streaming-json"}
				switch {
				case ref != "":
					argv = append(argv, "--resume", ref) // native resume by id
				case hasHistory:
					argv = append(argv, "--continue") // per-cwd fallback
				}
				return append(argv, "-p", text)
			},
			ApplyControls: func(argv []string, c *Controls) []string {
				tail := argv[len(argv)-1] // positional prompt stays last
				if c.Model != "" {
					argv = append(argv[:len(argv)-1], "-m", c.Model)
				}
				if c.Thinking != "" && c.Thinking != "off" {
					argv = append(argv, "--reasoning-effort", c.Thinking)
				}
				return append(argv, tail)
			},
			Parse: parseGrok,
		}
	}},
	{"pi", func(p string) Adapter {
		return Adapter{
			ID: "pi", Label: "Pi", Color: "#7DA2F7",
			BuildLive: buildPiLive(selfExe(), p),
			ParseLive: parseClaude, // bridge emits the claude dialect
			BuildTUI:  func() []string { return []string{p} },
			Build: func(text, ref, cwd string, hasHistory bool) []string {
				argv := []string{p, "-p", "--mode", "json"}
				if ref != "" {
					argv = append(argv, "--session", ref)
				}
				return append(argv, text)
			},
			ApplyControls: func(argv []string, c *Controls) []string {
				tail := argv[len(argv)-1]
				if c.Model != "" {
					argv = append(argv[:len(argv)-1], "--model", c.Model)
				}
				if c.Thinking != "" {
					argv = append(argv, "--thinking", c.Thinking)
				}
				return append(argv, tail)
			},
			Parse: parsePi,
		}
	}},
	{"opencode", func(p string) Adapter {
		return Adapter{
			ID: "opencode", Label: "OpenCode", Color: "#E5C558",
			BuildLive: buildOpenCodeLive(selfExe(), p),
			ParseLive: parseClaude, // ACP bridge emits the claude dialect
			BuildTUI:  func() []string { return []string{p} },
			Build: func(text, ref, cwd string, hasHistory bool) []string {
				argv := []string{p, "run", "--format", "json", "--dir", cwd}
				if ref != "" {
					argv = append(argv, "--session", ref)
				}
				return append(argv, text)
			},
			ApplyControls: func(argv []string, c *Controls) []string {
				tail := argv[len(argv)-1]
				if c.Model != "" {
					argv = append(argv[:len(argv)-1], "-m", c.Model)
				}
				if c.Thinking != "" {
					argv = append(argv, "--variant", c.Thinking)
				}
				return append(argv, tail)
			},
			Parse: parseOpenCode,
		}
	}},
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

// ---- parsers: one function per agent, each handling the JSONL shapes
// ---- recorded during calibration. Unknown lines are ignored.

func parseClaude(line string) []Event {
	var ev map[string]any
	if json.Unmarshal([]byte(line), &ev) != nil {
		return nil
	}
	var out []Event
	switch ev["type"] {
	case "system":
		if ev["subtype"] == "init" {
			if id, ok := ev["session_id"].(string); ok && id != "" {
				out = append(out, Event{Kind: KindRef, Ref: id})
			}
		}
	case "capabilities":
		if b, err := json.Marshal(ev); err == nil {
			var caps Capabilities
			if json.Unmarshal(b, &caps) == nil && (len(caps.Models) > 0 || len(caps.Modes) > 0) {
				out = append(out, Event{Kind: KindCaps, Caps: &caps})
			}
		}
	case "assistant":
		msg, _ := ev["message"].(map[string]any)
		for _, b := range blocks(msg["content"]) {
			switch b["type"] {
			case "text":
				if t, ok := b["text"].(string); ok && t != "" {
					out = append(out, Event{Kind: KindText, Content: t})
				}
			case "tool_use":
				name, _ := b["name"].(string)
				if name == "" {
					name = "tool"
				}
				out = append(out, Event{Kind: KindTool, Name: name,
					State: "start", Detail: toolDetail(b["input"])})
			}
		}
	case "usage":
		if b, err := json.Marshal(ev); err == nil {
			var u Usage
			if json.Unmarshal(b, &u) == nil && (u.Total > 0 || u.Input > 0 || u.Output > 0 || u.Window > 0) {
				out = append(out, Event{Kind: KindUsage, Usage: &u})
			}
		}
	case "result":
		if id, ok := ev["session_id"].(string); ok && id != "" {
			out = append(out, Event{Kind: KindRef, Ref: id})
		}
		sub, _ := ev["subtype"].(string)
		res, _ := ev["result"].(string)
		if sub == "success" {
			if res != "" {
				out = append(out, Event{Kind: KindFinal, Content: res})
			}
		} else {
			msg := res
			if msg == "" {
				msg = "task failed"
			}
			out = append(out, Event{Kind: KindError, Content: msg})
		}
	case "control_request":
		// agent asks the user (e.g. tool permission) — ADR-0004
		reqID, _ := ev["request_id"].(string)
		if reqID == "" {
			reqID, _ = ev["id"].(string)
		}
		tool, _ := ev["tool_name"].(string)
		if tool == "" {
			tool = "tool"
		}
		input, _ := json.Marshal(ev["input"])
		if string(input) == "null" {
			if alt, ok := ev["tool"].(string); ok {
				input = []byte(alt)
			} else {
				input = []byte("{}")
			}
		}
		out = append(out, Event{Kind: KindControl, Ref: reqID, Name: tool,
			Detail: trunc(string(input), 300)})
	}
	return out
}

func parseCodex(line string) []Event {
	var ev map[string]any
	if json.Unmarshal([]byte(line), &ev) != nil {
		return nil
	}
	var out []Event
	switch ev["type"] {
	case "thread.started":
		if id, ok := ev["thread_id"].(string); ok && id != "" {
			out = append(out, Event{Kind: KindRef, Ref: id})
		}
	case "item.completed":
		item, _ := ev["item"].(map[string]any)
		switch item["type"] {
		case "agent_message":
			if t, ok := item["text"].(string); ok && t != "" {
				out = append(out, Event{Kind: KindText, Content: t},
					Event{Kind: KindFinal, Content: t})
			}
		case "command_execution":
			cmd, _ := item["command"].(string)
			detail := "$ " + cmd
			if ec, ok := item["exit_code"].(float64); ok {
				detail += " → exit " + strconv.Itoa(int(ec))
			}
			out = append(out, Event{Kind: KindTool, Name: "bash",
				State: "end", Detail: trunc(detail, 200)})
		case "file_change":
			path, _ := item["path"].(string)
			out = append(out, Event{Kind: KindTool, Name: "edit",
				State: "end", Detail: trunc(path, 200)})
		case "mcp_tool_call", "web_search":
			name, _ := item["name"].(string)
			out = append(out, Event{Kind: KindTool, Name: name,
				State: "end", Detail: ""})
		}
	case "turn.failed":
		out = append(out, Event{Kind: KindError, Content: trunc(line, 400)})
	}
	return out
}

func parseGrok(line string) []Event {
	var ev map[string]any
	if json.Unmarshal([]byte(line), &ev) != nil {
		return nil
	}
	var out []Event
	t, _ := ev["type"].(string)
	switch {
	case t == "text":
		if d, ok := ev["data"].(string); ok && d != "" {
			out = append(out, Event{Kind: KindText, Content: d})
		}
	case strings.HasPrefix(t, "tool"):
		d, _ := ev["data"].(string)
		if d == "" {
			if raw, err := json.Marshal(ev["data"]); err == nil && string(raw) != "null" {
				d = string(raw)
			}
		}
		out = append(out, Event{Kind: KindTool, Name: t, State: "end",
			Detail: trunc(d, 200)})
	case t == "end":
		if id, ok := ev["sessionId"].(string); ok && id != "" {
			out = append(out, Event{Kind: KindRef, Ref: id})
		}
	}
	return out
}

func parsePi(line string) []Event {
	var ev map[string]any
	if json.Unmarshal([]byte(line), &ev) != nil {
		return nil
	}
	var out []Event
	switch ev["type"] {
	case "session":
		if id, ok := ev["id"].(string); ok && id != "" {
			out = append(out, Event{Kind: KindRef, Ref: id})
		}
	case "message_update":
		ame, _ := ev["assistantMessageEvent"].(map[string]any)
		if ame["type"] == "text_delta" {
			if d, ok := ame["delta"].(string); ok && d != "" {
				out = append(out, Event{Kind: KindText, Content: d})
			}
		}
	case "tool_execution_start":
		name, _ := ev["toolName"].(string)
		if name == "" {
			name = "tool"
		}
		out = append(out, Event{Kind: KindTool, Name: name,
			State: "start", Detail: trunc(toolDetail(ev["args"]), 200)})
	case "tool_execution_end":
		name, _ := ev["toolName"].(string)
		if name == "" {
			name = "tool"
		}
		out = append(out, Event{Kind: KindTool, Name: name, State: "end",
			Detail: boolText(ev["isError"], "error", "ok")})
	case "agent_end":
		var texts []string
		if msgs, ok := ev["messages"].([]any); ok {
			for _, mm := range msgs {
				m, _ := mm.(map[string]any)
				if m["role"] != "assistant" {
					continue
				}
				for _, b := range blocks(m["content"]) {
					if b["type"] == "text" {
						if t, ok := b["text"].(string); ok && t != "" {
							texts = append(texts, t)
						}
					}
				}
			}
		}
		if len(texts) > 0 {
			out = append(out, Event{Kind: KindFinal, Content: strings.Join(texts, "\n\n")})
		}
	}
	return out
}

func parseOpenCode(line string) []Event {
	var ev map[string]any
	if json.Unmarshal([]byte(line), &ev) != nil {
		return nil
	}
	var out []Event
	switch ev["type"] {
	case "error":
		err, _ := ev["error"].(map[string]any)
		msg := "error"
		if data, ok := err["data"].(map[string]any); ok {
			if m, ok := data["message"].(string); ok && m != "" {
				msg = m
			}
		} else if m, ok := err["message"].(string); ok && m != "" {
			msg = m
		}
		out = append(out, Event{Kind: KindError, Content: trunc(msg, 400)})
	case "session.id":
		if id, ok := ev["sessionID"].(string); ok && id != "" {
			out = append(out, Event{Kind: KindRef, Ref: id})
		}
	case "message.part.updated":
		part, _ := ev["part"].(map[string]any)
		switch part["type"] {
		case "text":
			if t, ok := part["text"].(string); ok && t != "" {
				out = append(out, Event{Kind: KindText, Content: t})
			}
		case "tool":
			name, _ := part["tool"].(string)
			if name == "" {
				name = "tool"
			}
			out = append(out, Event{Kind: KindTool, Name: name, State: "end"})
		}
	}
	return out
}

// ---- small helpers to keep parsers declarative ----

func blocks(v any) []map[string]any {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, b := range raw {
		if m, ok := b.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func toolDetail(v any) string {
	b, _ := v.(map[string]any)
	if b == nil {
		return ""
	}
	for _, k := range []string{"command", "cmd", "file_path", "path", "description"} {
		if s, ok := b[k].(string); ok && s != "" {
			return trunc(s, 200)
		}
	}
	return ""
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func boolText(v any, on, off string) string {
	if b, ok := v.(bool); ok && b {
		return on
	}
	return off
}
