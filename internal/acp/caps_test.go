package acp

import (
	"encoding/json"
	"testing"
)

func TestCapsFromOpenCodeConfigOptions(t *testing.T) {
	sess := &SessionNewResult{
		SessionID: "s1",
		ConfigOptions: []SessionConfigOption{{
			ID: "model", Type: "select", CurrentValue: "big-pickle",
			Options: []struct {
				Value string `json:"value"`
				Name  string `json:"name"`
			}{{Value: "big-pickle", Name: "Big Pickle"}, {Value: "other", Name: "Other"}},
		}},
	}
	got := capsFrom(sess)
	ms := got["models"].([]map[string]any)
	if len(ms) != 2 || ms[0]["id"] != "big-pickle" || ms[0]["is_default"] != true {
		t.Fatalf("opencode models: %#v", ms)
	}
}

func TestCapsFromGrokModels(t *testing.T) {
	sess := &SessionNewResult{
		SessionID: "s1",
		Models: &SessionModels{
			CurrentModelID: "grok-4.6",
			AvailableModels: []struct {
				ModelID string `json:"modelId"`
				Name    string `json:"name"`
			}{{ModelID: "grok-4.6", Name: "Grok 4.6"}, {ModelID: "grok-4.5", Name: "Grok 4.5"}},
		},
		Meta: json.RawMessage(`{"x.ai/sessionConfig":{"options":[
			{"id":"low","category":"mode"},
			{"id":"high","category":"mode","name":"High"}
		]}}`),
	}
	got := capsFrom(sess)
	ms := got["models"].([]map[string]any)
	if len(ms) != 2 || ms[0]["id"] != "grok-4.6" || ms[0]["is_default"] != true {
		t.Fatalf("grok models: %#v", ms)
	}
	think, _ := ms[0]["thinking_options"].([]map[string]any)
	if len(think) != 2 || think[0]["id"] != "low" || think[1]["label"] != "High" {
		t.Fatalf("grok thinking: %#v", think)
	}
}

func TestIsMethodMissing(t *testing.T) {
	if !isMethodMissing(json.RawMessage(`{"code":-32601,"message":"Method not found"}`)) {
		t.Fatal("expected missing")
	}
	if isMethodMissing(json.RawMessage(`{"code":-32602,"message":"Invalid params"}`)) {
		t.Fatal("invalid params is not missing")
	}
	if isMethodMissing(nil) {
		t.Fatal("nil is not missing")
	}
}
