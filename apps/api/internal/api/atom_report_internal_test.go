package api

// atom_report_internal_test.go — validateMoments is unexported, so its test
// lives here (package api), not in atom_report_test.go (package api_test).

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

func TestValidateMoments(t *testing.T) {
	corpus := "我觉得人均排放更能说明责任。\n作者只讲了总量，没有讲人口。"
	got := validateMoments([]reportMoment{
		{Quote: "人均排放更能说明责任", Where: "我的收获"},
		{Quote: "她展现了批判性思维", Where: "我的收获"}, // AI's own prose — dropped
		{Quote: "中国碳排放世界第一", Where: "批注"},   // the article's — dropped
		{Quote: "  ", Where: "批注"},          // empty — dropped
	}, corpus)
	if len(got) != 1 {
		t.Fatalf("kept %d, want 1: %+v", len(got), got)
	}
	if got[0].Quote != "人均排放更能说明责任" {
		t.Errorf("kept the wrong moment: %+v", got[0])
	}
}

// TestBuildReadingNotes — 我的笔记 on the report. Three rules, each of which
// has a way of quietly breaking: a bare highlight is not a note, the cap is
// real, and both halves of a note survive intact (the article's sentence AND
// hers — see reportNote's doc comment on why conflating them is an R4 leak).
func TestBuildReadingNotes(t *testing.T) {
	got := buildReadingNotes([]sqlc.AtomAnnotation{
		{Quote: "第一段的句子。", Note: "  这里作者偷换了概念。  "},
		{Quote: "第二段的句子。", Note: "   "}, // a bare highlight — not a note
		{Quote: "第三段的句子。", Note: "这条要回去查。"},
	})
	if len(got) != 2 {
		t.Fatalf("kept %d notes, want 2: %+v", len(got), got)
	}
	if got[0].Note != "这里作者偷换了概念。" {
		t.Errorf("note not trimmed: %q", got[0].Note)
	}
	if got[0].Quote != "第一段的句子。" {
		t.Errorf("the article sentence must travel WITH her note, labelled: %q", got[0].Quote)
	}
	if got[1].Note != "这条要回去查。" {
		t.Errorf("second note = %q", got[1].Note)
	}
}

// A reader who marked up the whole article gets a poster, not a transcript.
func TestBuildReadingNotesCaps(t *testing.T) {
	var many []sqlc.AtomAnnotation
	for i := 0; i < maxReportNotes+7; i++ {
		many = append(many, sqlc.AtomAnnotation{Quote: "原文", Note: "笔记"})
	}
	if got := buildReadingNotes(many); len(got) != maxReportNotes {
		t.Fatalf("kept %d, want the cap %d", len(got), maxReportNotes)
	}
}

// cleanKeepSummary guards the report's biggest card against a model that
// ignores "3-4 句". Short text passes through untouched; long text is cut on a
// sentence boundary when there is one, and marked with an ellipsis when there
// isn't, so a truncation never masquerades as the end of a thought.
func TestCleanKeepSummary(t *testing.T) {
	if got := cleanKeepSummary("  你把装机量和发电量分开了。  "); got != "你把装机量和发电量分开了。" {
		t.Errorf("short summary should pass through trimmed, got %q", got)
	}
	if got := cleanKeepSummary("   "); got != "" {
		t.Errorf("blank summary must be empty (the caller renders no card), got %q", got)
	}

	// Long, WITH sentence ends past the halfway mark: cut on the last one.
	sentence := strings.Repeat("你注意到了这里的差别。", 40) // 11 runes each, far over the cap
	got := cleanKeepSummary(sentence)
	if len([]rune(got)) > maxKeepRunes {
		t.Fatalf("over the cap: %d runes", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "。") {
		t.Errorf("should have cut on a sentence end, got tail %q", got[len(got)-12:])
	}

	// Long, with NO sentence end at all: hard cut, visibly marked.
	noStops := strings.Repeat("字", maxKeepRunes+50)
	got = cleanKeepSummary(noStops)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("a hard cut must be visible, got tail %q", got[len(got)-6:])
	}
	if len([]rune(got)) != maxKeepRunes+1 { // +1 for the ellipsis
		t.Errorf("hard cut length = %d runes, want %d", len([]rune(got)), maxKeepRunes+1)
	}
}

// 用了透镜 N 个 counts SUBMITTED cards only — a card she summoned and walked
// away from is not a lens she used, and counting it inflates her own record.
func TestCountSubmittedCards(t *testing.T) {
	got := countSubmittedCards([]sqlc.AtomCard{
		{Status: "submitted"},
		{Status: "draft"},
		{Status: "submitted"},
		{Status: "abandoned"},
	})
	if got != 2 {
		t.Fatalf("countSubmittedCards = %d, want 2", got)
	}
}

// TestReportDedupesMomentsAgainstKeep is F4: her 收获 is already rendered verbatim
// as `keep`, so a moment that is the SAME sentence (or a substring of it)
// must be dropped — otherwise the same line prints twice on one report. A
// moment unrelated to keep, and a nil/empty keep, must both pass every
// moment through untouched.
func TestReportDedupesMomentsAgainstKeep(t *testing.T) {
	keep := &reportKeep{Label: "我的收获", Text: "我觉得应该多看数据来源，而不是只看结论。"}
	moments := []reportMoment{
		{Quote: "我觉得应该多看数据来源，而不是只看结论。", Where: "我的收获"}, // exact match with keep — dropped
		{Quote: "多看数据来源", Where: "我的收获"},               // substring of keep — dropped
		{Quote: "碳排放全球第一", Where: "写论证的时候"},            // unrelated — kept
	}

	got := dedupeMomentsAgainstKeep(moments, keep)

	if len(got) != 1 {
		t.Fatalf("kept %d, want 1: %+v", len(got), got)
	}
	if got[0].Quote != "碳排放全球第一" {
		t.Errorf("kept the wrong moment: %+v", got[0])
	}

	// A nil keep (writing reports never have one) must be a no-op.
	if got := dedupeMomentsAgainstKeep(moments, nil); len(got) != len(moments) {
		t.Errorf("nil keep should pass every moment through, got %d of %d", len(got), len(moments))
	}

	// An empty-text keep must also be a no-op — nothing to dedupe against.
	if got := dedupeMomentsAgainstKeep(moments, &reportKeep{Label: "我的收获", Text: "   "}); len(got) != len(moments) {
		t.Errorf("blank-text keep should pass every moment through, got %d of %d", len(got), len(moments))
	}
}

// TestLiteReportSystemAddressesHerDirectly — V2 fix: the report's `gains`
// used to come back in third person ("她抓住了…"), which reads as the
// student being described to someone else rather than a teacher speaking to
// her. The prompt must instruct second person (你) for gains, and must not
// itself model or invite third-person reference (她/这位学生/该生) — a
// third-person exemplar in the instruction would just teach the model the
// habit it's supposed to forbid. Precedent for pinning a prompt clause this
// way: TestReadingCoachSystemCarriesTheRulings in
// reading_coach_internal_test.go.
// TestBuildReadingLensNotes covers the four behaviours the lens-notes
// builder must have (see this feature's task doc): it picks the STUDENT
// anchor's quote — never the AI's own grounding example — pairs it with
// framework_fill.finding, skips any card that isn't 'submitted', skips a
// card whose id the registry doesn't know (rather than printing the raw
// id), and preserves the rows' own order.
func TestBuildReadingLensNotes(t *testing.T) {
	atomID := uuid.New()

	craapNote := sqlc.AtomCard{
		ID: uuid.New(), AtomID: atomID, CardID: "craap", Status: "submitted",
		Anchors: []byte(`[
			{"author":"ai","quote":"AI 挑的例句，不该出现"},
			{"author":"student","quote":"她自己选的句子一"}
		]`),
		FrameworkFill: []byte(`{"finding":"发现一"}`),
	}
	unsubmitted := sqlc.AtomCard{
		ID: uuid.New(), AtomID: atomID, CardID: "sift", Status: "proposed",
		Anchors:       []byte(`[{"author":"student","quote":"还没提交，不该出现"}]`),
		FrameworkFill: []byte(`{"finding":"还没提交，不该出现"}`),
	}
	unknownID := sqlc.AtomCard{
		ID: uuid.New(), AtomID: atomID, CardID: "not_a_real_card", Status: "submitted",
		Anchors:       []byte(`[{"author":"student","quote":"未知卡片，不该出现"}]`),
		FrameworkFill: []byte(`{"finding":"未知卡片，不该出现"}`),
	}
	concessionNote := sqlc.AtomCard{
		ID: uuid.New(), AtomID: atomID, CardID: "concession", Status: "submitted",
		Anchors:       []byte(`[{"author":"student","quote":"她自己选的句子二"}]`),
		FrameworkFill: []byte(`{"finding":"发现二"}`),
	}
	empty := sqlc.AtomCard{
		// submitted, known id, but neither a student anchor nor a finding —
		// nothing to show, must be skipped.
		ID: uuid.New(), AtomID: atomID, CardID: "sift", Status: "submitted",
		Anchors:       []byte(`[]`),
		FrameworkFill: []byte(`{}`),
	}

	got := buildReadingLensNotes([]sqlc.AtomCard{craapNote, unsubmitted, unknownID, concessionNote, empty})

	if len(got) != 2 {
		t.Fatalf("got %d lens notes, want 2: %+v", len(got), got)
	}

	if got[0].Quote != "她自己选的句子一" {
		t.Errorf("note 0 quote = %q, want the STUDENT anchor, not the AI example", got[0].Quote)
	}
	if got[0].Finding != "发现一" {
		t.Errorf("note 0 finding = %q, want %q", got[0].Finding, "发现一")
	}
	if got[0].Lens != "信源辨识卡 CRAAP / CRRAAB" {
		t.Errorf("note 0 lens = %q, want the craap card's registry display name", got[0].Lens)
	}

	// Order: craapNote comes before concessionNote in the input, and the
	// unsubmitted/unknown/empty rows between them must not shift that.
	if got[1].Quote != "她自己选的句子二" {
		t.Errorf("note 1 quote = %q, want the second submitted card's student pick, in order", got[1].Quote)
	}
	if got[1].Finding != "发现二" {
		t.Errorf("note 1 finding = %q, want %q", got[1].Finding, "发现二")
	}
	if got[1].Lens != "让步段 · 以退为进" {
		t.Errorf("note 1 lens = %q, want the concession card's registry display name", got[1].Lens)
	}
}

// TestBuildReadingLensNotesDropsDegradedFinding is the fix for the finding
// raised on 47d1c7c4's review: ev.Degraded (selectionEvalDTO, readeval.go)
// means agent.fallbackEval minted the finding, not a model that actually
// read her sentence — "你选了这句作为证据。" is the REAL canned string
// fallbackEval sets (reading_eval.go), used verbatim here so this test
// fails if that string ever drifts silently. A degraded card must keep her
// quote (her pick is her work regardless of whether the model said anything
// useful) but drop the canned finding — never present it as the room's
// genuine 发现.
func TestBuildReadingLensNotesDropsDegradedFinding(t *testing.T) {
	atomID := uuid.New()
	const cannedFinding = "你选了这句作为证据。" // agent.fallbackEval's exact canned text

	degraded := sqlc.AtomCard{
		ID: uuid.New(), AtomID: atomID, CardID: "craap", Status: "submitted",
		Anchors:       []byte(`[{"author":"student","quote":"她自己选的句子"}]`),
		FrameworkFill: []byte(`{"finding":"` + cannedFinding + `","degraded":true}`),
	}

	got := buildReadingLensNotes([]sqlc.AtomCard{degraded})

	if len(got) != 1 {
		t.Fatalf("got %d lens notes, want 1 (the quote must survive a degraded finding): %+v", len(got), got)
	}
	if got[0].Quote != "她自己选的句子" {
		t.Errorf("quote = %q, want her pick kept even though the finding degraded", got[0].Quote)
	}
	if got[0].Finding != "" {
		t.Errorf("finding = %q, want empty — the canned fallback text must never be shown as a genuine 发现", got[0].Finding)
	}
}

func TestLiteReportSystemAddressesHerDirectly(t *testing.T) {
	if !strings.Contains(liteReportSystem, "用\"你\"称呼她本人") {
		t.Error("liteReportSystem must explicitly instruct gains to address her as 你, not describe her in third person")
	}
	for _, banned := range []string{"她抓住了", "她能说出", "她调整了", "这位学生", "该生"} {
		if strings.Contains(liteReportSystem, banned) {
			t.Errorf("liteReportSystem must not model third-person reference as an example, found %q", banned)
		}
	}
}

// 转折时刻这一节的全部安全性都压在这条测试上：**正文必须来自行，不来自模型**。
// 模型只回编号，服务端拿编号去 atom_message 里取原话。所以「引了她没说过的
// 句子」这件事在这里是结构上不可能的，不是被验出来的。
func TestResolveTurningPointsTakesItsTextFromTheRowsNotTheModel(t *testing.T) {
	pairs := numberedTurns([]sqlc.AtomMessage{
		{Seq: 1, Role: "student", Content: "我觉得人均排放更能说明责任。"},
		{Seq: 2, Role: "ai", Content: "那总量还重要吗？"},
		{Seq: 3, Role: "system", Content: "工具已了结"},
		{Seq: 4, Role: "student", Content: "重要，但它们回答的是两个问题。"},
		{Seq: 5, Role: "ai", Content: "把这句写进结论试试。"},
	})
	if len(pairs) != 2 {
		t.Fatalf("numbered %d turns, want 2: %+v", len(pairs), pairs)
	}

	got := resolveTurningPoints([]modelTurnPick{
		{Turn: 2, Why: "她在这里把两个问题分开了"},
		{Turn: 2, Why: "重复的编号"},
		{Turn: 9, Why: "越界"},
		{Turn: 0, Why: "编号从 1 开始"},
		{Turn: 1, Why: "   "},
	}, pairs)

	if len(got) != 1 {
		t.Fatalf("kept %d, want 1: %+v", len(got), got)
	}
	if got[0].Student != "重要，但它们回答的是两个问题。" || got[0].Coach != "把这句写进结论试试。" {
		t.Errorf("text did not come from the rows verbatim: %+v", got[0])
	}
	if got[0].Why != "她在这里把两个问题分开了" {
		t.Errorf("why: %q", got[0].Why)
	}
}

// 最多三条。模型给十条也只留三条 —— 这一节是报告上的一块，不是一份逐字记录，
// 那份记录在「对话」那一格里。
func TestResolveTurningPointsKeepsAtMostThree(t *testing.T) {
	var msgs []sqlc.AtomMessage
	var picks []modelTurnPick
	for i := 1; i <= 10; i++ {
		msgs = append(msgs,
			sqlc.AtomMessage{Seq: int32(i * 2), Role: "student", Content: "她的第" + strconv.Itoa(i) + "句"},
			sqlc.AtomMessage{Seq: int32(i*2 + 1), Role: "ai", Content: "印记接的第" + strconv.Itoa(i) + "句"},
		)
		picks = append(picks, modelTurnPick{Turn: i, Why: "理由"})
	}
	if got := resolveTurningPoints(picks, numberedTurns(msgs)); len(got) != 3 {
		t.Fatalf("kept %d, want 3", len(got))
	}
}

// 她说完没等到回复就走了，这一轮仍然算一轮 —— 她说的那句话不会因为没人接就
// 不存在。Coach 为空，渲染时那一半不显示。
func TestNumberedTurnsKeepsHerLastWordWithNoReply(t *testing.T) {
	pairs := numberedTurns([]sqlc.AtomMessage{
		{Seq: 1, Role: "student", Content: "我想再想想。"},
	})
	if len(pairs) != 1 || pairs[0].Coach != "" {
		t.Fatalf("pairs: %+v", pairs)
	}
}

func TestTurnsBlockNumbersHerTurnsAndSaysWhoSpoke(t *testing.T) {
	block := buildTurnsBlock(numberedTurns([]sqlc.AtomMessage{
		{Seq: 1, Role: "student", Content: "我觉得人均排放更重要。"},
		{Seq: 2, Role: "ai", Content: "为什么？"},
	}))
	for _, want := range []string{"1.", "她：", "你："} {
		if !strings.Contains(block, want) {
			t.Fatalf("block does not read as a numbered transcript (missing %q):\n%s", want, block)
		}
	}
}

// 🚨 喂给模型的那一份可以截断（它只是用来挑编号的），**渲染出来的那一份
// 永远不截断**。2026-09-12：她自己写的字被切到 400，她跟印记说了三次
// 「我的字被截断了」然后重打了整段。
func TestTurnPromptIsCappedButTheStoredTextIsNot(t *testing.T) {
	long := strings.Repeat("我", 900)
	msgs := []sqlc.AtomMessage{
		{Seq: 1, Role: "student", Content: long},
		{Seq: 2, Role: "ai", Content: "嗯。"},
	}
	pairs := numberedTurns(msgs)

	block := buildTurnsBlock(pairs)
	if len([]rune(block)) > turnPromptCap+120 {
		t.Errorf("the prompt copy should be capped, got %d runes", len([]rune(block)))
	}

	got := resolveTurningPoints([]modelTurnPick{{Turn: 1, Why: "这里"}}, pairs)
	if len(got) != 1 {
		t.Fatalf("kept %d", len(got))
	}
	if got[0].Student != long {
		t.Errorf("her own words were truncated on the way to the report: %d runes, want %d", len([]rune(got[0].Student)), len([]rune(long)))
	}
}

func TestParseReportReplyReadsTurningPoints(t *testing.T) {
	reply, ok := parseReportReply(`{"moments":[],"gains":[],"summary":"","turningPoints":[{"turn":3,"why":"她改了主意"}]}`)
	if !ok {
		t.Fatal("did not parse")
	}
	if len(reply.TurningPoints) != 1 || reply.TurningPoints[0].Turn != 3 {
		t.Fatalf("turningPoints: %+v", reply.TurningPoints)
	}
}

// 冻结的判据不是「函数返回几」，而是「存下来的那块 JSON 里有没有这个数」——
// 报告是一整块存进 atom_report.report 的 jsonb，序号一旦写进去就再也不会被
// 重新数。这条测试守的是那个形状。
func TestReportOrdinalIsStoredAsANumberAndOmittedWhenAbsent(t *testing.T) {
	b, err := json.Marshal(liteReportDTO{Version: 1, Kind: "reading", Ordinal: 8})
	if err != nil {
		t.Fatal(err)
	}
	var back liteReportDTO
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Ordinal != 8 {
		t.Fatalf("ordinal did not survive the round trip: %d", back.Ordinal)
	}

	// 0 的意思是「这份报告早于这个字段」。整个键必须缺席 —— 前端据此不渲染
	// 那一句，而渲染成「第 0 篇」比不渲染糟得多。
	old, err := json.Marshal(liteReportDTO{Version: 1, Kind: "reading"})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(old, []byte("ordinal")) {
		t.Errorf("a report with no ordinal must omit the key entirely: %s", old)
	}
}
