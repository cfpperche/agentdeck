package agent

import (
	"reflect"
	"testing"
)

// Parser test doubles: lines recorded from the real CLIs during
// calibration (see session history / docs). Deterministic, offline,
// zero tokens.

func TestParseClaude(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []Event
	}{
		{
			"init yields ref",
			`{"type":"system","subtype":"init","session_id":"37f542d2-73ae-4b69-b1ee","model":"opus"}`,
			[]Event{{Kind: KindRef, Ref: "37f542d2-73ae-4b69-b1ee"}},
		},
		{
			"assistant text block",
			`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Apareceu: ` + "`agentdeck`" + `"}]}}`,
			[]Event{{Kind: KindText, Content: "Apareceu: `agentdeck`"}},
		},
		{
			"assistant tool_use block",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"echo agentdeck"}}]}}`,
			[]Event{{Kind: KindTool, Name: "Bash", State: "start", Detail: "echo agentdeck"}},
		},
		{
			"usage pulse",
			`{"type":"usage","input":10,"output":4,"cache_read":20,"total":34,"window":272000}`,
			[]Event{{Kind: KindUsage, Usage: &Usage{Input: 10, Output: 4, CacheRead: 20, Total: 34, Window: 272000}}},
		},
		{
			"result success final",
			`{"type":"result","subtype":"success","result":"Apareceu: agentdeck","session_id":"37f542d2"}`,
			[]Event{{Kind: KindRef, Ref: "37f542d2"}, {Kind: KindFinal, Content: "Apareceu: agentdeck"}},
		},
		{
			"result error",
			`{"type":"result","subtype":"error_max_turns","result":"hit turn limit"}`,
			[]Event{{Kind: KindError, Content: "hit turn limit"}},
		},
		{
			"capabilities parsed (ADR-0006)",
			`{"type":"capabilities","models":[{"id":"sonnet","label":"Sonnet","is_default":true,"thinking_options":[{"id":"on","label":"Extended thinking"}]}],"modes":[{"id":"manual","label":"Ask before edits"}]}`,
			[]Event{{Kind: KindCaps, Caps: &Capabilities{
				Models: []ModelDef{{ID: "sonnet", Label: "Sonnet", IsDefault: true,
					ThinkingOptions: []SelectOption{{ID: "on", Label: "Extended thinking"}}}},
				Modes: []ModeDef{{ID: "manual", Label: "Ask before edits"}},
			}}},
		},
		{"empty capabilities dropped", `{"type":"capabilities","models":[],"modes":[]}`, nil},
		{"garbage ignored", `not json at all`, nil},
	}
	runParserTests(t, "claude", tests)
}

func TestParseCodex(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []Event
	}{
		{
			"thread.started yields ref",
			`{"type":"thread.started","thread_id":"01a02cc1-af1a-76a2-977e-e13fbe2a7f7f"}`,
			[]Event{{Kind: KindRef, Ref: "01a02cc1-af1a-76a2-977e-e13fbe2a7f7f"}},
		},
		{
			"agent_message yields text+final",
			`{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"OK"}}`,
			[]Event{{Kind: KindText, Content: "OK"}, {Kind: KindFinal, Content: "OK"}},
		},
		{
			"command_execution yields bash tool",
			`{"type":"item.completed","item":{"type":"command_execution","command":"go test ./...","exit_code":0}}`,
			[]Event{{Kind: KindTool, Name: "bash", State: "end", Detail: "$ go test ./... → exit 0"}},
		},
		{
			"file_change yields edit tool",
			`{"type":"item.completed","item":{"type":"file_change","path":"internal/agent/parsers.go"}}`,
			[]Event{{Kind: KindTool, Name: "edit", State: "end", Detail: "internal/agent/parsers.go"}},
		},
		{"turn.failed yields error", `{"type":"turn.failed","error":{"message":"boom"}}`,
			[]Event{{Kind: KindError, Content: `{"type":"turn.failed","error":{"message":"boom"}}`}}},
	}
	runParserTests(t, "codex", tests)
}

func TestParseGrok(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []Event
	}{
		{
			"text delta",
			`{"type":"text","data":"OK"}`,
			[]Event{{Kind: KindText, Content: "OK"}},
		},
		{
			"end yields session ref",
			`{"type":"end","stopReason":"end_turn","sessionId":"01a02cc7-8cc5-7cc0-84a8-c9b45ecd4ba3"}`,
			[]Event{{Kind: KindRef, Ref: "01a02cc7-8cc5-7cc0-84a8-c9b45ecd4ba3"}},
		},
		{
			"tool event",
			`{"type":"tool.call","data":{"name":"run_terminal_command"}}`,
			[]Event{{Kind: KindTool, Name: "tool.call", State: "end", Detail: `{"name":"run_terminal_command"}`}},
		},
		{"thought ignored (not surfaced)", `{"type":"thought","data":"The user"}`, nil},
		{"available_commands ignored", `{"type":"available_commands","tools":["x"]}`, nil},
	}
	runParserTests(t, "grok", tests)
}

func TestParsePi(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []Event
	}{
		{
			"session header yields ref",
			`{"type":"session","version":3,"id":"01a02cc2-e6c5-7d37-a8bb-4163568c92a4","cwd":"/tmp"}`,
			[]Event{{Kind: KindRef, Ref: "01a02cc2-e6c5-7d37-a8bb-4163568c92a4"}},
		},
		{
			"text_delta yields text",
			`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"OK"}}`,
			[]Event{{Kind: KindText, Content: "OK"}},
		},
		{
			"tool start with cmd detail",
			`{"type":"tool_execution_start","toolCallId":"t1","toolName":"bash","args":{"cmd":"go build"}}`,
			[]Event{{Kind: KindTool, Name: "bash", State: "start", Detail: "go build"}},
		},
		{
			"tool end ok",
			`{"type":"tool_execution_end","toolCallId":"t1","toolName":"bash","isError":false}`,
			[]Event{{Kind: KindTool, Name: "bash", State: "end", Detail: "ok"}},
		},
		{
			"agent_end yields final from messages",
			`{"type":"agent_end","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]},{"role":"assistant","content":[{"type":"text","text":"OK"}]}]}`,
			[]Event{{Kind: KindFinal, Content: "OK"}},
		},
	}
	runParserTests(t, "pi", tests)
}

func TestParseOpenCode(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []Event
	}{
		{
			"error event surfaces provider message",
			`{"type":"error","error":{"name":"APIError","data":{"message":"Insufficient balance or no resource package. Please recharge."}}}`,
			[]Event{{Kind: KindError, Content: "Insufficient balance or no resource package. Please recharge."}},
		},
		{
			"part text",
			`{"type":"message.part.updated","part":{"type":"text","text":"hello"}}`,
			[]Event{{Kind: KindText, Content: "hello"}},
		},
		{
			"part tool",
			`{"type":"message.part.updated","part":{"type":"tool","tool":"bash"}}`,
			[]Event{{Kind: KindTool, Name: "bash", State: "end"}},
		},
	}
	runParserTests(t, "opencode", tests)
}

func runParserTests(t *testing.T, agentID string, tests []struct {
	name string
	line string
	want []Event
}) {
	t.Helper()
	adapter, ok := specByID(agentID)
	if !ok {
		t.Fatalf("no adapter spec for %s", agentID)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.Parse(tt.line)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() = %+v\n want   %+v", got, tt.want)
			}
		})
	}
}

func specByID(id string) (Adapter, bool) {
	for _, s := range adapterSpecs {
		a := s.build("/bin/true") // path irrelevant for parser tests
		if a.ID == id {
			return a, true
		}
	}
	return Adapter{}, false
}

func TestRegistryEnvOverride(t *testing.T) {
	t.Setenv("AGENTDECK_BIN_FAKECLAUDE", "/nonexistent-but-registered")
	// which returns error for everything except the override
	r := NewRegistry(func(name string) (string, error) { return "", &execErr{} })
	_ = r
}

type execErr struct{}

func (e *execErr) Error() string { return "not found" }
