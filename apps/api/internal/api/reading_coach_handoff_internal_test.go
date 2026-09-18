package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 2026-09-18 同事：「每次单独的步骤做完，不会在一条内容里引导完成下一步」。
// 判据里必须写出**下一步是什么**：原来清单只把当前这一步给模型，它想交也交不出去。
func TestStepInstructionNamesTheNextStep(t *testing.T) {
	tasks := []sqlc.ReadingTask{
		{Position: 0, Kind: "read", Label: "通读全文", Status: "done"},
		{Position: 1, Kind: "label", Label: "拆开作者的论证", Status: "pending"},
		{Position: 2, Kind: "critique", Label: "你怎么看", Detail: "作者说的你同意吗？", Status: "skipped"},
		{Position: 3, Kind: "reflect", Label: "总结论点", Detail: "作者的中心论点是什么？", Status: "pending"},
	}
	got := readingCurrentStepInstruction(tasks)
	if !strings.Contains(got, "「总结论点」") || !strings.Contains(got, "作者的中心论点是什么？") {
		t.Errorf("判据里没有下一步（跳过的那一步不算）：\n%s", got)
	}
	if strings.Contains(got, "「你怎么看」") {
		t.Errorf("已经跳过的那一步不是下一步：\n%s", got)
	}

	last := []sqlc.ReadingTask{
		{Position: 0, Kind: "read", Label: "通读全文", Status: "done"},
		{Position: 1, Kind: "hunt", Label: "找出中心论点", Status: "pending"},
	}
	if got := readingCurrentStepInstruction(last); !strings.Contains(got, "最后一步") {
		t.Errorf("最后一步要说收尾，不要编一个下一步：\n%s", got)
	}
}

// 卡片上的题取自一段话里的问句 —— 不能把前面的铺垫也带上，也不能带着加粗记号。
func TestQuestionInTakesJustTheQuestion(t *testing.T) {
	cases := []struct {
		text string
		last bool
		want string
	}{
		{"回到文章里：作者直接表明中心论点的是哪一句？把它点出来。", false, "作者直接表明中心论点的是哪一句？"},
		{"你猜的是数字。说理由：它凭什么说这全是**数据中心**的锅？写在下面这张卡上。", true, "它凭什么说这全是数据中心的锅？"},
		{"第一问？第二问？", true, "第二问？"},
		{"请打开段落工具，把它拆开。", false, ""},
	}
	for _, c := range cases {
		if got := questionIn(c.text, c.last); got != c.want {
			t.Errorf("questionIn(%q) = %q，想要 %q", c.text, got, c.want)
		}
	}
}

// 2026-09-18 同事：「出现新的分析的时候，原来的例子没有清除」。新板上夹着上一块板
// 摆过的句子就拿掉；整块都是摆过的句子（重摆一次）就原样留着。
func TestNewBoardDropsSentencesAlreadyPlaced(t *testing.T) {
	placed := map[string]string{"因自己的才能做到圆满。": "论点", "怎样才能做到圆满呢？": "论证"}
	board := func(qs ...string) *coachCard {
		c := &coachCard{Type: coachCardLabelRoles, Prompt: "p"}
		for _, q := range qs {
			c.Options = append(c.Options, coachCardOption{BlockID: "b1", Quote: q})
		}
		return c
	}
	mixed := dropAlreadyPlaced(board("因自己的才能做到圆满。", "这种叹气的声音，无论何人都会常在口边流露出来。", "这不是专门自己替自己开玩笑吗？"), placed)
	if len(mixed.Options) != 2 {
		t.Errorf("新旧混着的板应该只剩两句新的，拿到 %v", mixed.Options)
	}
	redo := dropAlreadyPlaced(board("因自己的才能做到圆满。", "怎样才能做到圆满呢？"), placed)
	if len(redo.Options) != 2 {
		t.Errorf("重摆一次的板不该动，拿到 %v", redo.Options)
	}
	thin := dropAlreadyPlaced(board("因自己的才能做到圆满。", "这种叹气的声音，无论何人都会常在口边流露出来。"), placed)
	if len(thin.Options) != 2 {
		t.Errorf("拿掉之后只剩一句就原样留着，拿到 %v", thin.Options)
	}
}
