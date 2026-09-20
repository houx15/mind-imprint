package api

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

func TestWritingVerdictClosedSet(t *testing.T) {
	for _, v := range []string{writingVerdictPass, writingVerdictPolish, writingVerdictRevise} {
		if !writingVerdictValid(v) {
			t.Errorf("%q 应当合法", v)
		}
	}
	if writingVerdictValid("很好") {
		t.Error("编出来的 verdict 不该合法")
	}

	// 🚨 认不出来的退到 polish，不是 revise。
	// 判错的方向不对称：把一处可选的优化说成必改，正是同事指出的那个毛病。
	if got := normalizeWritingVerdict("看起来不错"); got != writingVerdictPolish {
		t.Errorf("认不出的 verdict 该退到 polish，得到 %q", got)
	}
	if got := normalizeWritingVerdict("  REVISE "); got != writingVerdictRevise {
		t.Errorf("大小写和空白不该让一个合法的 verdict 掉队，得到 %q", got)
	}
}

// 🚨 同事 2026-09-20 的验收标准第一条：
// 「开头预告、正文举例时，不要求开头重复补例子」。
func TestOpeningNotAskedForAnExampleTheBodyCarries(t *testing.T) {
	points := []CommentPoint{{
		Kind: "issue", Symptom: "claim_no_evidence",
		Text: "这一句是判断，没有一件具体的事。", Action: "举一件你经历过的事。",
		Quote: "手机能帮我们联系家长",
	}}
	later := "上周三五点半我放学等车，用手机给我妈打了电话，她来接我。"

	if got := dropIssuesLaterBlocksAnswer(points, writingKindOpening, later); len(got) != 0 {
		t.Fatalf("开头是引子、例子在后面的段里 —— 这一条该丢掉：%+v", got)
	}

	// 🚨 正文段里同样的毛病**不能**丢：那一段就是该带例子的地方。
	if got := dropIssuesLaterBlocksAnswer(points, writingKindPoint, later); len(got) != 1 {
		t.Fatalf("正文段的「缺例子」不该被丢掉：%+v", got)
	}

	// 🚨 后面的段里也没有具体的事：这一条是真的缺，不能丢。
	vague := "手机还有很多别的好处，对学习也有帮助。"
	if got := dropIssuesLaterBlocksAnswer(points, writingKindOpening, vague); len(got) != 1 {
		t.Fatalf("后面也没有例子，这一条是真的缺：%+v", got)
	}
	if got := dropIssuesLaterBlocksAnswer(points, writingKindOpening, ""); len(got) != 1 {
		t.Fatalf("后面的段还没写，这一条不能丢：%+v", got)
	}
}

// 她照着上一条改过了，就不要再说一遍（意见 10：「排除……已解决和重复的问题」）。
func TestDropIssuesSheAlreadyFixed(t *testing.T) {
	sid := uuid.New()
	old, _ := json.Marshal([]CommentPoint{{
		Kind: "issue", Symptom: "example_no_detail",
		Text: "这件事没有时间地点。", Action: "把那天几点、在哪儿写进去。", Quote: "有一次我放学等车",
	}})
	prior := []sqlc.WritingComment{{
		Scope: "block", SnippetID: pgtype.UUID{Bytes: sid, Valid: true},
		Points: old, SourceText: "有一次我放学等车。",
	}}
	fresh := []CommentPoint{{
		Kind: "issue", Symptom: "example_no_detail",
		Text: "这件事还是没有时间地点。", Action: "把那天几点、在哪儿写进去。", Quote: "我放学等车",
	}}

	// 她改过了：现在这一版和当初那一版不一样。
	changed := "上周三五点半我放学等车，用手机给我妈打了电话。"
	if got := dropIssuesSheAlreadyFixed(fresh, prior, sid, changed); len(got) != 0 {
		t.Fatalf("她已经改过的那一条不该再说一遍：%+v", got)
	}

	// 🚨 她一个字都没改：必须留着。判错的方向不对称 ——
	// 她没做完而产品当她做完了，比多说一次更糟。
	same := "有一次我放学等车。"
	if got := dropIssuesSheAlreadyFixed(fresh, prior, sid, same); len(got) != 1 {
		t.Fatalf("她一个字都没改，这一条必须留着：%+v", got)
	}

	// 别人那一段的意见不算数。
	other := uuid.New()
	if got := dropIssuesSheAlreadyFixed(fresh, prior, other, changed); len(got) != 1 {
		t.Fatalf("别的段的历史意见不该影响这一段：%+v", got)
	}

	// 换了一种毛病：这一条是新的，留着。
	newKind := []CommentPoint{{
		Kind: "issue", Symptom: "claim_no_evidence",
		Text: "这一句还是判断。", Action: "说清这件事凭什么证明你的看法。", Quote: "我放学等车",
	}}
	if got := dropIssuesSheAlreadyFixed(newKind, prior, sid, changed); len(got) != 1 {
		t.Fatalf("换了一种毛病的那一条是新的，要留着：%+v", got)
	}
}

// 后面那几块她写下的字 —— 只数**这一块之后**的。
func TestWritingLaterBlocksText(t *testing.T) {
	o1 := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindThesis, Depth: 0, Position: 0}
	o2 := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 1}
	o3 := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 2}
	outline := []sqlc.WritingOutline{o1, o2, o3}
	snippets := []sqlc.WritingSnippet{
		pieceSnippet(o1.ID, "开头这一段。"),
		pieceSnippet(o2.ID, "第二段写的东西。"),
		pieceSnippet(o3.ID, "第三段写的东西。"),
	}

	got := writingLaterBlocksText(outline, snippets, &o1)
	for _, want := range []string{"第二段写的东西", "第三段写的东西"} {
		if !contains(got, want) {
			t.Errorf("缺 %q：%q", want, got)
		}
	}
	// 🚨 **她自己这一段不算「后面」** —— 算进去的话，一段自带例子的正文会
	// 被判成「后面有例子」，于是那条意见被丢掉，而她其实什么都没看到。
	if contains(got, "开头这一段") {
		t.Errorf("这一块自己不该算进「后面的段」：%q", got)
	}
	// 最后一块后面没有东西。
	if s := writingLaterBlocksText(outline, snippets, &o3); contains(s, "第三段") {
		t.Errorf("最后一块后面不该有东西：%q", s)
	}
}
