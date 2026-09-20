package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

func pieceSnippet(o uuid.UUID, text string) sqlc.WritingSnippet {
	return sqlc.WritingSnippet{
		ID:        uuid.New(),
		OutlineID: pgtype.UUID{Bytes: o, Valid: true},
		Text:      text,
	}
}

// 🚨 同事 2026-09-20 的意见 9：
//
//	「这个第一段的分析，这个具体的举例写在了第二段和第三段，但是 ai 在分析
//	  第一段的时候没有进行关联，也不知道开头段只是一个引子的作用，
//	  给出了错误的分析结果。」
//
// 它判不出来不是因为它笨，是因为 `buildWritingCommentPrompt` 手上**只有这一段**。
func TestPieceContextShowsTheOtherBlocks(t *testing.T) {
	wr := sqlc.Writing{Lang: "zh", Title: "学校应不应该允许学生带手机"}
	o1 := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindThesis, Depth: 0, Position: 0, Text: "应该允许"}
	o2 := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 1, Text: "放学能联系家长"}
	o3 := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 2, Text: "学习上能查题"}
	outline := []sqlc.WritingOutline{o1, o2, o3}
	snippets := []sqlc.WritingSnippet{
		pieceSnippet(o2.ID, "有一次我放学等车，用手机给我妈打了电话。"),
		pieceSnippet(o3.ID, "英语单词不会，我查了一下就懂了。"),
	}

	got := buildWritingPieceContext(wr, outline, snippets, nil, &o1)

	if !strings.Contains(got, "开头") {
		t.Errorf("没说清她停在的这一块是开头：\n%s", got)
	}
	// 🚨 别的块要带着**她写下的正文**，不只是标题 —— 意见 9 的核心就是这个。
	// 只给标题解决不了：标题是她的要点，不是她写的字。
	for _, want := range []string{"有一次我放学等车", "英语单词不会"} {
		if !strings.Contains(got, want) {
			t.Errorf("别的段写的内容没进上下文（缺 %q）：\n%s", want, got)
		}
	}
	// 每一块要说清它是什么，好让模型知道开头的活和正文的活不一样。
	if !strings.Contains(got, "分论点") {
		t.Errorf("别的块的种类没说：\n%s", got)
	}
}

// 她这一块之前收到过的意见，以及她做到了没有。
func TestPieceContextCarriesPriorComments(t *testing.T) {
	o := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 0, Text: "放学能联系家长"}
	sn := pieceSnippet(o.ID, "上周三五点半我放学等车，用手机给我妈打了电话，她来接我。")
	points, _ := json.Marshal([]CommentPoint{{
		Kind: "issue", Symptom: "example_no_detail",
		Text: "这件事没有时间地点。", Action: "把那天几点、在哪儿写进去。", Quote: "有一次我放学等车",
	}})
	prior := []sqlc.WritingComment{{
		ID: uuid.New(), Scope: "block", SnippetID: pgtype.UUID{Bytes: sn.ID, Valid: true},
		Summary: "这一段有事，但看不见。", Points: points,
		// 她后来改过了：现在这一版和当初读到的那一版不一样。
		SourceText: "有一次我放学等车。",
	}}

	got := buildWritingPieceContext(sqlc.Writing{Lang: "zh"},
		[]sqlc.WritingOutline{o}, []sqlc.WritingSnippet{sn}, prior, &o)

	if !strings.Contains(got, "把那天几点、在哪儿写进去") {
		t.Errorf("上一条意见没进上下文：\n%s", got)
	}
	if !strings.Contains(got, "她已经改过") {
		t.Errorf("没说她已经照着改过了 —— 于是它会把同一件事再说一遍：\n%s", got)
	}
}

// 她一个字都没改的时候，要说「还没动」—— 判错的方向不对称：
// 她没做完而产品当她做完了，是最伤的那个。
func TestPieceContextSaysWhenSheHasNotActed(t *testing.T) {
	o := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 0, Text: "放学能联系家长"}
	same := "有一次我放学等车。"
	sn := pieceSnippet(o.ID, same)
	points, _ := json.Marshal([]CommentPoint{{
		Kind: "issue", Symptom: "example_no_detail",
		Text: "这件事没有时间地点。", Action: "把那天几点、在哪儿写进去。", Quote: same,
	}})
	prior := []sqlc.WritingComment{{
		Scope: "block", SnippetID: pgtype.UUID{Bytes: sn.ID, Valid: true},
		Points: points, SourceText: same,
	}}

	got := buildWritingPieceContext(sqlc.Writing{Lang: "zh"},
		[]sqlc.WritingOutline{o}, []sqlc.WritingSnippet{sn}, prior, &o)
	if !strings.Contains(got, "还没动") {
		t.Errorf("她一个字都没改，上下文要说出来：\n%s", got)
	}
}

// 🚨 成本契约（2026-09-20）：她的正文每轮都变，必须排在稳定的块后面，
// 否则 DashScope 的隐式缓存整轮落空、按全价计费。
func TestPieceContextPutsVolatileLast(t *testing.T) {
	o := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Text: "理由"}
	got := buildWritingPieceContext(sqlc.Writing{Lang: "zh", Title: "校服该不该穿"},
		[]sqlc.WritingOutline{o}, nil, nil, &o)
	title := strings.Index(got, "校服该不该穿")
	focus := strings.Index(got, "她现在停在这一块")
	if title < 0 {
		t.Fatalf("题目没进去：\n%s", got)
	}
	if focus < 0 {
		t.Fatalf("没标出她停在哪一块：\n%s", got)
	}
	if title > focus {
		t.Errorf("稳定的块要排在易变的块前面（title=%d focus=%d）：\n%s", title, focus, got)
	}
}

// 🚨 一篇长稿不能整篇原样塞进去，但截断**要说出来**。
//
// 不说的话模型会以为她那一段就写了这么多，然后指责她写得短 ——
// [[observation-tool-is-the-bug-2026-09-12]]：她的字被切掉，而产品照着被切的
// 那一版评价她，她连着三次跟印记说「我的字被截断了」。
func TestPieceContextAnnouncesTruncation(t *testing.T) {
	o1 := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindThesis, Depth: 0, Position: 0, Text: "主张"}
	o2 := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 1, Text: "理由"}
	long := strings.Repeat("这是很长的一段。", 200) // 远超上限
	got := buildWritingPieceContext(sqlc.Writing{Lang: "zh"},
		[]sqlc.WritingOutline{o1, o2}, []sqlc.WritingSnippet{pieceSnippet(o2.ID, long)}, nil, &o1)

	if !strings.Contains(got, "后面还有") {
		t.Errorf("截断了却没说出来：\n%s", got[:min(len(got), 800)])
	}
	if len([]rune(got)) > 4000 {
		t.Errorf("上下文太长了（%d 字）—— 每一次「看看这一段」都会按它计费", len([]rune(got)))
	}
}

// 自由段落没有结构图节点（focus 是 nil），不能因此炸掉。
func TestPieceContextHandlesNoFocus(t *testing.T) {
	o := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindThesis, Depth: 0, Text: "主张"}
	got := buildWritingPieceContext(sqlc.Writing{Lang: "zh"}, []sqlc.WritingOutline{o}, nil, nil, nil)
	if strings.TrimSpace(got) == "" {
		t.Error("没有 focus 也该给出整篇的结构")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 打印一份真实形状的上下文 —— 人眼看一遍模型会读到什么。
//
//	go test ./internal/api -run TestPieceContextShape -v
func TestPieceContextShape(t *testing.T) {
	wr := sqlc.Writing{Lang: "zh", Title: "学校应不应该允许学生带手机"}
	w := int32(500)
	wr.TargetWords = &w
	th := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindThesis, Depth: 0, Position: 0, Text: "学校应该允许学生带手机"}
	p1 := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 1, Text: "放学能联系家长"}
	e1 := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindEvidence, Depth: 2, Position: 2, Text: "我放学等车那次"}
	p2 := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 3, Text: "学习上能查题"}
	cl := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindClosing, Depth: 0, Position: 4, Text: "允许带，但老师同意才能拿出来"}
	outline := []sqlc.WritingOutline{th, p1, e1, p2, cl}

	// 开头那一段就是同事截图里那一段：两个判断，具体的事在后面两段。
	opening := pieceSnippet(th.ID, "手机可以帮助我们联系家长，也能用来学习。所以我觉得学校应该允许学生带手机。")
	snippets := []sqlc.WritingSnippet{
		opening,
		pieceSnippet(p1.ID, "上周三五点半我放学等车，用手机给我妈打了电话，她来接我。"),
		pieceSnippet(p2.ID, "英语单词不会，我查了一下就懂了，比问同学快。"),
	}
	t.Logf("模型会读到的整篇上下文：\n%s", buildWritingPieceContext(wr, outline, snippets, nil, &th))
}

// 🚨 断言的是**喂给模型的那份 prompt 本身**，不是「它能解析」。
//
// 2026-09-11 的教训：系统提示词写着「method 取自【可用的方法】的 id」，而那张表
// 从来没被放进 prompt 过 —— 单元测试全绿（我喂的 JSON 里写的是真 id），
// 真模型回了两个不存在的 id。能验的东西，得先真的喂进去。
func TestCommentPromptCarriesTheWholePiece(t *testing.T) {
	wr := sqlc.Writing{Lang: "zh", Title: "学校应不应该允许学生带手机"}
	piece := "\n【整篇的结构，以及她在每一块写下的字】\n" +
		"- 第 2 张 · 分论点：放学能联系家长\n    她写的：上周三五点半我放学等车。\n"

	got := buildWritingCommentPrompt(wr, "她写的这一段", "手机可以帮助我们联系家长。", piece)
	if !strings.Contains(got, "上周三五点半我放学等车") {
		t.Errorf("整篇上下文没进 prompt：\n%s", got)
	}

	// 🚨 成本契约：整篇上下文要在方法表**之后**、她这一段的正文**之前**。
	methods := strings.Index(got, "【可用的方法】")
	ctx := strings.Index(got, "上周三五点半")
	para := strings.Index(got, "手机可以帮助我们联系家长")
	if methods < 0 || ctx < 0 || para < 0 {
		t.Fatalf("三个块少了一个（methods=%d ctx=%d para=%d）", methods, ctx, para)
	}
	if !(methods < ctx && ctx < para) {
		t.Errorf("块的顺序不对：方法表 %d → 整篇 %d → 她这一段 %d", methods, ctx, para)
	}
}

// 深入一层和写作引导也要吃到同一份 —— 三处分岔过一次就够了。
func TestDeepenAndGuideCarryTheWholePiece(t *testing.T) {
	wr := sqlc.Writing{Lang: "zh"}
	block := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindThesis, Depth: 0, Text: "主张"}
	piece := "别的段里写着：上周三五点半我放学等车。"

	deepen := buildDeepenBrief(wr, []sqlc.WritingOutline{block}, block, "", nil, "", piece)
	if !strings.Contains(deepen, "上周三五点半") {
		t.Errorf("深入一层没吃到整篇上下文：\n%s", deepen)
	}

	guide := buildWritingGuidePrompt(wr, block, []sqlc.WritingOutline{block}, "", nil, piece)
	if !strings.Contains(guide, "上周三五点半") {
		t.Errorf("写作引导没吃到整篇上下文：\n%s", guide)
	}

	// piece 为空时引导退回只列要点，不能整块消失。
	fallback := buildWritingGuidePrompt(wr, block, []sqlc.WritingOutline{block}, "", nil, "")
	if !strings.Contains(fallback, "整篇的结构") {
		t.Errorf("piece 为空时该退回只列要点：\n%s", fallback)
	}
}
