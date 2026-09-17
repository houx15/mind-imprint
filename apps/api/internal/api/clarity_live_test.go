package api

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/claritytest"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

func TestClarityReading(t *testing.T) {
	cases := []struct {
		name, kind, label, text, advance string
		picks                            []readingPick
	}{
		{"concept", "focus_block", "比较装机容量和发电量，说明它们能否直接用于同一种判断", "装机容量和发电量这两个词是什么意思？", "", nil},
		{"help", "focus_block", "找出一个关键数字并说明它衡量什么", "我不会，请给一个提示。", "", nil},
		{"partial", "focus_block", "找出一个关键数字并说明它衡量什么", "我选第三段连续八年第一。", "", nil},
		{"done", "focus_block", "找出一个关键数字并说明它衡量什么", "我选2023年新增太阳能装机超过一半在中国。这是中国在全球新增太阳能发电能力中的占比，不是实际发电量的占比。", "done", nil},
		{"skip", "focus_block", "找出一个关键数字并说明它衡量什么", "请跳过这一步。", "skipped", nil},
		{"read-done", "read", "通读全文", "我已经读完了。", "done", nil},
		{"connect", "connect", "联系自己的经历", "我家去年装了太阳能板，阴天和晴天发的电差别很大。", "done", nil},
		{"hunt-no-pick", "hunt", "请在文章里点出最能支持作者观点的一句", "我觉得第三段不错，但我还没有点击或划线。", "", nil},
		{"hunt-picked", "hunt", "请在文章里点出最能支持作者观点的一句", "我选了这句，因为它提供了一个长期变化的指标。", "done", []readingPick{{BlockID: "b3", Quote: "中国的可再生能源新增装机量连续八年位居世界第一。"}}},
		{"hunt-skip", "hunt", "请在文章里点出最能支持作者观点的一句", "我想跳过，不选了。", "skipped", nil},
	}
	for _, input := range cases {
		t.Run(input.name, func(t *testing.T) {
			blocks := SplitBlocks(benchReadingArticle)
			tasks := []sqlc.ReadingTask{{ID: fixtureTaskID(1), Position: 1, Kind: input.kind, Label: input.label, BlockID: "b3", Status: "pending"}, {ID: fixtureTaskID(2), Position: 2, Kind: "reflect", Label: "总结读后的观点变化", Status: "pending"}}
			history := []sqlc.AtomMessage{{Seq: 1, Role: "ai", Content: "当前任务：" + input.label + "。"}}
			req := gateway.ChatRequest{Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: buildReadingCoachSystem(readingLangOf(benchReadingArticle))}, {Role: gateway.RoleUser, Content: buildReadingCoachPrompt("中国的能源转型", blocks, readingOutline{}, tasks, history, input.picks, input.text, nil, "")}}}
			claritytest.Run(t, gateway.ClassDialogue, req, func(raw string) error {
				out, ok := parseReadingCoachReply(raw, blocks, readingLangOf(benchReadingArticle), func(string) bool { return true })
				if !ok {
					return errors.New("reading parse failed")
				}
				if out.Advance != input.advance {
					return fmt.Errorf("advance = %q, want %q", out.Advance, input.advance)
				}
				return nil
			})
		})
	}
}

// Keep completion refeed distinct from a student's claim to have used a lens.
// Check the raw parsed decision before the handler's lens guard can correct it.
func TestClarityReadingLens(t *testing.T) {
	for _, c := range []struct {
		name, text, advance string
		done                *readingLensDone
	}{
		{"not-completed", "我还没做这个透镜，应该怎么开始？", "", nil},
		{"completed", "", "done", &readingLensDone{CardName: "信源评估", Quote: "中国的可再生能源新增装机量连续八年位居世界第一。", Finding: "这句话提供了连续多年的比较结果，但还需核对统计出处。"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			blocks := SplitBlocks(benchReadingArticle)
			tasks := []sqlc.ReadingTask{{ID: fixtureTaskID(1), Position: 1, Kind: "lens", Label: "用信源评估透镜分析一条证据", BlockID: "b3", Status: "pending"}, {ID: fixtureTaskID(2), Position: 2, Kind: "reflect", Label: "总结读后的观点变化", Status: "pending"}}
			history := []sqlc.AtomMessage{{Seq: 1, Role: "ai", Content: "请用信源评估透镜分析文章中的一条证据。"}}
			req := gateway.ChatRequest{Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: buildReadingCoachSystem(readingLangOf(benchReadingArticle))}, {Role: gateway.RoleUser, Content: buildReadingCoachPrompt("中国的能源转型", blocks, readingOutline{}, tasks, history, nil, c.text, c.done, "")}}}
			claritytest.Run(t, gateway.ClassDialogue, req, func(raw string) error {
				out, ok := parseReadingCoachReply(raw, blocks, readingLangOf(benchReadingArticle), func(string) bool { return true })
				if !ok {
					return errors.New("reading lens parse failed")
				}
				if out.Advance != c.advance {
					return fmt.Errorf("advance = %q, want %q", out.Advance, c.advance)
				}
				return nil
			})
		})
	}
}

func TestClarityWriting(t *testing.T) {
	wr := sqlc.Writing{Title: "学校图书馆是否应延长开放时间", Lang: "zh"}
	block := sqlc.WritingOutline{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Text: "晚自习后需要安静的自习场所", Role: "一条理由"}
	text := "学校图书馆应该延长开放时间。晚自习后教室关闭，住校生缺少安静的自习场所。上周我和三位同学因此去走廊复习，但走廊里一直有人经过。"
	t.Run("plan", func(t *testing.T) {
		claritytest.Run(t, gateway.ClassDialogue, gateway.ChatRequest{MaxTokens: 4096, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: fmt.Sprintf(writingPlanSystem, writingPlanMaxNewNodes)}, {Role: gateway.RoleUser, Content: buildWritingPlanPrompt(wr, nil, nil, "我想主张延长开放。晚自习后教室关门，上周我和三位同学只能在走廊复习。请帮我整理这些想法。")}}}, func(raw string) error {
			_, ok := parseWritingPlanReply(raw)
			if !ok {
				return errors.New("plan parse failed")
			}
			return nil
		})
	})
	t.Run("guide", func(t *testing.T) {
		claritytest.Run(t, gateway.ClassCompose, gateway.ChatRequest{MaxTokens: 4096, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: writingGuideSystem}, {Role: gateway.RoleUser, Content: buildWritingGuidePrompt(wr, block, nil, "", nil)}}}, func(raw string) error {
			_, ok := parseWritingGuide(raw)
			if !ok {
				return errors.New("guide parse failed")
			}
			return nil
		})
	})
	t.Run("comment", func(t *testing.T) {
		claritytest.Run(t, gateway.ClassReview, gateway.ChatRequest{MaxTokens: 4096, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: buildWritingCommentSystem(wr.Lang, writingBlockCommentMaxIssues)}, {Role: gateway.RoleUser, Content: buildWritingCommentPrompt(wr, "她写的这一段", text)}}}, func(raw string) error {
			out, ok := parseWritingComment(raw)
			if !ok {
				return errors.New("comment parse failed")
			}
			for _, p := range out.Points {
				if !strings.Contains(text, p.Quote) {
					return errors.New("comment quote not in original")
				}
			}
			return nil
		})
	})
}
