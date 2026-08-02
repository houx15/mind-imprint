package gateway

import (
	"encoding/json"
	"testing"
)

func TestChatMessageJSONShape(t *testing.T) {
	m := ChatMessage{
		Role:    RoleAssistant,
		Content: "hi",
		ToolCalls: []ToolCall{
			{ID: "tc_1", Name: "summon_card", Args: map[string]any{"card_id": "craap"}},
		},
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["role"] != "assistant" {
		t.Fatalf("role = %v, want assistant", got["role"])
	}
	if _, ok := got["toolCalls"]; !ok {
		t.Fatalf("toolCalls missing")
	}
}

func TestRoleAndStopConstants(t *testing.T) {
	if RoleSystem != "system" || RoleUser != "user" || RoleAssistant != "assistant" || RoleTool != "tool" {
		t.Fatal("role constants drifted")
	}
	if StopStop != "stop" || StopToolCall != "tool_call" || StopLength != "length" || StopOther != "other" {
		t.Fatal("stop constants drifted")
	}
}
