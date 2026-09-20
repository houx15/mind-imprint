package api

// writing_piece_live_test.go —— 拿同事 2026-09-20 那个真实用例去见一次真模型。
//
//	LIVE_LLM=1 go test ./internal/api -run TestLiveWritingPieceOpening -v -count=1
//
// 🚨 为什么只能用真模型验：stub 里我写什么它就回什么。这一轮要证的是
// **提示词和上下文合在一起，能不能让它不再拿正文的标准去量开头** ——
// 那是模型的判断，不是解析器的行为。

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// 同事截图里那一篇：500 字、带手机、开头两句判断，具体的事在第二三段。
func livePhoneEssay() (sqlc.Writing, []sqlc.WritingOutline, []sqlc.WritingSnippet) {
	wr := sqlc.Writing{Lang: "zh", Title: "学校应不应该允许学生带手机"}
	w := int32(500)
	wr.TargetWords = &w

	th := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindThesis, Depth: 0, Position: 0,
		Text: "学校应该允许学生带手机"}
	p1 := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 1,
		Text: "放学能联系家长"}
	p2 := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 2,
		Text: "学习上能查不会的题"}
	cl := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindClosing, Depth: 0, Position: 3,
		Text: "允许带，但老师同意才能拿出来"}
	outline := []sqlc.WritingOutline{th, p1, p2, cl}

	snippets := []sqlc.WritingSnippet{
		pieceSnippet(th.ID, liveOpeningParagraph),
		pieceSnippet(p1.ID, "上周三五点半我放学等车，公交迟迟不来，我用手机给我妈打了电话，她开车来接的我。"),
		pieceSnippet(p2.ID, "有一次英语作业里有个单词我不认识，我用手机查了意思，还听了发音，才看懂那句话。"),
	}
	return wr, outline, snippets
}

// 开头那一段：两句判断，具体的事在后面两段里。
const liveOpeningParagraph = "手机可以帮助我们联系家长，也能用来学习。所以我觉得学校应该允许学生带手机。"

// 🚨 同事的验收标准第一条：「开头预告、正文举例时，不要求开头重复补例子」。
func TestLiveWritingPieceOpeningIsNotAskedForAnExample(t *testing.T) {
	prov, resolved := liveClass(t, gateway.ClassReview)
	wr, outline, snippets := livePhoneEssay()
	focus := &outline[0] // 中心论点 → 开头那张卡

	piece := buildWritingPieceContext(wr, outline, snippets, nil, focus)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	res, err := gateway.Collect(ctx, prov, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildWritingCommentSystem(wr.Lang, writingBlockCommentMaxIssues, writingKindOpening)},
			{Role: gateway.RoleUser, Content: buildWritingCommentPrompt(wr, "她写的这一段", liveOpeningParagraph, piece, genreArgument)},
		},
	})
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	t.Logf("模型原样回的：\n%s", res.Text)

	parsed, ok := parseWritingComment(res.Text)
	if !ok {
		t.Fatalf("解析不了：\n%s", res.Text)
	}
	if !writingVerdictValid(parsed.Verdict) {
		t.Errorf("verdict 不合法：%q", parsed.Verdict)
	}

	points := validateCommentPoints(parsed.Points, liveOpeningParagraph, wr.Lang, writingBlockCommentMaxIssues)
	later := writingLaterBlocksText(outline, snippets, focus)
	points = dropIssuesLaterBlocksAnswer(points, writingKindOpening, later)

	t.Logf("verdict=%s summary=%q 减完剩 %d 条", parsed.Verdict, parsed.Summary, len(points))
	for _, p := range points {
		t.Logf("  [%s] symptom=%s\n    说：%s\n    做：%s", p.Kind, p.Symptom, p.Text, p.Action)
	}

	// 🚨 这条断言就是同事那句话：**开头不该被要求补一个具体的例子**，
	// 因为那两件事就写在后面两段里，而模型现在看得见它们。
	for _, p := range points {
		if p.Kind != "issue" {
			continue
		}
		for _, bad := range []string{"具体的事", "举一个例子", "举个例子", "没有例子", "缺少例子", "亲身经历"} {
			if strings.Contains(p.Text+p.Action, bad) {
				t.Errorf("还在要求开头补例子（「%s」）—— 那两件事就在后面两段里：\n说：%s\n做：%s",
					bad, p.Text, p.Action)
			}
		}
	}
}

// 正文那一段自带具体的事 ⇒ 这一轮不该判 revise。
func TestLiveWritingPieceBodyWithAnExampleIsNotRevise(t *testing.T) {
	prov, resolved := liveClass(t, gateway.ClassReview)
	wr, outline, snippets := livePhoneEssay()
	focus := &outline[1] // 第一条分论点
	body := snippets[1].Text

	piece := buildWritingPieceContext(wr, outline, snippets, nil, focus)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	res, err := gateway.Collect(ctx, prov, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildWritingCommentSystem(wr.Lang, writingBlockCommentMaxIssues, writingKindPoint)},
			{Role: gateway.RoleUser, Content: buildWritingCommentPrompt(wr, "她写的这一段", body, piece, genreArgument)},
		},
	})
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	parsed, ok := parseWritingComment(res.Text)
	if !ok {
		t.Fatalf("解析不了：\n%s", res.Text)
	}
	t.Logf("verdict=%s summary=%q", parsed.Verdict, parsed.Summary)
	for _, p := range parsed.Points {
		t.Logf("  [%s] %s / %s", p.Kind, p.Text, p.Action)
	}

	// 🚨 一段有时间、地点、人物的亲身经历，缺的顶多是一句「这凭什么证明你的
	// 看法」—— 那是 polish，不是 revise。同事：「将可选优化判为必改」。
	if parsed.Verdict == writingVerdictRevise {
		t.Errorf("一段自带具体事例的正文被判成必改：summary=%q", parsed.Summary)
	}
}
