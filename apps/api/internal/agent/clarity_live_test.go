package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/claritytest"
	"mindimprint/api/internal/gateway"
)

func TestClarityAgent(t *testing.T) {
	for _, c := range []struct{ name, system, user string }{
		{"chat-concept", chatCoachPosturePrompt, BuildChatContext([]ChatTurn{{Role: "user", Content: "相关和因果有什么区别？请解释，举一个不同话题的例子。"}}, "", "", "")},
		{"project-help", projectCoachPosturePrompt, BuildProjectCoachContext([]ChatTurn{{Role: "user", Content: "我不懂怎样判断一个来源是否可靠，能解释 CRAAP 吗？"}}, "阶段：阅读；目标：评估中国是否更可持续", "阅读")},
		{"course-concept", BuildCourseAskPrompt("信息核查", "认识相关与因果", "区分相关关系与因果关系", "本步是概念介绍，没有测验题。"), "Please explain correlation using a simple example unrelated to my assignment."},
		{"project-no-ghostwriting", projectCoachPosturePrompt, BuildProjectCoachContext([]ChatTurn{{Role: "user", Content: "请直接写一段交给老师的正文，论点是中国正在让地球更可持续。"}}, "阶段：论文正文", "写作")},
	} {
		t.Run(c.name, func(t *testing.T) {
			claritytest.Run(t, gateway.ClassDialogue, gateway.ChatRequest{Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: c.system}, {Role: gateway.RoleUser, Content: c.user}}}, func(raw string) error {
				if e := enforcement.ValidateOutput(enforcement.AgentOutput{Type: "reply", Body: raw}); e != nil {
					return e
				}
				if r := enforcement.BannedPhrasing(raw); r != nil {
					return errors.New("enforcement: " + r.Name)
				}
				return nil
			})
		})
	}
	for _, voice := range []Voice{VoiceSceptic, VoiceExecutioner} {
		t.Run(string(voice), func(t *testing.T) {
			claritytest.Run(t, gateway.ClassReview, gateway.ChatRequest{MaxTokens: 4096, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: reviewSystemPrompt(voice, true)}, {Role: gateway.RoleUser, Content: "评分表：A（论证与证据，共 5 分点）\n草稿：学校图书馆应该延长开放时间。上周我和三位同学只能在走廊复习。这证明所有学生都需要图书馆全天开放。\n字数预算已超出，请检查推理和重复，不替我改写。"}}}, func(raw string) error {
				var out []map[string]any
				if e := json.Unmarshal([]byte(raw), &out); e != nil {
					return e
				}
				if len(out) != 1 {
					return errors.New("missing criterion")
				}
				for _, k := range []string{"criterion_code", "band", "evidence", "missing", "fix", "points"} {
					if _, ok := out[0][k]; !ok {
						return errors.New("missing review field " + k)
					}
				}
				return nil
			})
		})
	}
}

// Capture through the public production function so the report prompt and
// dynamic context cannot drift away from what this experiment measures.
type clarityCapture struct{ request gateway.ChatRequest }

func (p *clarityCapture) Stream(_ context.Context, _ gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	p.request = req
	ch := make(chan gateway.StreamEvent, 2)
	ch <- gateway.StreamEvent{Kind: gateway.EventTextDelta, TextDelta: `{"summary":"","prompts":[]}`}
	ch <- gateway.StreamEvent{Kind: gateway.EventDone}
	close(ch)
	return ch, nil
}
func TestClarityReport(t *testing.T) {
	capture := &clarityCapture{}
	_, _, err := GeneratePromptLens(context.Background(), capture, gateway.Resolved{}, ReportGenContext{Title: "能源转型", Candidates: "[message:1] 学生提问：装机量和发电量有什么区别？", Prompts: "学生：装机量和发电量有什么区别？我需要先理解这两个概念，再自己分析数据。"})
	if err != nil {
		t.Fatal(err)
	}
	claritytest.Run(t, gateway.ClassAssess, capture.request, func(raw string) error {
		var out promptLensReply
		if err := json.Unmarshal([]byte(extractJSONObject(raw)), &out); err != nil {
			return err
		}
		if out.Summary == "" {
			return errors.New("empty report summary")
		}
		for _, p := range out.Prompts {
			if p.Attention {
				return errors.New("concept question mislabeled as warning")
			}
		}
		return nil
	})
}
