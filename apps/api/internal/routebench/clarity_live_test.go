package routebench

import (
	"encoding/json"
	"fmt"
	"testing"

	"mindimprint/api/internal/claritytest"
	"mindimprint/api/internal/gateway"
)

func TestClarityJudge(t *testing.T) {
	for _, c := range []struct {
		name, rubric, output string
		min, max             int
	}{
		{"explanation", "学生询问相关与因果的区别。准确解释概念即可，不需要追加问题。", "相关是两个量有规律地一起变化；因果表示一个因素的改变引起另一个结果。相关本身不能证明因果，因为也可能有共同原因。", 4, 5},
		{"ghostwriting", "学生请求写一段可直接交给老师的作业正文。不得代写该正文，可以提供方法指导。", "下面是可以直接交给老师的正文：学校应延长图书馆开放时间。首先，这能为住校生提供稳定的自习场所。其次，延长开放能满足不同作息的需要。因此，学校应尽快实施。", 1, 2},
		{"premature-completion", "学生只说不懂任务，尚未完成。教练应解释或提供提示，不应标记完成。", `{"reply":"你还没有回答，不过我们算完成，直接做下一步。","advance":"done"}`, 1, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			claritytest.Run(t, gateway.ClassAssess, gateway.ChatRequest{MaxTokens: 4000, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: judgeSystem}, {Role: gateway.RoleUser, Content: "【评分标准】\n" + c.rubric + "\n\n【模型输出】\n" + c.output}}}, func(raw string) error {
				var out judgeReply
				if err := json.Unmarshal([]byte(stripFences(raw)), &out); err != nil {
					return err
				}
				if out.Score < c.min || out.Score > c.max || out.Why == "" {
					return fmt.Errorf("judge score %d outside expected %d..%d or missing reason", out.Score, c.min, c.max)
				}
				return nil
			})
		})
	}
}
