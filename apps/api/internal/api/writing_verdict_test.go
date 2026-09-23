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

// 每一种块该查什么，不该查什么 —— 同事 2026-09-20 的意见 9 的前半句：
// 「对每一段的分析需要明确各个段落各自的功能」。
func TestCommentSystemDispatchesByKind(t *testing.T) {
	opening := buildWritingCommentSystem("zh", 1, writingKindOpening, helpAsk, genreArgument)
	if !contains(opening, "不要求开头自带完整的事例") {
		t.Errorf("开头那一份没说清它的活：\n%s", opening)
	}

	point := buildWritingCommentSystem("zh", 1, writingKindPoint, helpAsk, genreArgument)
	// docs/2026-08-09-all-statuses.md §6 那条要求原样有效：
	//
	//	for each claim, we need evidence, analysis, limitation,
	//	how it correlates with others
	//
	// R4 起这四项用的是讲义里的名字（观点句 / 材料句 / 分析句），
	// 因为叫得出名字她才知道要补的是什么 —— 「还差一句把它和主张连起来的话」
	// 说的是缺什么，没说那一句叫什么。四项一项都没少，只是换了称呼。
	for _, want := range []string{
		"观点句", // claim
		"材料句", // evidence
		"分析句", // analysis
		"限制",  // limitation
		"别的段", // how it correlates with others
	} {
		if !contains(point, want) {
			t.Errorf("正文那一份缺 %q —— all-statuses §6 要的四项少了一项", want)
		}
	}
	// 🚨 **阐释句**是 R4 补上的那一句（讲义的五句型里，观点句和材料句之间
	// 的那座桥）。它最常被学生跳过，也最容易在下一次改 prompt 时被删掉。
	if !contains(point, "阐释句") {
		t.Error("正文那一份缺阐释句 —— 观点句和例子之间那座桥没人提，学生就会继续跳过它")
	}
	// 缺分析句的时候要点得出名字来，不能只说「要分析」。
	for _, want := range []string{"因果分析法", "假设分析法", "归纳分析法"} {
		if !contains(point, want) {
			t.Errorf("正文那一份没点名 %q —— 只说「要分析」等于没说怎么补", want)
		}
	}
	// 🚨 正文那一份**不能**带着「不要求开头自带事例」那句话 ——
	// 共用作用域里放某一个块的说明，是 2026-09-12 那条教训的原形
	// （阅读室的格子说明漏进了写作室）。
	if contains(point, "不要求开头自带完整的事例") {
		t.Error("开头那一块的说明漏进了正文那一份")
	}

	closing := buildWritingCommentSystem("zh", 1, writingKindClosing, helpAsk, genreArgument)
	if !contains(closing, "收束全文") {
		t.Errorf("结尾那一份没说清它的活：\n%s", closing)
	}

	rebuttal := buildWritingCommentSystem("zh", 1, writingKindRebuttal, helpAsk, genreArgument)
	if !contains(rebuttal, "反方") {
		t.Errorf("回应那一份没说清它的活：\n%s", rebuttal)
	}

	// 通篇审阅（kind 为空）不附分块的检查表。
	whole := buildWritingCommentSystem("zh", 3, "", helpAsk, genreArgument)
	for _, unwanted := range []string{"这一块是**开头**", "这一块是**结尾**"} {
		if contains(whole, unwanted) {
			t.Errorf("通篇那一份不该带分块的检查表（%q）", unwanted)
		}
	}
}

// 一层的等级只看落在这一层的 points。四层各自独立 —— 立意没问题、
// 字句一堆问题，她该看到的是「立意 pass，字句 revise」，而不是一个
// 笼统的 polish 把两件事拌在一起。
func TestLayerVerdictsComeFromThatLayerOnly(t *testing.T) {
	c := Comment{Points: []CommentPoint{
		{Kind: "issue", Layer: writingLayerSentence, Action: "把这句拆成两句"},
		{Kind: "good", Layer: writingLayerClaim},
	}}
	got := layerVerdictsOf(c)
	if got[writingLayerClaim] != writingVerdictPass {
		t.Errorf("立意那层没有 issue，应该 pass，拿到 %q", got[writingLayerClaim])
	}
	if got[writingLayerSentence] == writingVerdictPass {
		t.Error("字句那层有一条带 action 的 issue，不该是 pass")
	}
	// 没有任何 point 的那两层也要有值 —— 屏幕上四个格子都要有东西。
	if got[writingLayerMaterial] == "" || got[writingLayerStructure] == "" {
		t.Error("没有 point 的层也要给一个等级，不能是空")
	}
}

// 🚨 三档都要够得着。
//
// 第一版按「那条 issue 带没带 Action」判，而 validateCommentPoints 会把没有
// Action 的 point 整条丢掉（dropNoAction），于是活下来的 issue 全都带 Action，
// 三档塌成两档：pass 或 revise，polish 那一支永远走不到。后果是一处小的用词
// 问题就把「字句」判成 revise —— 屏幕上那是危险色，而 CommentPanel 里写着
// 「polish 不能长得像错误」。
//
// 这条测试钉住「一条 = polish，两条 = revise」，让中间那一档留在路上。
func TestLayerVerdictUsesAllThreeLevels(t *testing.T) {
	one := layerVerdictsOf(Comment{Points: []CommentPoint{
		{Kind: "issue", Layer: writingLayerSentence, Action: "把这句拆成两句"},
	}})
	if one[writingLayerSentence] != writingVerdictPolish {
		t.Errorf("一层只有一条 issue，应该是 polish，拿到 %q", one[writingLayerSentence])
	}

	two := layerVerdictsOf(Comment{Points: []CommentPoint{
		{Kind: "issue", Layer: writingLayerSentence, Action: "把这句拆成两句"},
		{Kind: "issue", Layer: writingLayerSentence, Action: "换掉重复的那个词"},
	}})
	if two[writingLayerSentence] != writingVerdictRevise {
		t.Errorf("一层有两条 issue，应该是 revise，拿到 %q", two[writingLayerSentence])
	}
}

// 不认识的值归一到 polish 而不是 revise —— writing_verdict.go 已有的
// 非对称兜底，按层算的时候不许把它改成对称的。
func TestLayerVerdictNeverInventsRevise(t *testing.T) {
	c := Comment{Points: []CommentPoint{{Kind: "issue", Layer: 99}}}
	for _, v := range layerVerdictsOf(c) {
		if v == writingVerdictRevise {
			t.Error("一个认不出层号的 point 把某一层判成了 revise")
		}
	}
}

// Source category must not affect readiness, regardless of language or length.
func TestPlanReadinessDoesNotRankMaterialSources(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		for _, length := range []int32{0, 500, 1200, 3000} {
			wr := sqlc.Writing{Lang: lang}
			if length > 0 {
				wr.TargetWords = &length
			}
			need := writingPlanNeedOf(wr)
			personal := writingPlanShape{Top: 1, Points: need.Points, Material: need.Material}
			external := personal
			external.Wider = need.Material
			if !personal.ready(need) || personal.ready(need) != external.ready(need) {
				t.Fatalf("source quota remains for %s/%d", lang, length)
			}
			if personal.missing(need) != "" {
				t.Fatal("source-only gap remains")
			}
		}
	}
	// Removing the automatic quota must not remove the teacher's task from input.
	prompt := "写一篇议论文，要求引用一份调查数据。"
	wr := sqlc.Writing{Lang: "zh", AssignedPrompt: &prompt}
	if !contains(buildWritingPlanPrompt(wr, nil, nil, ""), prompt) {
		t.Fatal("teacher requirements lost")
	}
}
