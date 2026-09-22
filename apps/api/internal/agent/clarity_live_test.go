package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/claritytest"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/skills"
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
			claritytest.Run(t, gateway.ClassReview, gateway.ChatRequest{MaxTokens: 4096, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: reviewSystemPrompt(voice, true)}, {Role: gateway.RoleUser, Content: "评分表：A（论证与证据，共 5 分点）\n\n论证摘要：\n\n草稿（分段）：学校图书馆应该延长开放时间。上周我和三位同学只能在走廊复习。这证明所有学生都需要图书馆全天开放。"}}}, func(raw string) error {
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
				for _, row := range out {
					for _, key := range []string{"band", "missing", "fix"} {
						if value, ok := row[key].(string); ok {
							for _, phrase := range []string{"主张", "撑", "站得住", "最狠", "直接猜", "落点"} {
								if strings.Contains(value, phrase) {
									return errors.New("review teaching language: " + phrase)
								}
							}
						}
					}
				}
				return nil
			}, func(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, _ gateway.ChatRequest) (gateway.ChatResult, error) {
				tapped := &clarityReviewProvider{real: prov, t: t}
				items, _, err := ProposeReview(ctx, tapped, resolved, []skills.ReviewCriterion{{Code: "A", Name: "论证与证据", Points: 5}}, []string{"学校图书馆应该延长开放时间。上周我和三位同学只能在走廊复习。这证明所有学生都需要图书馆全天开放。"}, "", voice, true)
				b, _ := json.Marshal(items)
				return gateway.ChatResult{Text: string(b), Usage: tapped.usage}, err
			})
		})
	}
}

type clarityReviewProvider struct {
	real  gateway.Provider
	usage gateway.ChatUsage
	t     *testing.T
}

func (p *clarityReviewProvider) Stream(ctx context.Context, r gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	res, err := gateway.Collect(ctx, p.real, r, req)
	p.usage.InputTokens += res.Usage.InputTokens
	p.usage.OutputTokens += res.Usage.OutputTokens
	p.usage.CachedInputTokens += res.Usage.CachedInputTokens
	p.t.Logf("production review attempt: %s", res.Text)
	if err != nil {
		return nil, err
	}
	return gateway.NewStubProvider([]gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: res.Text}, {Kind: gateway.EventUsage, Usage: &res.Usage}, {Kind: gateway.EventDone}}).Stream(ctx, r, req)
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
