package agent

import (
	"encoding/json"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
)

// SummonCardArgs mirrors the TS SummonCardArgs (the model-supplied tool input).
type SummonCardArgs struct {
	CardID    string `json:"card_id"`
	Reason    string `json:"reason"`
	NudgeText string `json:"nudge_text"`
}

// SummonCardCall mirrors the TS SummonCardCall: the persisted tool_call shape on
// an assistant message, linking the call to its created card_instance.
type SummonCardCall struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Args           SummonCardArgs `json:"args"`
	CardInstanceID string         `json:"card_instance_id"`
}

// StoredMessage is the agent-layer view of one messages row (decoded tool_call).
type StoredMessage struct {
	Role     string
	Content  string
	ToolCall *SummonCardCall
}

// BuildLlmMessagesOptions are the inputs to BuildLlmMessages.
type BuildLlmMessagesOptions struct {
	SystemPrompt string
	Messages     []StoredMessage
	CardByID     func(id string) (CardInstance, bool)
	SpecByID     func(cardID string) (cards.Spec, bool)
}

// BuildLlmMessages replays the stored transcript into the ChatMessage[] sent to
// the model, pairing each RESOLVED card proposal's tool_use with its tool_result
// (the serialized refeed), and degrading UNRESOLVED proposals to plain coaching
// text. Ports the TS buildLlmMessages verbatim.
func BuildLlmMessages(opts BuildLlmMessagesOptions) []gateway.ChatMessage {
	result := []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: opts.SystemPrompt}}

	for _, m := range opts.Messages {
		switch m.Role {
		case "user":
			result = append(result, gateway.ChatMessage{Role: gateway.RoleUser, Content: m.Content})
		case "assistant":
			call := m.ToolCall
			if call != nil && call.Name == "summon_card" {
				ci, ok := opts.CardByID(call.CardInstanceID)
				if ok && (ci.Status == "completed" || ci.Status == "skipped") {
					// RESOLVED: tool_use + paired tool_result.
					result = append(result, gateway.ChatMessage{
						Role:    gateway.RoleAssistant,
						Content: m.Content,
						ToolCalls: []gateway.ToolCall{{
							ID:   call.ID,
							Name: "summon_card",
							Args: map[string]any{
								"card_id":    call.Args.CardID,
								"reason":     call.Args.Reason,
								"nudge_text": call.Args.NudgeText,
							},
						}},
					})
					spec, _ := opts.SpecByID(ci.CardID)
					payload, _ := json.Marshal(SerializeCardForRefeed(spec, ci))
					result = append(result, gateway.ChatMessage{
						Role:       gateway.RoleTool,
						Content:    string(payload),
						ToolCallID: call.ID,
					})
				} else {
					// UNRESOLVED (proposed/active): plain coaching text.
					content := m.Content
					if content == "" {
						content = call.Args.NudgeText
					}
					result = append(result, gateway.ChatMessage{Role: gateway.RoleAssistant, Content: content})
				}
			} else {
				result = append(result, gateway.ChatMessage{Role: gateway.RoleAssistant, Content: m.Content})
			}
		case "system":
			result = append(result, gateway.ChatMessage{Role: gateway.RoleSystem, Content: m.Content})
		}
	}
	return result
}
