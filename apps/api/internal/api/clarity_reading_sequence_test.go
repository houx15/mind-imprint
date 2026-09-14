package api

import (
	"fmt"
	"mindimprint/api/internal/claritytest"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
	"testing"
)

// These articles were not used to draft the current-step prompt. The student's
// next turn carries the model's actual previous reply, not a fabricated history.
func TestClarityReadingSequence(t *testing.T) {
	cases := []struct {
		name, article, task string
		students            []string
	}{
		{"library", "学校图书馆试行延长开放时间。\n\n试行四周后，每晚平均有32名学生在延长时段借阅或自习。这个数字记录的是实际到馆人数，不是全校赞成延长开放的比例。\n\n校方计划继续记录使用情况，再决定长期安排。", "找出第二段的数字并说明它统计的对象", []string{"我不知道怎么解释这个数字，给我一个提示。", "我选32这个数字。", "32统计的是试行期间每晚延长开放时段实际到馆的平均人数，并不代表全校学生赞成的比例。"}},
		{"urban-heat", "城区与郊区的气温并不总是相同。\n\n连续七个晴天的下午，观测点记录的城区平均气温比郊区高2摄氏度。这是两类地点在该时段的温差，不是城区过去一年的升温幅度。\n\n研究者提醒，天气与测量时段都会影响比较结果。", "找出第二段的测量结果并说明它比较的对象", []string{"给我一个提示，我该怎样理解测量结果？", "我找到的是高2摄氏度。", "这2摄氏度比较的是七个晴天下午城区和郊区观测点的平均气温，不是同一个地方一年内升高了多少。"}},
	}
	t.Setenv("CLARITY_SAMPLES", "1")
	for _, c := range cases {
		for repeat := 1; repeat <= 3; repeat++ {
			t.Run(fmt.Sprintf("%s-%d", c.name, repeat), func(t *testing.T) {
				blocks := SplitBlocks(c.article)
				tasks := []sqlc.ReadingTask{{ID: fixtureTaskID(1), Position: 1, Kind: "focus_block", Label: c.task, BlockID: "b2", Status: "pending"}, {ID: fixtureTaskID(2), Position: 2, Kind: "reflect", Label: "请说明这篇文章对你有什么启发", Status: "pending"}}
				msgs := []sqlc.AtomMessage{{Seq: 1, Role: "ai", Content: c.task + "。"}}
				for turn, student := range c.students {
					t.Run(fmt.Sprintf("turn-%d", turn+1), func(t *testing.T) {
						req := gateway.ChatRequest{Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: buildReadingCoachSystem("zh")}, {Role: gateway.RoleUser, Content: buildReadingCoachPrompt(c.name, blocks, readingOutline{}, tasks, msgs, nil, student, nil)}}}
						var reply string
						claritytest.Run(t, gateway.ClassDialogue, req, func(raw string) error {
							got, ok := parseReadingCoachReply(raw, blocks, "zh", func(string) bool { return true })
							if !ok {
								return fmt.Errorf("reading parse failed")
							}
							reply = got.Reply
							want := ""
							if turn == 2 {
								want = "done"
							}
							if got.Advance != want {
								return fmt.Errorf("advance %q, want %q", got.Advance, want)
							}
							return nil
						})
						msgs = append(msgs, sqlc.AtomMessage{Seq: int32(len(msgs) + 1), Role: "student", Content: student}, sqlc.AtomMessage{Seq: int32(len(msgs) + 2), Role: "ai", Content: reply})
					})
				}
			})
		}
	}
}
