package agent

import (
	"encoding/json"
	"strconv"
	"strings"

	"mindimprint/api/internal/cards"
)

// fallbackCard is an ordered struct used when no spec is found in the registry,
// preserving field order card_id, card_name, status in the JSON output.
// (map[string]string does NOT preserve order in Go.)
type fallbackCard struct {
	CardID   string `json:"card_id"`
	CardName string `json:"card_name"`
	Status   string `json:"status"`
}

// AssembleEvalInput ports the TS assembleEvalInput verbatim: a "## 对话"
// transcript (学生/陪练 lines, 陪练（建议工具卡） when a summon_card rode along)
// followed by a "## 工具卡" dump (per card: display name + status label, the
// serialized refeed payload for completed/skipped, and the event-count line).
func AssembleEvalInput(messages []StoredMessage, cardInsts []CardInstance, specByID func(string) (cards.Spec, bool)) string {
	var parts []string

	// ── Section 1: Conversation transcript ──────────────────────────────────

	parts = append(parts, "## 对话")
	parts = append(parts, "")

	for _, m := range messages {
		switch m.Role {
		case "user":
			parts = append(parts, "学生："+m.Content)
		case "assistant":
			if m.ToolCall != nil && m.ToolCall.Name == "summon_card" {
				parts = append(parts, "陪练（建议工具卡）："+m.Content)
			} else {
				parts = append(parts, "陪练："+m.Content)
			}
		}
		// system and tool messages are skipped in the eval transcript
	}

	// ── Section 2: Tool card envelopes ──────────────────────────────────────

	parts = append(parts, "")
	parts = append(parts, "## 工具卡")

	if len(cardInsts) == 0 {
		parts = append(parts, "")
		parts = append(parts, "（本次对话未触发工具卡）")
	}

	for _, c := range cardInsts {
		spec, hasSpec := specByID(c.CardID)
		displayName := c.CardID
		if hasSpec {
			displayName = spec.Name
		}
		statusLabel := c.Status
		if c.Status == "skipped" {
			statusLabel = "跳过"
		}

		parts = append(parts, "")
		parts = append(parts, "【"+displayName+"】 "+statusLabel)

		if c.Status == "completed" || c.Status == "skipped" {
			if hasSpec {
				b, _ := json.Marshal(SerializeCardForRefeed(spec, c))
				parts = append(parts, string(b))
			} else {
				// No spec in registry — emit a minimal envelope noting the status
				if c.Status == "skipped" {
					parts = append(parts, "（跳过，无规格信息）")
				}
				b, _ := json.Marshal(fallbackCard{CardID: c.CardID, CardName: c.CardID, Status: c.Status})
				parts = append(parts, string(b))
			}
		}

		parts = append(parts, "（共 "+strconv.Itoa(c.EventTraceLen)+" 个操作事件）")
	}

	return strings.Join(parts, "\n")
}
