package api

import (
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 规划那一步的停止判据。这些是「读代码看不出对错」的那一类：一个字符串算不算
// 「等于没答」，以及「连续两轮」到底从哪儿数起。

func student(text string) sqlc.AtomMessage {
	return sqlc.AtomMessage{Role: "student", Content: text}
}
func coach(text string) sqlc.AtomMessage {
	return sqlc.AtomMessage{Role: "ai", Content: text}
}

func TestLowSubstanceReply(t *testing.T) {
	empty := []string{
		"", "   ", "嗯", "嗯。", "嗯嗯", "还行", "不知道", "我不知道", "没什么",
		"没感觉", "随便", "都行", "ok", "OK", "I don't know", "不太清楚",
		"好的！", "行吧～",
	}
	for _, s := range empty {
		if !lowSubstanceReply(s) {
			t.Errorf("lowSubstanceReply(%q) = false, want true", s)
		}
	}

	// 🚨 这一组是这条判据真正的边界：**含有**「不知道」但确实说了一件事。
	// 子串比对会把它们判成空的，然后恰好在她终于说出一件事的那一轮停止提问。
	real := []string{
		"不知道该从哪儿说起，可能是食堂那个事吧",
		"我们学校食堂每天倒掉的饭特别多",
		"没什么特别的想法，但我想写我爷爷修自行车那件事",
		"我想写的是校车该不该装安全带",
	}
	for _, s := range real {
		if lowSubstanceReply(s) {
			t.Errorf("lowSubstanceReply(%q) = true, want false — she actually said something", s)
		}
	}
}

// 「连续两轮」必须包含她**这一轮刚说的那句话**。调用点读 msgs 的时候这一轮
// 还没落库，所以只数 msgs 永远差最新的一轮。
func TestWritingPlanStalled_CountsThePendingTurn(t *testing.T) {
	msgs := []sqlc.AtomMessage{
		student("我想写食堂浪费"), coach("为什么这件事值得写？"), student("不知道"), coach("你见过哪一次？"),
	}
	if !writingPlanStalled(msgs, "嗯") {
		t.Fatal("two low-substance replies in a row (the stored 不知道 + this turn's 嗯) must stall")
	}
	if writingPlanStalled(msgs, "上周五我数了一下，有六个桶是满的") {
		t.Fatal("a substantive reply this turn must clear the stall, whatever came before")
	}
}

// 只数**结尾处**连续的那一段。中间敷衍过、最近答得好，不该停。
func TestWritingPlanStalled_OnlyTheTrailingRun(t *testing.T) {
	msgs := []sqlc.AtomMessage{
		student("不知道"), coach("……"), student("嗯"), coach("……"),
		student("我想写我们小区那个垃圾分类的牌子没人看"), coach("……"),
	}
	if writingPlanStalled(msgs, "牌子上写的字特别小，我奶奶根本看不清") {
		t.Fatal("an earlier run of empty replies must not stall a conversation that recovered")
	}
}

// 一轮不算「连续」。谁都会有一句「嗯」。
func TestWritingPlanStalled_OneEmptyTurnIsNotEnough(t *testing.T) {
	msgs := []sqlc.AtomMessage{student("我想写食堂浪费"), coach("为什么？")}
	if writingPlanStalled(msgs, "嗯") {
		t.Fatal("a single 嗯 must not stall — only a RUN of them does")
	}
}

// 老师说的话不算数：印记 连说两轮不影响她答得好不好。
func TestWritingPlanStalled_IgnoresCoachTurns(t *testing.T) {
	msgs := []sqlc.AtomMessage{
		student("我想写校车该不该装安全带"), coach("嗯"), coach("好的"),
	}
	if writingPlanStalled(msgs, "嗯") {
		t.Fatal("only her replies count toward the stall; the coach's own short turns must not")
	}
}

// `node(text, depth)` 和 ready 那条地板本身已经有 TestPlanLooksReady 在守
//（writing_plan_ready_internal_test.go），这里不再写第二遍，只测它没测的两件事：
// 计数本身，和那段要喂进 prompt 的事实。
func TestWritingPlanShape_CountsByDepth(t *testing.T) {
	rows := []sqlc.WritingOutline{
		node("中心论点", 0), node("理由一", 1), node("理由二", 1),
		node("我上周数的那六个桶", 2), node("  ", 1), // 空的不算
	}
	s := writingPlanShapeOf(rows)
	if s.Top != 1 || s.Points != 2 || s.Material != 1 {
		t.Fatalf("shape = %+v, want {Top:1 Points:2 Material:1}", s)
	}
	// 一份判据两个实现是它们悄悄分岔的唯一原因。
	if planLooksReady(rows) != s.ready() {
		t.Fatal("planLooksReady and writingPlanShape.ready must never disagree")
	}
}

// prompt 里那段事实要说出**还缺什么**——模型下一个问题直接由缺口决定。
func TestWritingPlanShape_PromptBlockNamesWhatIsMissing(t *testing.T) {
	block := writingPlanShapeOf([]sqlc.WritingOutline{node("中心论点", 0)}).promptBlock()
	for _, want := range []string{"分论点还不到两条", "还没有一条她自己"} {
		if !contains(block, want) {
			t.Fatalf("prompt block should name the gap %q:\n%s", want, block)
		}
	}

	full := writingPlanShapeOf([]sqlc.WritingOutline{
		node("a", 0), node("b", 1), node("c", 1), node("d", 2),
	}).promptBlock()
	if !contains(full, "这一轮就请她去写") {
		t.Fatalf("a plan that meets every criterion must say so:\n%s", full)
	}
}
