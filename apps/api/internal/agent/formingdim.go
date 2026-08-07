package agent

import (
	"context"
	"encoding/json"
	"strings"

	"mindimprint/api/internal/gateway"
)

// FormingDimSuggestion is a 克制 confirm-chip offer: the coach believes the
// student just articulated one still-empty kick-off dimension, and offers to
// record a concise, faithful summary of HER OWN WORDS into that dimension —
// which she confirms with a tap (打开由学生确认). The AI never writes the
// proposal on its own; this is a suggestion, not an autofill (#13).
type FormingDimSuggestion struct {
	Dim   string `json:"dim"`   // objective | reason | activities | resources | counterpoints
	Value string `json:"value"` // one-line summary in the student's own words
}

var formingDimLabels = map[string]string{
	"objective":     "目标",
	"reason":        "缘由",
	"activities":    "活动",
	"resources":     "资源",
	"counterpoints": "反例/张力",
}

const formingDimSystemPrompt = `你在帮一个学生立题。提案要点有五栏：目标 objective / 缘由 reason / 活动 activities / 资源 resources / 反例·张力 counterpoints（她的论点可能撞上的反例、张力或反方证据）。
判断学生刚说的这句话，是否已经清楚地表达了其中【某一个还没填】的栏目。
- 如果是：输出 JSON {"dim":"<objective|reason|activities|resources|counterpoints 之一>","value":"<用学生自己的话，一句话概括要记进那一栏的内容>"}。value 必须忠于她的原话，别替她扩写、别下结论、别润色成不是她说的意思。
- 如果她这句话没有清楚落到任何一个还没填的栏目：输出 {"dim":""}。
只输出这个 JSON，不要多余文字。`

func stripJSONFence(s string) string {
	c := strings.TrimSpace(s)
	if strings.HasPrefix(c, "```json") {
		c = strings.TrimLeft(strings.TrimPrefix(c, "```json"), " \t\r\n")
	} else if strings.HasPrefix(c, "```") {
		c = strings.TrimLeft(c[3:], " \t\r\n")
	}
	if strings.HasSuffix(c, "```") {
		c = strings.TrimRight(c[:len(c)-3], " \t\r\n")
	}
	return strings.TrimSpace(c)
}

// ProposeFormingDim runs ONE cheap classification: did the student's latest
// message articulate one of the still-empty kick-off dimensions? Returns a
// suggestion ONLY for a dimension in `uncovered` (never one already filled),
// with a non-empty value. No spend when there's nothing to classify.
func ProposeFormingDim(ctx context.Context, prov gateway.Provider, r gateway.Resolved, studentText string, uncovered []string) (*FormingDimSuggestion, gateway.ChatUsage, error) {
	if len([]rune(strings.TrimSpace(studentText))) < MinClassifyRunes || len(uncovered) == 0 {
		return nil, gateway.ChatUsage{}, nil // nothing to classify → no spend
	}
	uncoveredSet := map[string]bool{}
	names := make([]string, 0, len(uncovered))
	for _, d := range uncovered {
		uncoveredSet[d] = true
		if label := formingDimLabels[d]; label != "" {
			names = append(names, label+"("+d+")")
		}
	}
	user := "还没填的维度：" + strings.Join(names, "、") + "\n\n学生刚说：" + strings.TrimSpace(studentText)
	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: formingDimSystemPrompt},
			{Role: gateway.RoleUser, Content: user},
		},
	})
	if err != nil {
		return nil, gateway.ChatUsage{}, err
	}
	usage := res.Usage

	var sug FormingDimSuggestion
	if jerr := json.Unmarshal([]byte(stripJSONFence(res.Text)), &sug); jerr != nil {
		return nil, usage, nil // unparseable → no chip, never crash the turn
	}
	sug.Dim = strings.TrimSpace(sug.Dim)
	sug.Value = strings.TrimSpace(sug.Value)
	// Only offer a dimension that is genuinely still empty, with a real value.
	if sug.Dim == "" || sug.Value == "" || !uncoveredSet[sug.Dim] {
		return nil, usage, nil
	}
	return &sug, usage, nil
}
