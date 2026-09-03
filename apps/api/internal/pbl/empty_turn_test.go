package pbl

import (
	"strings"
	"testing"
)

// 🚨 她没打字的那一轮，上文不能停在印记自己的话上。
//
// 2026-09-02 线上实测（mind-lite 生产环境）：她做完「观察日记」、带回两条观察、
// 按下「贴到板上」，印记的下一句是**一字不差地重复上一句**——「能不能先花几天
// 时间观察一下课间？」——并且把她刚做完的那件工具又递了一次。
//
// 原因是 postPblTurn 在 studentText == "" 时什么都不往 Recent 里加，于是模型
// 拿到的是一段以自己结尾的对话，续写最省力的办法就是把最后那句再说一遍。回灌
// 的内容其实一直都在 prompt 里（她一追问，印记立刻能背出那两条观察）——缺的
// 是「刚才发生了什么」这一句。
func TestBuildCoachContext_EmptyTurnSaysWhatJustHappened(t *testing.T) {
	ctx := buildCoachContext(CoachInput{
		Idea:         "下课没人去操场",
		JustHappened: "她做完了「观察日记」。",
		Recent: []Turn{
			{Role: "student", Content: "我猜大家就是懒"},
			{Role: "ai", Content: "能不能先花几天时间观察一下课间？"},
		},
	})
	if !strings.Contains(ctx, "她做完了「观察日记」") {
		t.Fatalf("没告诉印记刚发生了什么：\n%s", ctx)
	}
	// 光说发生了什么不够——必须明确禁止那两个具体的失败动作。
	for _, want := range []string{"不要重复你上一句", "不要再把这件事请她做一遍"} {
		if !strings.Contains(ctx, want) {
			t.Fatalf("缺少禁止重复的指令：%s\n---\n%s", want, ctx)
		}
	}
	// 这一段必须排在对话后面，否则会被后面的上文盖过去。
	if i, j := strings.Index(ctx, "她刚做完这件事"), strings.Index(ctx, "刚才说到"); i < j {
		t.Fatalf("「刚做完这件事」排在了对话前面，会被盖过去：\n%s", ctx)
	}
}

// 她打了字的那一轮不该出现这一段——那一轮的上文本来就以她的话结尾。
func TestBuildCoachContext_NoEventSectionWhenSheTyped(t *testing.T) {
	ctx := buildCoachContext(CoachInput{Idea: "下课没人去操场"})
	if strings.Contains(ctx, "她刚做完这件事") {
		t.Fatalf("没有事件却出现了事件那一段：\n%s", ctx)
	}
}

// 🚨 做完的工具不能再被递一次。
//
// 同一次实测里，印记把已经 done 的「观察日记」重新召了一遍，因为 prompt 里的
// 工具目录只有名字，没有状态。
func TestBuildCoachContext_ListsToolsAlreadyDone(t *testing.T) {
	ctx := buildCoachContext(CoachInput{
		Idea:      "下课没人去操场",
		ToolsUsed: []string{"观察日记", "问题识别"},
	})
	if !strings.Contains(ctx, "观察日记") || !strings.Contains(ctx, "问题识别") {
		t.Fatalf("做完的工具没告诉印记：\n%s", ctx)
	}
	if !strings.Contains(ctx, "不要再递") {
		t.Fatalf("没说清楚做完的就别再递：\n%s", ctx)
	}
}

// 🚨 tool_reason 是印记说给她本人看的一句话，会原样印在工具卡上。
//
// 线上实测拿到的是「他需要从观察事实开始，而不是凭猜测直接跳到解决方案。」
// ——prompt 通篇用第三人称写学生（36 处「他」），模型于是照着那个人称写理由，
// 而这句话直接印到了她的屏幕上。
func TestCoachSystem_TellsModelToolReasonAddressesHer(t *testing.T) {
	if !strings.Contains(coachSystem, "tool_reason") {
		t.Fatal("system prompt 里没有 tool_reason 的说明")
	}
	if !strings.Contains(coachSystem, "「你」称呼她") {
		t.Fatalf("没有规定 tool_reason 用第二人称——第三人称会印到她屏幕上")
	}
}
