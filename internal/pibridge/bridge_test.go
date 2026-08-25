package pibridge

import (
	"encoding/json"
	"testing"
)

func TestIngestModelsGroupsProviders(t *testing.T) {
	b := &Bridge{}
	raw, _ := json.Marshal(map[string]any{
		"models": []any{
			map[string]any{"id": "ox-alpha", "name": "ox-alpha", "provider": "openrouter", "reasoning": true},
			map[string]any{"id": "stealth/ox-alpha", "name": "stealth/ox-alpha", "provider": "openrouter", "reasoning": true},
			map[string]any{"id": "gpt-5.4", "name": "GPT-5.4", "provider": "openai", "reasoning": false},
		},
	})
	b.ingestModels(raw)
	if len(b.providers) != 2 {
		t.Fatalf("providers %d", len(b.providers))
	}
	if b.providers[0]["id"] != "openrouter" {
		t.Fatalf("%v", b.providers[0])
	}
	models := b.providers[0]["models"].([]map[string]any)
	if len(models) != 2 || models[1]["id"] != "stealth/ox-alpha" {
		t.Fatalf("%v", models)
	}
	think := models[0]["thinking_options"].([]map[string]any)
	if len(think) < 5 {
		t.Fatalf("thinking %v", think)
	}
}
