package acp

import (
	"encoding/json"
	"testing"
)

func TestEmitUsageFromGrokNotification(t *testing.T) {
	raw := json.RawMessage(`{
		"sessionId":"s1",
		"update":{
			"sessionUpdate":"response_completed",
			"usage":{"input_tokens":2609,"output_tokens":32,"cache_read":100}
		}
	}`)
	// extract via the same maps the emitter walks
	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	u := top["update"].(map[string]any)["usage"].(map[string]any)
	if anyInt(u["input_tokens"]) != 2609 || anyInt(u["output_tokens"]) != 32 {
		t.Fatalf("%v", u)
	}
}

func TestEmitUsageFromGrokPromptMeta(t *testing.T) {
	raw := json.RawMessage(`{
		"stopReason":"end_turn",
		"_meta":{"totalTokens":14289,"inputTokens":14257,"outputTokens":32,"cachedReadTokens":8000}
	}`)
	var top map[string]any
	json.Unmarshal(raw, &top)
	meta := top["_meta"].(map[string]any)
	if anyInt(meta["totalTokens"]) != 14289 || anyInt(meta["cachedReadTokens"]) != 8000 {
		t.Fatalf("%v", meta)
	}
	// walking the helper should not panic
	emitUsageFromACP(raw)
}

func TestAnyInt(t *testing.T) {
	if anyInt(float64(3)) != 3 || anyInt(int64(4)) != 4 || anyInt(nil) != 0 {
		t.Fatal("anyInt")
	}
}
