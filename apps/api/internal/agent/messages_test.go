package agent

import (
	"encoding/json"
	"testing"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
)

func TestBuildLlmMessagesUserAndPlainAssistant(t *testing.T) {
	msgs := BuildLlmMessages(BuildLlmMessagesOptions{
		SystemPrompt: "SYS",
		Messages: []StoredMessage{
			{Role: "user", Content: "你好"},
			{Role: "assistant", Content: "我们开始吧"},
		},
		CardByID: func(string) (CardInstance, bool) { return CardInstance{}, false },
		SpecByID: func(string) (cards.Spec, bool) { return cards.Spec{}, false },
	})
	if len(msgs) != 3 {
		t.Fatalf("len = %d, want 3 (system+user+assistant)", len(msgs))
	}
	if msgs[0].Role != gateway.RoleSystem || msgs[0].Content != "SYS" {
		t.Fatalf("system message wrong")
	}
	if msgs[1].Role != gateway.RoleUser || msgs[1].Content != "你好" {
		t.Fatalf("user message wrong")
	}
	if msgs[2].Role != gateway.RoleAssistant || len(msgs[2].ToolCalls) != 0 {
		t.Fatalf("plain assistant should carry no toolCalls")
	}
}

func TestBuildLlmMessagesResolvedCardEmitsToolUsePlusResult(t *testing.T) {
	spec, _ := cards.ByID("sift_craap")
	call := &SummonCardCall{
		ID:             "tc_1",
		Name:           "summon_card",
		Args:           SummonCardArgs{CardID: "sift_craap", Reason: "r", NudgeText: "n"},
		CardInstanceID: "ci_1",
	}
	ci := CardInstance{ID: "ci_1", CardID: "sift_craap", Status: "completed", FieldValues: map[string]map[string]any{"sift": {"stop": "x"}}}
	msgs := BuildLlmMessages(BuildLlmMessagesOptions{
		SystemPrompt: "SYS",
		Messages:     []StoredMessage{{Role: "assistant", Content: "看看这张卡", ToolCall: call}},
		CardByID:     func(id string) (CardInstance, bool) { return ci, id == "ci_1" },
		SpecByID:     func(id string) (cards.Spec, bool) { return spec, id == "sift_craap" },
	})
	// system + assistant(tool_use) + tool(result)
	if len(msgs) != 3 {
		t.Fatalf("len = %d, want 3", len(msgs))
	}
	asst := msgs[1]
	if asst.Role != gateway.RoleAssistant || len(asst.ToolCalls) != 1 || asst.ToolCalls[0].ID != "tc_1" {
		t.Fatalf("assistant tool_use wrong: %+v", asst)
	}
	tool := msgs[2]
	if tool.Role != gateway.RoleTool || tool.ToolCallID != "tc_1" {
		t.Fatalf("tool result wrong: %+v", tool)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(tool.Content), &payload); err != nil {
		t.Fatalf("tool content not JSON: %v", err)
	}
	if payload["card_id"] != "sift_craap" || payload["status"] != "completed" {
		t.Fatalf("refeed payload wrong: %v", payload)
	}
}

func TestBuildLlmMessagesUnresolvedCardFallsBackToText(t *testing.T) {
	call := &SummonCardCall{
		ID:             "tc_1",
		Name:           "summon_card",
		Args:           SummonCardArgs{CardID: "sift_craap", Reason: "r", NudgeText: "要不要试试这张卡？"},
		CardInstanceID: "ci_1",
	}
	ci := CardInstance{ID: "ci_1", CardID: "sift_craap", Status: "proposed"}
	// content empty → should fall back to nudge_text
	msgs := BuildLlmMessages(BuildLlmMessagesOptions{
		SystemPrompt: "SYS",
		Messages:     []StoredMessage{{Role: "assistant", Content: "", ToolCall: call}},
		CardByID:     func(id string) (CardInstance, bool) { return ci, id == "ci_1" },
		SpecByID:     func(string) (cards.Spec, bool) { return cards.Spec{}, false },
	})
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2 (system+assistant text)", len(msgs))
	}
	if len(msgs[1].ToolCalls) != 0 {
		t.Fatalf("unresolved proposal must not emit toolCalls")
	}
	if msgs[1].Content != "要不要试试这张卡？" {
		t.Fatalf("content = %q, want nudge_text fallback", msgs[1].Content)
	}
}
