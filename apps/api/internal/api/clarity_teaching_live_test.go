package api

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"mindimprint/api/internal/claritytest"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// An experiment check, not a production word filter. Full replies also need
// human review for teaching quality, accuracy and continuity with the student.
func checkClarityTeachingLanguage(s string) error {
	for _, phrase := range []string{"撑", "立靶子", "站得住", "立住", "最狠", "直接猜", "落点", "白写", "只剩下", "只有这一条路", "哪一边赢", "主张"} {
		if strings.Contains(s, phrase) {
			return fmt.Errorf("compressed or confrontational teaching language: %s", phrase)
		}
	}
	if regexp.MustCompile(`\bb[0-9]+\b`).MatchString(s) {
		return errors.New("internal paragraph id in visible text")
	}
	return nil
}

func TestClarityTeacherReading(t *testing.T) {
	for _, c := range []struct {
		name, kind, label, input string
		mustAsk                  bool
	}{
		{"compare-not-verdict", "focus_block", "比较装机容量和发电量，说明它们能否直接用于同一种判断", "新增装机超过一半在中国，所以中国的实际发电量也超过全球一半吧？", true},
		{"explain-concept", "focus_block", "比较装机容量和发电量", "我不知道装机容量是什么意思，请先解释一下这个词。", false},
		{"predict-opening", "predict", "根据标题预测文章讨论的问题", "", true},
		{"direct-answer", "focus_block", "比较装机容量和发电量", "请直接告诉我答案：装机容量是不是实际发电量？", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			blocks := SplitBlocks(benchReadingArticle)
			tasks := []sqlc.ReadingTask{{ID: fixtureTaskID(1), Position: 1, Kind: c.kind, Label: c.label, BlockID: "b2", Status: "pending"}}
			req := gateway.ChatRequest{Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: buildReadingCoachSystem("zh")}, {Role: gateway.RoleUser, Content: buildReadingCoachPrompt("中国的能源转型", blocks, readingOutline{}, tasks, nil, nil, c.input, nil, "")}}}
			claritytest.Run(t, gateway.ClassDialogue, req, func(raw string) error {
				out, ok := parseReadingCoachReply(raw, blocks, "zh", func(string) bool { return true })
				if !ok {
					return errors.New("reading parse failed")
				}
				if out.Advance != "" {
					return errors.New("question or misconception incorrectly completes task")
				}
				visible := out.Reply
				if out.Card != nil {
					visible += out.Card.Prompt
				}
				if c.name == "compare-not-verdict" && (strings.Contains(visible, "两者不是") || strings.Contains(visible, "还要看设备") || strings.Contains(strings.ReplaceAll(visible, "能不能直接说明", ""), "不能直接说明")) {
					return errors.New("comparison prompt states the core answer before asking the student")
				}
				if c.mustAsk && !strings.ContainsAny(visible, "?？") && !strings.Contains(visible, "能不能") && !strings.Contains(visible, "是否") {
					return errors.New("guidance gives no question for the student to consider")
				}
				return checkClarityTeachingLanguage(visible)
			})
		})
	}
}

func TestClarityTeacherReview(t *testing.T) {
	cases := []struct{ name, lang, kind, genre, title, body string }{
		{"argument-scope", "zh", writingKindPoint, genreArgument, "学校图书馆是否应延长开放时间", "学校图书馆可以适当延长开放时间。上周晚自习结束后，教室关闭，我和三位同学只能在走廊复习。走廊里不断有人经过，我们很难集中注意力。"},
		{"narrative", "zh", writingKindScene, genreNarrative, "记一次难忘的经历", "那天晚上下着雨，我在楼道口等着。风越刮越大。后来我看见一个人走过来，是我爸。他给了我一件雨衣。我们一路上没说话。我很感动。"},
		{"english", "en", writingKindPoint, genreArgument, "Should our school library stay open later?", "Our school library should stay open for an extra hour. Last Tuesday, my classroom closed at seven, so I revised in the noisy corridor. A quiet library would help students like me concentrate. This may not suit everyone, so the school could try it once a week first."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wr := sqlc.Writing{Title: c.title, Lang: c.lang}
			req := gateway.ChatRequest{MaxTokens: 4096, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: buildWritingCommentSystem(c.lang, writingBlockCommentMaxIssues, c.kind, helpAsk, c.genre)}, {Role: gateway.RoleUser, Content: buildWritingCommentPrompt(wr, "她写的这一段", c.body, "", c.genre)}}}
			claritytest.Run(t, gateway.ClassReview, req, func(raw string) error {
				out, ok := parseWritingComment(raw)
				if !ok {
					return errors.New("review parse failed")
				}
				kept := validateCommentPoints(out.Points, c.body, c.lang, writingBlockCommentMaxIssues)
				if marker, _ := writingCommentProblem(out, kept); marker != "" {
					return fmt.Errorf("production review check: %s", marker)
				}
				visible := out.Summary
				for _, p := range kept {
					if p.Quote == "" || !strings.Contains(c.body, p.Quote) {
						return errors.New("review quote lost grounding")
					}
					visible += p.Text + p.Action
				}
				if c.genre == genreNarrative && strings.Contains(visible, "论点") {
					return errors.New("narrative reviewed as argument")
				}
				return checkClarityTeachingLanguage(visible)
			}, clarityCommentDelivery(t, c.body, c.lang))
		})
	}
}

func TestClarityReadingPlan(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		t.Run(lang, func(t *testing.T) {
			blocks := SplitBlocks(benchReadingArticle)
			title := "中国的能源转型"
			if lang == "en" {
				blocks = liveEnglishBlocks()
				title = "Is skipping breakfast a moral failure?"
			}
			claritytest.Run(t, gateway.ClassCompose, gateway.ChatRequest{Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: readingPlanSystem}, {Role: gateway.RoleUser, Content: buildReadingPlanPrompt(lang, title, blocks)}}}, func(raw string) error {
				plan, routine, reject := parseReadingPlan(raw, lang)
				if reject != planOK {
					return fmt.Errorf("plan rejected: %v", reject)
				}
				outline, ok := validateOutline(plan.outline(), blocks)
				if !ok {
					return errors.New("outline not usable")
				}
				positions, _, _, _, _ := buildReadingTasks(routine, plan, blocks, outline.Parts)
				if len(positions) == 0 {
					return errors.New("no reading steps")
				}
				if len(blocks) >= 4 && len(outline.Parts) == 0 {
					return errors.New("missing article parts")
				}
				visible := plan.OneLine + plan.Gist + plan.Shape
				for _, step := range plan.Steps {
					visible += step.Detail
				}
				for _, part := range plan.Parts {
					visible += part.Title + part.Does
				}
				return checkClarityTeachingLanguage(visible)
			})
		})
	}
}
