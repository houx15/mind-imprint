package api

// writing_ceiling_live_test.go —— 年级门槛发给真模型跑一遍。
//
// 跑法：
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveCeiling -v -count=1
//
// 🚨 为什么这条必须用真模型：门槛是一段**克制**的指令 —— 它要模型**不要**做
// 一件它很想做的事。离线测试只能证明那段字在 prompt 里
// （[[prompt-output-must-be-verifiable-2026-09-03]]）。
//
// 用例挑的正是诱惑最大的那一轮：一个初二学生直接问「怎么写得更高级一点」。
// 不设门槛时，模型给出的「高级」写法就是倒装、虚拟语气、独立主格 ——
// 而这三样对初二是超纲的，她照着写只会写错，还会因为堆砌被扣恰当性的分。

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func TestLiveCeilingDoesNotPushSeniorGrammarAtAJuniorStudent(t *testing.T) {
	wr := sqlc.Writing{Lang: "en", Title: "Should students be allowed to bring phones to school?"}
	words := int32(120)
	wr.TargetWords = &words

	rows := []sqlc.WritingOutline{
		{Text: "Students should be allowed to bring phones", Kind: writingKindThesis, Depth: 0, Position: 0},
		{Text: "I can call my parents after school", Kind: writingKindPoint, Depth: 1, Position: 1},
	}
	said := "我觉得我写得太简单了，句子都是 I think... and... 你能教我几个更高级的写法吗？"

	out := livePlanTurnForGrade(t, genreArgument, "junior2", wr, rows, said)
	parsed, ok := parseWritingPlanReply(out)
	if !ok {
		t.Fatalf("真模型的回复读不出来：\n%s", out)
	}
	reply := parsed.Reply
	t.Logf("印记回的（初二）：%s", reply)

	// 🚨 这三样是高考弹药库里的，对初中超纲。源里逐字：
	// 「[G]/[C] 的高考弹药库（倒装、独立主格、虚拟语气）对七八年级是超纲的」。
	for _, banned := range []string{"虚拟语气", "倒装", "独立主格"} {
		if strings.Contains(reply, banned) {
			t.Errorf("对一个初二学生推了超纲写法 %q：%s", banned, reply)
		}
	}
	// 反方向：门槛不是「什么都别教」。她问了怎么提升，就该拿到她这个年级
	// 写得出来的那一档（In my opinion / not only…but also… / such…that…）。
	offered := false
	for _, ok8 := range []string{"In my opinion", "not only", "such", "too", "As far as"} {
		if strings.Contains(reply, ok8) {
			offered = true
			break
		}
	}
	if !offered {
		t.Errorf("她问了怎么写得更好，却什么也没拿到（门槛不该变成闭嘴）：%s", reply)
	}
}

// 同一轮，换成高二 —— 高中没有年级上的语法门槛，那三样不该再被拦掉。
//
// 这条是上面那条的对照：只证明「初中收到了门槛」还不够，还要证明
// **门槛没有漏到不该有它的地方**，否则等于把所有人都当初中生教。
func TestLiveCeilingLeavesSeniorStudentsAlone(t *testing.T) {
	wr := sqlc.Writing{Lang: "en", Title: "Should students be allowed to bring phones to school?"}
	words := int32(120)
	wr.TargetWords = &words

	rows := []sqlc.WritingOutline{
		{Text: "Students should be allowed to bring phones", Kind: writingKindThesis, Depth: 0, Position: 0},
		{Text: "I can call my parents after school", Kind: writingKindPoint, Depth: 1, Position: 1},
	}
	said := "我觉得我写得太简单了，句子都是 I think... and... 你能教我几个更高级的写法吗？"

	out := livePlanTurnForGrade(t, genreArgument, "senior2", wr, rows, said)
	parsed, ok := parseWritingPlanReply(out)
	if !ok {
		t.Fatalf("真模型的回复读不出来：\n%s", out)
	}
	t.Logf("印记回的（高二）：%s", parsed.Reply)
	// 这里不钉它必须说出某一个语法名字 —— 挑哪一样是它的判断。
	// 钉的是它**开口教了**：一句话都不给才是门槛漏错了地方。
	if len([]rune(parsed.Reply)) < 30 {
		t.Errorf("高二学生问怎么提升，回得过短：%s", parsed.Reply)
	}
}
