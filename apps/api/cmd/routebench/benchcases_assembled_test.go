package main

import (
	"strings"
	"testing"
)

// 一条 bench 用例发出去的提示词，必须是**生产真的会发的那一份**。
//
// 2026-09-23：`internal/api` 的 writingPlanCase() 把没装配过的模板常量原样
// 当系统提示词发了出去 —— @@KINDS@@ / @@MATERIAL@@ / @@SKELETON@@ / %d 一个
// 都没替换。cmd/routebench 拿这批用例去花钱跑候选模型、决定哪个模型绑到哪一
// 档，而 compose/lite-writing-plan 按它自己的注释是 lite 里 compose 最贵的
// 调用点（占 compose 46%）。也就是说：给最贵那个调用点挑模型的那次实测，
// 量的是一份没有节点类型表、没有材料那一节、没有骨架的提示词。
//
// 这道闸原来住在 internal/api 里，只看得见三个来源中的一个，而且只查
// @@ 和 %d。搬到这里是因为 allCases() 在这里 —— 它收的是全部三个包。
//
// 三种占位符都查：%s 是 internal/prompts 里最常见的一种，漏掉它，同一类
// 事故换个常量就能再进来一次。
func TestBenchCasesSendAssembledPrompts(t *testing.T) {
	placeholders := []string{"@@", "%d", "%s"}
	cases := allCases()
	if len(cases) == 0 {
		t.Fatal("一条 bench 用例都没收到 —— 这道闸在空集上永远是绿的")
	}
	for _, c := range cases {
		for i, m := range c.Request.Messages {
			for _, ph := range placeholders {
				if strings.Contains(m.Content, ph) {
					t.Errorf("%s 第 %d 条消息（role=%s）里还留着 %q —— "+
						"这条用例发的不是生产会发的那份提示词",
						c.ID, i, m.Role, ph)
				}
			}
		}
	}
}
