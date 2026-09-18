package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

// 🚨 走查和 routebench 的阅读用例曾经把步骤状态写成 "active"/"todo"，而生产的
// currentReadingTask 只认 "pending"。于是 prompt 里没有当前步，模型被告知
// 「所有步骤都走完了。跟她说一句收尾的话」，第一轮就把答案讲完收尾 ——
// 每个模型在这一格都拿 1 分，而那测的是用例，不是陪练。
//
// 这条测试守的是用例本身：喂给模型的 prompt 必须有且只有一个当前步。
func TestReadingFixturesHaveACurrentStep(t *testing.T) {
	check := func(name string, msgs []gateway.ChatMessage) {
		t.Helper()
		var user string
		for _, m := range msgs {
			if m.Role == gateway.RoleUser {
				user = m.Content
			}
		}
		if user == "" {
			t.Fatalf("%s: no user message", name)
		}
		if strings.Contains(user, "所有步骤都走完了") {
			t.Errorf("%s: the fixture tells the coach every step is finished", name)
		}
		if n := strings.Count(user, "她现在在这一步"); n != 1 {
			t.Errorf("%s: current-step marker appears %d times, want exactly 1", name, n)
		}
	}
	check("coachwalk reading driver", NewReadingWalkDriver().Request().Messages)
	for _, c := range BenchCases() {
		if c.Suite == liteReadingCoachSuite {
			check("routebench "+c.ID, c.Request.Messages)
		}
	}
}
