package codexbridge

import (
	"encoding/json"
	"testing"
)

func TestIngestModelsPicksDefaultAndThinking(t *testing.T) {
	b := &Bridge{}
	rep := line{"result": map[string]any{
		"data": []any{
			map[string]any{
				"id": "gpt-5.6-terra", "displayName": "GPT-5.6-Terra", "isDefault": true,
				"hidden": false, "defaultReasoningEffort": "low",
				"supportedReasoningEfforts": []any{
					map[string]any{"reasoningEffort": "low", "description": "Fast"},
					map[string]any{"reasoningEffort": "high", "description": "Deep"},
				},
			},
			map[string]any{"id": "hidden-x", "displayName": "X", "hidden": true},
			map[string]any{"id": "gpt-5.5", "displayName": "GPT-5.5", "isDefault": false},
		},
	}}
	b.ingestModels(rep)
	if len(b.models) != 2 || b.model != "gpt-5.6-terra" {
		t.Fatalf("models=%v default=%s", b.models, b.model)
	}
	think, _ := b.models[0]["thinking_options"].([]map[string]any)
	if len(think) != 2 || think[0]["id"] != "low" || think[0]["is_default"] != true {
		t.Fatalf("thinking %#v", think)
	}
}

func TestExtractErrUnwrapsCodexJSON(t *testing.T) {
	raw := `{"type":"error","status":400,"error":{"type":"invalid_request_error","message":"The 'gpt-5.6-sol' model is not supported when using Codex with a ChatGPT account."}}`
	got := extractErr(raw)
	if got == raw || got == "" {
		t.Fatalf("did not unwrap: %q", got)
	}
}

func TestAsIntFromJSONNumber(t *testing.T) {
	var m map[string]any
	json.Unmarshal([]byte(`{"id":3}`), &m)
	id, ok := asInt(m["id"])
	if !ok || id != 3 {
		t.Fatalf("id=%d ok=%v", id, ok)
	}
}

func TestThreadFrom(t *testing.T) {
	if threadFrom(nil) != "" {
		t.Fatal("nil")
	}
	id := threadFrom(line{"result": map[string]any{"thread": map[string]any{"id": "abc"}}})
	if id != "abc" {
		t.Fatal(id)
	}
}
