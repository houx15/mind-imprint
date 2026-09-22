package api

import (
	"fmt"
	"strings"
	"testing"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/claritytest"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/prompts"
	"mindimprint/api/internal/store/sqlc"
)

// This opt-in case follows the production writing-turn context builders and
// reply enforcement. The final prose still needs human review for ghostwriting.
func TestClarityWritingRoleBoard(t *testing.T) {
	wr := sqlc.Writing{Title: "学校是否应该保留午间自由活动", Lang: "zh", Stage: "writing"}
	rows := []sqlc.WritingOutline{
		planRow("a", 0, "学校应该保留午间自由活动", writingKindThesis),
		planRow("b", 1, "自主安排有助于适应不同需求", writingKindPoint),
		planRow("c", 2, "同学在午间选择不同的活动", writingKindEvidence),
	}
	draft := "午间自由活动能让同学按照自己的需要安排时间。上周三，小林去操场踢球，小周留在教室看书，我在走廊和朋友聊天。"
	// Same message shape as composeRoleBoardAnswer, with already classified sentences.
	said := "我给「自主安排有助于适应不同需求」这一段的每一句标了它在干什么：\n1 主张 —— 午间自由活动能让同学按照自己的需要安排时间。\n2 证据 —— 上周三，小林去操场踢球，小周留在教室看书，我在走廊和朋友聊天。\n这一段里没有：解释、让步、背景"
	projection := buildWritingCoachProjection(wr, rows, nil, draft) + writingBoardNote("role")
	req := gateway.ChatRequest{Messages: []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: prompts.ProjectCoachPosturePrompt},
		{Role: gateway.RoleUser, Content: agent.BuildProjectCoachContext(buildWritingTurnHistory(nil, said), projection, writingStageLabel(wr.Stage))},
	}}
	claritytest.Run(t, gateway.ClassDialogue, req, func(raw string) error {
		if err := enforcement.ValidateOutput(enforcement.AgentOutput{Type: "reply", Body: strings.TrimSpace(raw)}); err != nil {
			return err
		}
		if rule := enforcement.BannedPhrasing(raw); rule != nil {
			return fmt.Errorf("production banned-phrasing check: %s", rule.Name)
		}
		if !strings.Contains(raw, "小林") && !strings.Contains(raw, "踢球") && !strings.Contains(raw, "按照自己的需要") && !strings.Contains(raw, "看书") {
			return fmt.Errorf("feedback does not refer to the submitted paragraph")
		}
		for _, phrase := range []string{"请重新拆分", "请先拆分", "请重新标注", "请先标注", "我帮你改写", "可以改成", "可以写成", "帮你写一段"} {
			if strings.Contains(raw, phrase) {
				return fmt.Errorf("repeats completed work or offers body text: %s", phrase)
			}
		}
		return checkClarityTeachingLanguage(raw)
	})
}
