package pbl

import (
	"fmt"
	"strings"
	"testing"

	"mindimprint/api/internal/claritytest"
	"mindimprint/api/internal/gateway"
)

func TestClarityPBL(t *testing.T) {
	cases := []struct {
		name string
		in   CoachInput
		kind string
	}{
		{"examples", CoachInput{Idea: "做自己的个人主页", Kind: "website", Stuck: 3, AskedForHelp: true, Recent: []Turn{{Role: "ai", Content: "你想让谁看？"}, {Role: "student", Content: "不知道，能给几个例子吗？"}}}, ""},
		{"plan", CoachInput{Idea: "调查食堂浪费", Recent: []Turn{{Role: "student", Content: "目标是比较小份菜推出前后每天剩饭的重量。我有两周，每天午餐结束称重，第一周记录现状，第二周试小份菜。我已经问过食堂老师，可以配合。别再问了，请根据这些信息生成计划。"}}}, "plan"},
		{"split", CoachInput{Idea: "食堂剩饭调查", Steps: []string{"记录每日剩饭重量并分析变化"}, Recent: []Turn{{Role: "student", Content: "帮我把记录每日剩饭重量并分析变化这一步分工。我负责去食堂称重，你负责给出记录表结构和分析方法。请打开分工建议。"}}}, "substeps"},
		{"observe", CoachInput{Idea: "了解学校食堂剩饭的情况", Recent: []Turn{{Role: "student", Content: "我还没观察过食堂。请打开观察日记，给我一份午餐时可以执行的观察清单。"}}}, ""},
		{"course", CoachInput{Idea: "调查食堂浪费", Courses: []CourseOption{{Slug: "interview-basics", Title: "访谈入门", Blurb: "学习设计中性访谈问题并记录原话", TimeLabel: "15分钟"}}, Recent: []Turn{{Role: "student", Content: "我不知道访谈该怎么提问。请让我学习课程库中的访谈入门这门课。"}}}, "course"},
		{"concept", CoachInput{Idea: "研究太阳能发电", Recent: []Turn{{Role: "student", Content: "装机容量和发电量有什么区别？我只是想知道这两个词的意思。"}}}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			claritytest.Run(t, gateway.ClassDialogue, gateway.ChatRequest{MaxTokens: 16384, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: coachPrompt(c.in.Kind)}, {Role: gateway.RoleUser, Content: buildCoachContext(c.in)}}}, func(raw string) error {
				out, e := parseCoachOutput(raw)
				if e != nil {
					return e
				}
				if c.kind != "" && (out.Produce == nil || out.Produce.Kind != c.kind) {
					return fmt.Errorf("missing requested %s", c.kind)
				}
				if strings.Contains(out.Reply, "tool_reason") || strings.Contains(out.Reply, "hook_kind") {
					return fmt.Errorf("internal field leaked into visible reply")
				}
				if c.name == "observe" {
					if out.Tool != "observe" || len(out.Mission) < 3 || len(out.Mission) > 5 {
						return fmt.Errorf("missing observation checklist")
					}
				}
				if out.Tool != "" {
					if tool, ok := LookupTool(out.Tool); !ok {
						return fmt.Errorf("unknown tool")
					} else if tool.Needs != "" && (out.Produce == nil || out.Produce.Kind != tool.Needs) {
						return fmt.Errorf("tool/produce mismatch")
					}
				}
				return nil
			})
		})
	}
}

func TestClarityLookback(t *testing.T) {
	in := benchLookbackInput()
	claritytest.Run(t, gateway.ClassAssess, gateway.ChatRequest{MaxTokens: 16384, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: fmt.Sprintf(lookbackSystem, lookbackSectionList(), buildLookbackContext(in))}}}, func(raw string) error { _, err := parseLookback(raw); return err })
}
