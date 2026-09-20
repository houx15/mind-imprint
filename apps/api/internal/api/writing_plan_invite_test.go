package api

import (
	"testing"

	"github.com/google/uuid"
	"mindimprint/api/internal/store/sqlc"
)

// planRow 造一行提纲。
//
// 🚨 kind 必须给，而且要给真的那一个：0182 之后每一行都带着 kind，一个
// kind 空着的用例在数形状时会走 writingKindFromRole 的兜底，测出来的东西
// 就不是生产里发生的事（2026-09-14 的教训：用例的枚举值只从生产代码抄）。
func planRow(id string, depth int32, text, kind string) sqlc.WritingOutline {
	return sqlc.WritingOutline{
		ID:    uuid.NewSHA1(uuid.Nil, []byte(id)),
		Depth: depth,
		Kind:  kind,
		Role:  writingKindLabel(kind, ""),
		Text:  text,
	}
}

// 产品负责人 2026-09-12 带截图报的那一刻，照着复原。
//
// 图上已有：中心论点 ×2、分论点 ×1、她自己的经历 ×1。这一轮印记又加了第二条
// 分论点「各地都能开设很好的学校」，于是形状刚好过线；而它同一句话里还问了
// 「你自己有没有见过非省会城市办出好高中的例子？」。
//
// 绿框「计划已可开始写作」就贴在那个问号底下。
func TestWritingPlanInvite_CatchesTheQuestionOnTheTurnThatCrossesTheLine(t *testing.T) {
	rows := []sqlc.WritingOutline{
		planRow("a", 0, "人口多的城市，好高中多，大家读的高中不会那么统一", writingKindThesis),
		planRow("b", 0, "人多了，学校多了，自然就分散了", writingKindOpening),
		planRow("c", 1, "孩子多了学位不够，所以要多开设学校", writingKindPoint),
		planRow("d", 2, "家乡考生从五千涨到一万多，学校多开了好几所", writingKindEvidence),
		// 一份教育部的报道是她找来的，不是她见过的事 —— Wider 那条判据数的就是它。
		planRow("e", 2, "教育部公布的县域高中数量报道", writingKindReference),
	}

	// 还没加这一轮那条分论点之前：分论点只有 1 条，没过线。
	if writingPlanShapeOf(rows).ready(writingPlanNeedOf(sqlc.Writing{})) {
		t.Fatal("这一轮之前就该是没过线的")
	}

	// 🚨 2026-09-20：原来这里试了两种挂法（parentId 空 ⇒ 顶层，或挂在中心论点
	// 下面），断言「判据不该依赖它挂在哪一层」。现在它挂在哪一层已经不是模型
	// 能决定的事了 —— 一条分论点永远在深度 1。那个不确定性没有了，
	// 所以也没有两种挂法要试。
	add := []writingPlanAdd{{Kind: writingKindPoint, Text: "各地都能开设很好的学校"}}
	got := writingPlanShapeWith(rows, add)
	if got.Points != 2 || !got.ready(writingPlanNeedOf(sqlc.Writing{})) {
		t.Fatalf("加上这一轮那条分论点之后应当过线，得到 %+v", got)
	}

	reply := "好，这可以当第二条理由的骨架：各地都有条件办出好学校。但读者会问：凭什么各地「都能」？" +
		"中间差一步——是各地的学生基数撑起了学校，还是政策往各地分资源，或是别的？" +
		"你自己有没有见过非省会城市办出好高中的例子？"
	if !writingPlanReplyAsks(reply) {
		t.Error("这句话里明明有问号，判据没认出来")
	}
}

// 请她去写的那一句，不带问号，照常放行。
func TestWritingPlanReplyAsks_LetsTheInviteThrough(t *testing.T) {
	for _, s := range []string{
		"你这份计划现在站得住：一句话说清了人多和高中分散的关系，底下两条理由各有各的说法，" +
			"其中一条还挂着你自己家乡从五千考生涨到一万多的事。去写吧。",
		"Your plan holds up now. Go write the middle.",
	} {
		if writingPlanReplyAsks(s) {
			t.Errorf("把一句没有问号的邀请判成了提问：%s", s)
		}
	}
}

// 判据跟着篇幅走。
//
// 产品负责人 2026-09-12：「800字和3000字的论文的思路长短不一，需要扩展。」
// 在这之前，两种长度共用一条写死的线（两条分论点 + 一条材料），于是一篇 3000 字
// 的论文在第三条分论点还没出现时就被判成「想好了」。
func TestWritingPlanNeedOf_ScalesWithLength(t *testing.T) {
	words := func(n int32) *int32 { return &n }
	for _, tc := range []struct {
		name    string
		wr      sqlc.Writing
		wantPts int
		wantMat int
	}{
		// 2026-09-18：例子至少和分论点一样多，下限 2 个（800 字要 2–3 个例子）。
		{"没设目标", sqlc.Writing{Lang: "zh"}, 2, 2},
		{"800 字短文", sqlc.Writing{Lang: "zh", TargetWords: words(800)}, 2, 2},
		{"1600 字", sqlc.Writing{Lang: "zh", TargetWords: words(1600)}, 4, 4},
		{"3000 字论文，封顶", sqlc.Writing{Lang: "zh", TargetWords: words(3000)}, 4, 4},
		{"很短的也不低于两条", sqlc.Writing{Lang: "zh", TargetWords: words(200)}, 2, 2},
		{"英文按词算：500 词", sqlc.Writing{Lang: langEnglish, TargetWords: words(500)}, 2, 2},
		{"英文 1200 词", sqlc.Writing{Lang: langEnglish, TargetWords: words(1200)}, 4, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := writingPlanNeedOf(tc.wr)
			if got.Points != tc.wantPts || got.Material != tc.wantMat {
				t.Fatalf("要 %d 条分论点 / %d 条材料，得到 %+v", tc.wantPts, tc.wantMat, got)
			}
		})
	}
}

// 同一份形状，在短文里过线、在长论文里不过线 —— 这就是这条改动的全部意思。
func TestWritingPlanReady_SameShapeDiffersByLength(t *testing.T) {
	shape := writingPlanShape{Top: 1, Points: 2, Material: 2, Wider: 1}
	n := int32(800)
	short := sqlc.Writing{Lang: "zh", TargetWords: &n}
	m := int32(3000)
	long := sqlc.Writing{Lang: "zh", TargetWords: &m}

	if !shape.ready(writingPlanNeedOf(short)) {
		t.Error("800 字的短文，两条分论点加两个例子该过线")
	}
	if shape.ready(writingPlanNeedOf(long)) {
		t.Error("3000 字的论文，两条分论点就说想好了 —— 正是要修的那个毛病")
	}
}

// 空白节点和重复节点不该被数进来 —— 落库那边也会把它们丢掉。
func TestWritingPlanShapeWith_SkipsWhatTheInsertWouldDrop(t *testing.T) {
	rows := []sqlc.WritingOutline{planRow("a", 1, "孩子多了学位不够", writingKindPoint)}

	// 「挂在一个不存在的父亲上」那一条不见了：模型不再给 parentId，
	// 所以不再有这一类会被落库丢掉的节点。kind 编错的那一条在解析时就没了，
	// 到这个函数手上的每一条都算数。
	got := writingPlanShapeWith(rows, []writingPlanAdd{
		{Kind: writingKindPoint, Text: "   "},        // 空白
		{Kind: writingKindPoint, Text: "孩子多了学位不够"}, // 和图上那条一模一样
	})
	if got.Top != 0 || got.Points != 1 || got.Material != 0 {
		t.Fatalf("该丢的没丢：%+v", got)
	}
}
