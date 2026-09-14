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
	for _, input := range []struct{ name, text, advance string }{{"concept", "装机容量和发电量是什么意思？请解释区别。", ""}, {"help", "我还是不懂，可以给我看一个完整示范吗？", ""}, {"exercise", "我选装机量连续八年第一，说明投入很大。", ""}, {"done", "我选2023年全球新增太阳能发电装机中超过一半在中国。这衡量的是新增的发电能力，不能直接说明实际发电量或替代了多少化石燃料。", "done"}, {"skip", "请跳过这一步。", "skipped"}} {
		t.Run(input.name, func(t *testing.T) {
			c := readingCoachCase()
			c.Request.Messages[1].Content = buildReadingCoachPrompt("中国的能源转型", SplitBlocks(benchReadingArticle), readingOutline{}, []sqlc.ReadingTask{{Position: 1, Kind: "label", Label: "找出关键数字并说明它衡量什么", Status: "pending"}}, nil, nil, input.text, nil)
			claritytest.Run(t, c.Class, c.Request, func(raw string) error {
				out, ok := parseReadingCoachReply(raw, SplitBlocks(benchReadingArticle), readingLangOf(benchReadingArticle), func(string) bool { return true })
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
		claritytest.Run(t, gateway.ClassReview, gateway.ChatRequest{MaxTokens: 4096, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: writingCommentSystem}, {Role: gateway.RoleUser, Content: buildWritingCommentPrompt(wr, "她写的这一段", text)}}}, func(raw string) error {
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
