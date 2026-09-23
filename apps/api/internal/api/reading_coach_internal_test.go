package api

// reading_coach_internal_test.go — white-box tests for the coach's reply
// parsing and prompt assembly (unexported symbols), following the
// internal-test convention used by reading_brief_internal_test.go /
// writing_guide_internal_test.go / writing_plan_internal_test.go. Lives
// separately from reading_coach_test.go (package api_test, black-box) which
// cannot see unexported functions like parseReadingCoachReply.

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func TestParseReadingCoachReplyLens(t *testing.T) {
	blocks := []Block{{ID: "b1", Text: "第一段。"}, {ID: "b2", Text: "第二段。"}}
	allow := func(id string) bool { return id == "craap" }

	cases := []struct {
		name string
		json string
		want string
	}{
		{"aimed and allowed", `{"reply":"看这段","advance":"","focusBlock":"b2","lens":"craap"}`, "craap"},
		// An un-aimed lens is indistinguishable from her opening 透镜库
		// herself — the whole point of the coach summoning one is that it
		// lands on a paragraph it just talked about.
		{"un-aimed is dropped", `{"reply":"来看看来源","advance":"","focusBlock":"","lens":"craap"}`, ""},
		{"not allowed right now", `{"reply":"x","advance":"","focusBlock":"b1","lens":"sift"}`, ""},
		{"unknown id", `{"reply":"x","advance":"","focusBlock":"b1","lens":"nope"}`, ""},
		{"absent", `{"reply":"x","advance":"","focusBlock":"b1"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseReadingCoachReply(tc.json, blocks, "zh", allow)
			if !ok {
				t.Fatalf("parse failed for %s", tc.json)
			}
			if got.Lens != tc.want {
				t.Errorf("lens = %q, want %q", got.Lens, tc.want)
			}
			if got.Reply == "" {
				t.Error("a dropped lens must not take the reply down with it")
			}
		})
	}
}

// TestValidateReadingPicks — 一条 pick 活下来的条件是**它真的引了文章里的一句
// 话**。改述、空串照旧丢掉。
//
// 🚨 2026-09-23 这条测试改过，因为判据改了（产品负责人第 1 条：「学生划线句
// 包括答案句，AI就识别不出来，必须得一个字不差。」）：
//
//   - 原来「段号报错」= 整条丢掉。现在是**把段号纠正过来**。段号是客户端从
//     DOM 上算出来的，算错了是我们的毛病，不是她没点。丢掉的代价她担着 ——
//     印记 连【她在文章里点出来的句子】那一栏都看不见。
//   - 原来是字节级比对。现在按 quotematch 的规矩找（归一化），但存下来的
//     Quote 仍然是**原文里逐字的那一段**。
//
// 没有放宽的：归一化之后仍然要在某一段里连续出现。改述照旧一条都进不来。
func TestValidateReadingPicks(t *testing.T) {
	blocks := []Block{
		{ID: "b1", Text: "中国的碳排放总量位居世界第一。"},
		{ID: "b2", Text: "但人均排放仍低于多数发达国家。"},
	}
	got := validateReadingPicks([]readingPick{
		{BlockID: "b1", Quote: "碳排放总量位居世界第一"},     // 逐字 —— 留下
		{BlockID: "b2", Quote: "人均排放低于发达国家"},      // 改述 —— 丢掉
		{BlockID: "b1", Quote: "  "},              // 空 —— 丢掉
		{BlockID: "b9", Quote: "中国的碳排放总量"},        // 段号不存在 —— 纠正成 b1
		{BlockID: "b1", Quote: "但人均排放仍低于多数发达国家。"}, // 段号报错 —— 纠正成 b2
	}, blocks)
	if len(got) != 3 {
		t.Fatalf("kept %d picks, want 3: %+v", len(got), got)
	}
	want := []readingPick{
		{BlockID: "b1", Quote: "碳排放总量位居世界第一"},
		{BlockID: "b1", Quote: "中国的碳排放总量"},
		{BlockID: "b2", Quote: "但人均排放仍低于多数发达国家"},
	}
	for i, w := range want {
		if got[i].BlockID != w.BlockID || got[i].Quote != w.Quote {
			t.Errorf("pick %d = %+v, want %+v", i, got[i], w)
		}
	}
}

// 🚨 标点漂移不再让一条对的 pick 掉地上。这是产品负责人那一条的核心：
// 浏览器的选区带上或落下一个句号、一个半角逗号，都不该让「我明明划了」变成
// 「它说我没划」。
func TestValidateReadingPicksSurvivesPunctuationDrift(t *testing.T) {
	blocks := []Block{{ID: "b1", Text: "我料定这老女人并没有伤，别人也没有看见。"}}
	for _, quote := range []string{
		"我料定这老女人并没有伤。",  // 多了一个句号
		"我料定这老女人并没有伤",   // 少了那个逗号
		" 我料定这老女人并没有伤 ", // 两头带空格
		"我料定这老女人并没有伤,",  // 半角逗号
	} {
		got := validateReadingPicks([]readingPick{{BlockID: "b1", Quote: quote}}, blocks)
		if len(got) != 1 {
			t.Fatalf("%q: 被丢掉了", quote)
		}
		// 🚨 存下来的仍然是**原文里**那一段，不是她选区里的字。
		if got[0].Quote != "我料定这老女人并没有伤" {
			t.Errorf("%q: 存的不是原文的那一段：%q", quote, got[0].Quote)
		}
	}
}

// 🚨 跨了空行的选区拆成每段一条，而不是整条丢掉。她拖过一个空行的意思是
// 「这两段我都要」，不是「我什么都没划」。
func TestValidateReadingPicksSplitsACrossParagraphSelection(t *testing.T) {
	blocks := []Block{
		{ID: "b1", Text: "中国的碳排放总量位居世界第一。"},
		{ID: "b2", Text: "但人均排放仍低于多数发达国家。"},
	}
	got := validateReadingPicks([]readingPick{
		{BlockID: "b1", Quote: "中国的碳排放总量位居世界第一。\n\n但人均排放仍低于多数发达国家。"},
	}, blocks)
	if len(got) != 2 {
		t.Fatalf("kept %d, want 2: %+v", len(got), got)
	}
	if got[0].BlockID != "b1" || got[1].BlockID != "b2" {
		t.Errorf("段号不对：%+v", got)
	}
}

// 🚨 **绝不放宽比对。** 改述、她自己写的话、文章里根本没有的字，一条都不许
// 变成 pick —— 松了这一条，hunt 那一步就能靠她没点过的东西结束
// （memory: detector-must-target-the-real-failure）。
func TestValidateReadingPicksStillRefusesWhatIsNotInTheArticle(t *testing.T) {
	blocks := []Block{{ID: "b1", Text: "中国的碳排放总量位居世界第一。"}}
	for _, quote := range []string{
		"人均排放低于发达国家",        // 改述
		"我觉得这说明中国做得不够好",     // 她自己写的
		"碳排放总量位居世界第二",       // 改了一个字
		"位居世界第一的是中国的碳排放总量", // 同样的字，另一个顺序
	} {
		if got := validateReadingPicks([]readingPick{{BlockID: "b1", Quote: quote}}, blocks); len(got) != 0 {
			t.Errorf("%q: 不该被当成原文：%+v", quote, got)
		}
	}
}

// TestReadingCoachPrompt_RendersPicksAsOrdinalsNeverBlockIDs — a survived
// pick must show up as its own section, labelled 第几段 the same way the
// paragraph listing above it is, and must NEVER speak the block id (b2):
// the system prompt is explicit that block ids are an internal marker the
// model must not repeat back to her, and picks are no exception. An empty
// picks slice must omit the section entirely rather than print a bare
// heading.
func TestReadingCoachPrompt_RendersPicksAsOrdinalsNeverBlockIDs(t *testing.T) {
	blocks := []Block{
		{ID: "b1", Text: "中国的碳排放总量位居世界第一。"},
		{ID: "b2", Text: "但人均排放仍低于多数发达国家。"},
	}

	withPicks := buildReadingCoachPrompt("标题", blocks, readingOutline{}, nil, nil,
		[]readingPick{{BlockID: "b2", Quote: "但人均排放仍低于多数发达国家。"}}, "", nil, "")
	if !strings.Contains(withPicks, "【学生在文章里点出来的句子】") {
		t.Fatalf("missing picks section:\n%s", withPicks)
	}
	if !strings.Contains(withPicks, "第2段：「但人均排放仍低于多数发达国家。」") {
		t.Fatalf("pick not rendered as 第几段:\n%s", withPicks)
	}
	// The paragraph listing above legitimately says b2（第2段）— but nothing
	// in the picks section itself may hand the model a bare "b2" as
	// something it could echo back to her.
	if i := strings.Index(withPicks, "【学生在文章里点出来的句子】"); i >= 0 && strings.Contains(withPicks[i:], "b2") {
		t.Errorf("picks section leaks the block id:\n%s", withPicks[i:])
	}

	noPicks := buildReadingCoachPrompt("标题", blocks, readingOutline{}, nil, nil, nil, "", nil, "")
	if strings.Contains(noPicks, "【学生在文章里点出来的句子】") {
		t.Errorf("picks section must be omitted when no picks survive:\n%s", noPicks)
	}
}

// TestReadingCoachSystemCarriesTheRulings — the prompt is a product surface:
// these clauses are what stops the coach reading as a clipped AI, and what
// makes the hunt a hunt. Pinning them keeps a later edit from quietly
// deleting a ruling.
func TestReadingCoachSystemCarriesTheRulings(t *testing.T) {
	for _, want := range []string{
		"200 个字", // the raised cap (铁律③ is one QUESTION, not one sentence)
		"找出关键句",  // the hunt step's own section, named the way her plan names it
		"链接经验",   // the connect step's own section, same
		"lens",   // the lens field is documented in the output contract
		// The two clauses that let 印记 reach for a little structure — bold ONE
		// word, a short list for two or three options — and the guard rail that
		// keeps that permission from undoing 铁律③. Without the second line, a
		// list of three "options" is three questions on three lines, which is
		// exactly what the one-question-at-a-time rule forbids; the permission
		// and its limit are one ruling and must be pinned together.
		"加粗仅用于确有必要强调的概念",
		"一轮最多问一个需要学生回答的问题",
		// Task 12's live walk found this one MISSING in behaviour: the card
		// section merely permitted a card (「有时候…更管用」), so against a real
		// model the opening turn came back as the very prose the sub-project
		// was built to replace — a plan, a 通读 instruction, and 「读完告诉我一
		// 声」. The spec's ruling is that 步骤的指令由卡片承担, so the section now
		// states the default only for a still-active task and calls out the
		// first turn by name. A completion turn must not attach the next tool.
		"仅在当前任务尚未完成、需要学生动手时，默认给学生一张卡片",
		"第一轮也一样",
		// R1's headline ruling, and the correction it encodes. The literal-quote
		// validator guarantees she must LOOK at the article; it does not
		// guarantee she understood it. 「哪一句最让你觉得作者在讲『为什么』」 is
		// answered by scanning four options for 因为/所以 — zero comprehension,
		// and it passes every check the validator makes. The fix is the SHAPE of
		// the question, which only the prompt can carry, so the self-check
		// sentence is pinned verbatim.
		"围绕当前任务提出一个具体、可依据原文回答的问题",
		"避免只需匹配关键词的机械提问",
		// 🚨 5W1H and 不能有唯一正解 are TWO rulings that must both hold — shape
		// vs answer space. Pinning them together is what stops a later edit
		// "simplifying" one into the other.
		"允许有依据的不同答案",
		// No two near-identical cards in a row: same options, one word changed,
		// reads as 「你答错了，再选一次」 even with no ✓ and no ✗.
		"不要连着出两张几乎一样的卡片",
		// R4 (3): the guard above is about the OPTIONS, and the live walk slipped
		// straight past it — 「哪一句最能看出钱流向了谁」 followed by 「哪一句让你最
		// 清楚地看到钱去了哪里」 carried different option sets, so nothing fired,
		// and she was asked the same question twice. The ruling is about what the
		// question ASKS, so it has to be its own line.
		"上一张卡片问过的那件事，这一张就换一件事问",
		// R4 (1) 🚨 the structural guarantee, the prompt half. The validator drops
		// an all-one-paragraph option set, and that drop is SILENT (fewer than two
		// survivors → the whole card vanishes → it reads as 「模型这轮没给卡片」).
		// So the prompt must ASK for cross-paragraph options, or the guarantee is
		// bought by quietly losing cards.
		"choose_span 的选项必须跨段落取：至少来自两个不同的段落",
		"所有有效选项来自同一段时，卡片无法通过校验",
		// Never scold her for a thing she was pointed at the wrong half of the
		// screen for.
		"不要预设学生害怕、偷懒、不认真",
		// R4 (2): this used to be a literal blocklist (「还没做完」/「别急着往下走」/
		// 「第一步还没做完」) and the model routed around it by dropping one
		// character — 「读完第4段了，那这一步还没完」. A blocklist is the wrong
		// instrument: it enumerates phrasings, and phrasings are infinite. The
		// rule is now positive and about the ACT — do not comment on the fact
		// that she has not done it — with worked examples of what to say instead.
		"不要评价学生答得快慢",
		"不要求学生操作已经收起的组件",
		// text-heavy without a card was the original complaint; the cap alone
		// never fixed it.
		"解释清楚后就停止",
		// Emphasis serves explanation; the copy review removes mechanical bolding.
		"不要求每轮使用",
	} {
		if !strings.Contains(readingCoachSystem, want) {
			t.Errorf("readingCoachSystem no longer mentions %q", want)
		}
	}
	if strings.Contains(readingCoachSystem, "120 个字") {
		t.Error("the 120-字 cap is the mechanical cause of the AI voice; it must be gone")
	}
	// R4 (2): the blocklist itself must be gone, not merely joined by a positive
	// rule. Left in place it teaches the model to hunt for a phrasing that is not
	// on the list, which is exactly what the live walk caught it doing.
	if strings.Contains(readingCoachSystem, "绝对不要说「还没做完」") {
		t.Error("the literal blocklist is the wrong instrument; the positive rule replaces it")
	}
}

// TestBuildReadingCoachSystem_NoPlaceholderSurvives — assembly is
// strings.Replace at count 1, not fmt.Sprintf: a second unresolved
// placeholder would ship to the model as a literal, silently. Both %s (the
// paragraph-tool menu) and %LENS% (the lens menu) must be gone after
// buildReadingCoachSystem runs, for both languages.
func TestBuildReadingCoachSystem_NoPlaceholderSurvives(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		system := buildReadingCoachSystem(lang, "")
		if strings.Contains(system, "%LENS%") {
			t.Errorf("lang=%s: %%LENS%% placeholder survived assembly", lang)
		}
		if strings.Contains(system, "%s") {
			t.Errorf("lang=%s: %%s placeholder survived assembly", lang)
		}
	}
}

// TestParseReadingCoachReplyLens_NilLensOK — the guard's `lensOK == nil`
// short-circuit has no caller yet exercising it: every real call site passes
// a real predicate. A nil lensOK must still drop the lens rather than panic
// on the nil call.
// TestReadingCoachPrompt_TaskLinesCarryKind — F2. The system prompt's "## 两
// 种特别的步骤" section addresses connect and hunt BY NAME, but the task
// listing never wrote t.Kind — so the instructions and the state they govern
// were never joined up. Every task line must now carry its kind, in the same
// identifier the system prompt uses.
func TestReadingCoachPrompt_TaskLinesCarryKind(t *testing.T) {
	tasks := []sqlc.ReadingTask{
		{Status: "pending", Kind: "hunt", Label: "找出关键句", Detail: "在文章里点出一句。"},
		{Status: "pending", Kind: "connect", Label: "链接经验", Detail: "想到什么说什么。"},
		{Status: "pending", Kind: "read", Label: "通读全文"},
	}
	prompt := buildReadingCoachPrompt("标题", nil, readingOutline{}, tasks, nil, nil, "", nil, "")
	for _, want := range []string{"(hunt)", "(connect)", "(read)"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("task listing missing kind marker %q:\n%s", want, prompt)
		}
	}
}

func TestParseReadingCoachReplyLens_NilLensOK(t *testing.T) {
	blocks := []Block{{ID: "b1", Text: "第一段。"}, {ID: "b2", Text: "第二段。"}}

	got, ok := parseReadingCoachReply(
		`{"reply":"看这段","advance":"","focusBlock":"b2","lens":"craap"}`, blocks, "zh", nil)
	if !ok {
		t.Fatalf("parse failed")
	}
	if got.Lens != "" {
		t.Errorf("lens = %q, want dropped when lensOK is nil", got.Lens)
	}
	if got.Reply == "" {
		t.Error("a dropped lens must not take the reply down with it")
	}
}
