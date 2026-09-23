package vocab

import (
	"bytes"
	"os"
	"testing"
)

// The library is the ONLY place 印记 may get a method name from, so a malformed
// entry is a product defect, not a runtime inconvenience — it must fail at load.
func TestLoad_EveryMethodIsUsable(t *testing.T) {
	all := All()
	if len(all) < 10 {
		t.Fatalf("library has %d methods, want at least 10", len(all))
	}
	seen := map[string]bool{}
	for _, m := range all {
		if m.ID == "" || m.Name == "" || m.Definition == "" {
			t.Errorf("method %+v is missing id, name or definition", m)
		}
		if seen[m.ID] {
			t.Errorf("duplicate method id %q", m.ID)
		}
		seen[m.ID] = true
		switch m.AppliesTo {
		// "whole" 是论证结构那一层（2026-09-20）：它属于整篇，不属于任何一个
		// 位置。For / ForLang 把它挡在按块取方法的那几条路外面。
		case "opening", "body", "closing", "any", WholePiece:
		default:
			t.Errorf("method %q has applies_to %q, want opening|body|closing|any|whole", m.ID, m.AppliesTo)
		}
		switch m.Lang {
		case "zh", "en", "any":
		default:
			t.Errorf("method %q has lang %q, want zh|en|any", m.ID, m.Lang)
		}
		// Borrowed material by construction: an example with no topic cannot be
		// checked for being about someone else's subject.
		for _, ex := range m.Examples {
			if ex.Topic == "" || ex.Text == "" {
				t.Errorf("method %q has an example missing topic or text", m.ID)
			}
		}
		for _, p := range m.Patterns {
			if p.Frame == "" {
				t.Errorf("method %q has a pattern with no frame", m.ID)
			}
		}
		if len(m.Examples) == 0 && len(m.Patterns) == 0 {
			t.Errorf("method %q teaches nothing: no examples and no patterns", m.ID)
		}
	}
}

// TestEveryMethodHasAStudentFacingName pins the 2026-08-28 ruling: 论证 is
// itself jargon, so `name` is what she reads (plain, an action she already
// performs) and `formal_name` is the curriculum term a card reveals. A missing
// `name` would leave her reading nothing; a missing `formal_name` is legitimate
// ONLY where no curriculum term exists — which is a short, deliberate list, not
// "whatever someone forgot to fill in".
func TestEveryMethodHasAStudentFacingName(t *testing.T) {
	// Named exceptions, for CHINESE entries only — a 语文 method without a
	// curriculum term is rare enough that each one should be argued for by
	// name rather than waved through.
	formalMayBeEmpty := map[string]bool{
		// 「最后提个建议」has no 语文 term of its own.
		"closing_scope": true,
	}
	// The English half is a RULE, not a list: an `en` entry's `name` already
	// IS what an English writer reads ("Naming with an appositive"), and there
	// is no second, more formal English label to reveal on a card. This used
	// to be three ids written out by hand, which meant every new English
	// method failed this test for a reason that was never a defect — and the
	// 2026-09-04 batch (vocabulary / sentence formats / story line) added nine
	// at once. Encoding the reason instead of the instances is what keeps the
	// test about the invariant.
	for _, m := range All() {
		if m.Name == "" {
			t.Errorf("method %q has no student-facing name", m.ID)
		}
		mayBeEmpty := formalMayBeEmpty[m.ID] || m.Lang == "en"
		if m.FormalName == "" && !mayBeEmpty {
			t.Errorf("method %q has an empty formal_name; only lang=en entries and %v are allowed to", m.ID, formalMayBeEmpty)
		}
		if formalMayBeEmpty[m.ID] && m.FormalName != "" {
			t.Errorf("method %q now has formal_name %q — update the exception list rather than leaving it stale", m.ID, m.FormalName)
		}
	}
	// Label is what a prompt prints: the plain name, with the curriculum term
	// alongside it so 印记 can say one and know the other.
	pee, ok := ByID("point_pee")
	if !ok {
		t.Fatal(`ByID("point_pee") not found`)
	}
	if pee.Name != "举个例子" || pee.FormalName != "举例论证" {
		t.Errorf("point_pee = %q / %q, want 举个例子 / 举例论证", pee.Name, pee.FormalName)
	}
	if got := pee.Label(); got != "举个例子（正式名称：举例论证）" {
		t.Errorf("point_pee.Label() = %q", got)
	}
	// Where the two names coincide, saying it twice teaches nothing.
	direct, _ := ByID("opening_direct")
	if got := direct.Label(); got != "开门见山" {
		t.Errorf("opening_direct.Label() = %q, want the bare name (both names are identical)", got)
	}
	scope, _ := ByID("closing_scope")
	if got := scope.Label(); got != "最后提个建议" {
		t.Errorf("closing_scope.Label() = %q, want the bare name (no formal term exists)", got)
	}
}

func TestByID_AndFor(t *testing.T) {
	if _, ok := ByID("point_concession"); !ok {
		t.Fatal(`ByID("point_concession") not found`)
	}
	if _, ok := ByID("no_such_method"); ok {
		t.Fatal("ByID returned ok for an unknown id")
	}
	openings := For("opening", "zh", "")
	if len(openings) == 0 {
		t.Fatal(`For("opening", "zh", "") returned nothing`)
	}
	for _, m := range openings {
		if m.AppliesTo != "opening" && m.AppliesTo != "any" {
			t.Errorf("For(\"opening\") returned %q with applies_to %q", m.ID, m.AppliesTo)
		}
	}
}

// TestFor_NeverOffersEnglishWordingToAChinesePiece pins the TWO properties the
// language axis exists for — deliberately as properties, not as counts, so that
// re-tagging an entry wrongly fails here instead of quietly passing:
//
//  1. A Chinese piece is never offered an English EXPRESSION. The wording lives
//     in `patterns` — so what we check is that no frame reaching lang=zh is
//     written in LATIN SCRIPT, which is the failure itself rather than a
//     stand-in for it.
//
//     🚨 2026-09-21: this used to read `len(m.Patterns) > 0`, on the premise
//     (stated in this comment) that frames were the only language-bound thing
//     in the library and therefore always English. R4 brought in the 分析句三法
//     from a Chinese teacher's 讲义, whose whole value is their 句式
//     (「假如……，那么……？」) — Chinese frames, for a Chinese piece. The old
//     line fired on them: it was aimed at the shadow of the failure, not the
//     failure. Reading the script keeps the real guarantee and also catches an
//     English entry that someone forgot to tag `lang: "en"`, which a bare
//     lang check would wave through.
//  2. An English piece keeps a full vocabulary at EVERY position. Tagging the
//     structural methods "zh" would have left an English writer with one
//     opening method and no closings — the mirror image of the reported bug,
//     and just as much a bug (2026-08-28 ruling).
func TestFor_NeverOffersEnglishWordingToAChinesePiece(t *testing.T) {
	positions := []string{"opening", "body", "closing"}

	for _, pos := range append(positions, "any") {
		for _, m := range For(pos, "zh", "") {
			if f, ok := latinFrame(m); ok {
				t.Errorf("For(%q, \"zh\") offered %q, whose frame %q is English wording — 中文作文 must never be handed it", pos, m.ID, f)
			}
			if m.Lang == "en" {
				t.Errorf("For(%q, \"zh\") offered English-only method %q", pos, m.ID)
			}
		}
	}
	for _, m := range ForLang("zh", "") {
		if f, ok := latinFrame(m); ok {
			t.Errorf(`ForLang("zh", "") offered %q, whose frame %q is English wording`, m.ID, f)
		}
		if m.Lang == "en" {
			t.Errorf(`ForLang("zh", "") offered %q, an English-only entry`, m.ID)
		}
	}

	// The English half: every position must still have something to teach.
	for _, pos := range positions {
		if got := For(pos, "en", ""); len(got) == 0 {
			t.Errorf("For(%q, \"en\") returned nothing — an English writer has no method to be offered at this position", pos)
		}
	}
	// …and the frames themselves are what an English writer gets that a
	// Chinese one must not.
	var sawFrames bool
	for _, m := range For("body", "en", "") {
		if len(m.Patterns) > 0 {
			sawFrames = true
		}
	}
	if !sawFrames {
		t.Error(`For("body", "en", "") offered no sentence frames — English writers lost the entries that are theirs`)
	}

	// The structural methods serve both, so a Chinese piece keeps its own.
	for _, pos := range positions {
		if got := For(pos, "zh", ""); len(got) == 0 {
			t.Errorf("For(%q, \"zh\") returned nothing", pos)
		}
	}
}

// TestEmbeddedCopyMatchesSourceOfTruth guards the fork forced by go:embed's
// inability to escape the package directory: packages/contracts/vocab/methods.json
// is the editable source of truth, apps/api/internal/vocab/methods.json is the
// embedded copy. Nothing enforces they stay identical except this test, so if
// someone edits one and forgets the other, this must fail loudly rather than
// let the two ends of the product quietly disagree about what a method is called.
func TestEmbeddedCopyMatchesSourceOfTruth(t *testing.T) {
	sourceOfTruth, err := os.ReadFile("../../../../packages/contracts/vocab/methods.json")
	if err != nil {
		t.Fatalf("could not read source of truth: %v", err)
	}
	if !bytes.Equal(sourceOfTruth, methodsJSON) {
		t.Fatal("apps/api/internal/vocab/methods.json has drifted from packages/contracts/vocab/methods.json — copy the source of truth over the embedded file and rerun")
	}
}

// TestEnglishPieceGetsVocabSentenceAndStoryMethods pins the 2026-09-04 ask,
// verbatim from the product owner:
//
//	> currently english directions for snippets, they write with not enough
//	> guidance, english should have methods about vocab, sentence formats,
//	> and also story line.
//
// Before that batch, every English-only entry was an ARGUMENT frame
// (concession / qualify / evidence). A student writing an English narrative —
// or any student stuck on a sentence rather than on a claim — was offered
// nothing that spoke to what she was actually doing. That is what "not enough
// guidance" meant, and no test could have caught it, because nothing was
// broken: the library simply had a hole shaped like two thirds of English
// writing.
//
// Asserted by id rather than by counting: the point is not "there are more
// methods now", it is that each of the three kinds of help is reachable, at a
// position where it makes sense.
func TestEnglishPieceGetsVocabSentenceAndStoryMethods(t *testing.T) {
	families := map[string][]string{
		"vocabulary":      {"en_word_precision", "en_word_register"},
		"sentence format": {"en_sentence_variety", "en_sentence_opener", "en_sentence_appositive", "en_sentence_parallel"},
		"story line":      {"en_story_scene", "en_story_turn", "en_story_landing"},
	}
	for family, ids := range families {
		for _, id := range ids {
			m, ok := ByID(id)
			if !ok {
				t.Errorf("%s: method %q is gone", family, id)
				continue
			}
			if m.Lang != "en" {
				t.Errorf("%s: %q has lang %q — these carry English wording and must never reach a Chinese piece", family, id, m.Lang)
			}
			if len(m.Examples) == 0 && len(m.Patterns) == 0 {
				t.Errorf("%s: %q teaches nothing", family, id)
			}
		}
	}

	// A story line needs all three of its beats, each where it belongs — an
	// arc with no turn is just events in the order they happened.
	for _, tc := range []struct{ id, pos string }{
		{"en_story_scene", "opening"},
		{"en_story_turn", "body"},
		{"en_story_landing", "closing"},
	} {
		var found bool
		for _, m := range For(tc.pos, "en", "") {
			if m.ID == tc.id {
				found = true
			}
		}
		if !found {
			t.Errorf("For(%q, \"en\") does not offer %q", tc.pos, tc.id)
		}
	}

	// 🚨 And none of it leaks the other way. TestFor_NeverOffersEnglishWording
	// ToAChinesePiece covers that generally; naming the new families here means
	// a future edit that retags one of them "any" fails with the reason
	// attached, rather than as a count mismatch somewhere else.
	for _, m := range ForLang("zh", "") {
		for family, ids := range families {
			for _, id := range ids {
				if m.ID == id {
					t.Errorf("%s: %q reached a Chinese piece", family, id)
				}
			}
		}
	}
}

// 同事 2026-09-20 的意见 5：「几种写法不太规范，我找了一些论证结构的思维导图
// 和教辅，可以参考一下这些论证方法规范一下语言」。
//
// 照他给的那两页教辅：论证方法有七种、论证结构有四种，库里各缺一半。
func TestVocabHasTheStandardArgumentMethods(t *testing.T) {
	want := []string{"举例论证", "引用论证", "对比论证", "比喻论证", "因果论证", "类比论证", "归谬论证"}
	have := map[string]bool{}
	for _, m := range All() {
		if m.FormalName != "" {
			have[m.FormalName] = true
		}
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("论证方法缺 %q", w)
		}
	}
}

func TestVocabHasTheFourArgumentStructures(t *testing.T) {
	want := map[string]bool{"总分式": true, "并列式": true, "层进式": true, "对照式": true}
	for _, m := range Structures("", "") {
		delete(want, m.Name)
		if m.Category != "structure" {
			t.Errorf("%s 的 category 该是 structure，得到 %q", m.Name, m.Category)
		}
		if m.AppliesTo != "whole" {
			t.Errorf("%s 的 applies_to 该是 whole，得到 %q", m.Name, m.AppliesTo)
		}
	}
	if len(want) != 0 {
		t.Errorf("论证结构缺：%v", want)
	}
}

// 🚨 整篇层的那四条**不许漏进按块取方法的那几条路**。
// 「这一段用总分式」是句错话，而 ForLang 正是喂给规划和批量引导两个 prompt
// 的那一份 —— 漏进去，模型就会在某一段的引导里说「这一段可以用总分式」。
func TestWholePieceStructuresStayOutOfPerBlockLists(t *testing.T) {
	for _, m := range ForLang("zh", "") {
		if m.AppliesTo == "whole" {
			t.Errorf("整篇层的 %q 漏进了 ForLang", m.Name)
		}
	}
	for _, pos := range []string{"opening", "body", "closing"} {
		for _, m := range For(pos, "zh", "") {
			if m.AppliesTo == "whole" {
				t.Errorf("整篇层的 %q 漏进了 For(%q)", m.Name, pos)
			}
		}
	}
}

// 🚨 一个现有的名字都不许改：提示词散文里引着它们，而
// api 包的 TestWritingPlanSystem_NamesOnlyRealMethods 逐字钉着 ——
// 在这里改名，会让那条测试在一个看上去和方法库毫不相干的地方红掉。
func TestVocabKeepsTheNamesPromptsAlreadyUse(t *testing.T) {
	for _, n := range []string{
		"并列论证", "递进论证", "对比论证", "举个例子", "讲道理",
		"先承认，再反驳", "说清前因后果", "留个悬念", "先抛一个问题",
		"开门见山", "从一件事讲起", "结尾回到开头", "最后提个建议",
	} {
		found := false
		for _, m := range All() {
			if m.Name == n {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("现有的名字 %q 不见了 —— 提示词里引着它", n)
		}
	}
}

// latinFrame 交出这一条里第一句用拉丁字母写的句式。
//
// 判的是**这句话是拿什么文字写的**，不是这一条有没有句式 —— 见
// TestFor_NeverOffersEnglishWordingToAChinesePiece 头上那段。一个 ASCII 字母
// 就够：中文的句式里只有汉字、省略号和句读。
func latinFrame(m Method) (string, bool) {
	for _, p := range m.Patterns {
		for _, r := range p.Frame {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				return p.Frame, true
			}
		}
	}
	return "", false
}

// 每一条 body 的方法都要说清自己属于哪一类；开篇/结尾/英文句式不在这条轴上。
func TestVocabCategoriesAreSet(t *testing.T) {
	// body 这一层上允许的几类。R4 之前只有前两个 —— 那两类回答的是
	// 「这一段怎么证」；analysis 回答的是「例子摆完之后那一句怎么写」，
	// detail 回答的是「记叙文这一段怎么写具体」，都是另一层的问题。
	bodyCategories := map[string]bool{
		"method": true, "structure": true, "analysis": true, "detail": true,
	}
	for _, m := range All() {
		switch m.AppliesTo {
		case "body":
			if m.Lang == "en" {
				continue // 英文句式不在这条轴上
			}
			if !bodyCategories[m.Category] {
				t.Errorf("%s（body）的 category 是 %q，不在允许的几类里", m.ID, m.Category)
			}
		case "whole":
			if m.Category != "structure" {
				t.Errorf("%s（whole）的 category 该是 structure", m.ID)
			}
		}
	}
}

// 🚨 每一种「语言 × 文体」都得有结构可摆。
//
// 同事 2026-09-22 的意见 7：「english writing is quite different from chinese.
// but now we use the same guidance. strange」。2026-09-22 起 Structures 按语言
// 挑，而按语言挑有一个对称的翻车方式：把中文那几条挡住之后，英文那一边
// 什么都没有，行文那一屏的「论证结构」整块不出现 —— 那不是修好，那是少给了
// 一整块。Method.Lang 的注释里写着同一件事的另一半。
func TestStructuresExistForEveryLangAndGenre(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		for _, genre := range []string{GenreArgument, GenreNarrative} {
			got := Structures(genre, lang)
			if len(got) == 0 {
				t.Errorf("%s × %s 一条结构都没有 —— 那一屏会少掉整块", lang, genre)
			}
			for _, m := range got {
				if !speaks(m, lang) {
					t.Errorf("%s × %s 里混进了 %q（lang=%s）", lang, genre, m.Name, m.Lang)
				}
				if !suits(m, genre) {
					t.Errorf("%s × %s 里混进了 %q（genre=%s）", lang, genre, m.Name, m.Genre)
				}
			}
		}
	}
}

// 英文议论文该看到的是英文那一套，不是语文课那四条。
func TestEnglishArgumentStructuresAreTheEnglishOnes(t *testing.T) {
	names := map[string]bool{}
	for _, m := range Structures(GenreArgument, "en") {
		names[m.Name] = true
	}
	for _, zh := range []string{"总分式", "并列式", "层进式", "对照式"} {
		if names[zh] {
			t.Errorf("英文议论文里还摆着语文课的「%s」—— 那正是意见 7 说的那件事", zh)
		}
	}
	// thesis statement 放第一段末尾、单独一段写反方，这两条是英文议论文
	// 和中文议论文真正不同的地方。
	for _, want := range []string{"Thesis-body-conclusion", "Claim-counterargument-refutation"} {
		if !names[want] {
			t.Errorf("英文议论文缺 %q", want)
		}
	}
}

// 句式的两个新字段要真的从 JSON 里读进来。这一条在内容写进去之前会红，
// 那是对的 —— Task 2 才填内容。
func TestPatternCarriesGlossAndExample(t *testing.T) {
	m, ok := ByID("en_concession")
	if !ok {
		t.Fatal("en_concession 不在库里")
	}
	if len(m.Patterns) == 0 {
		t.Fatal("en_concession 一条句式都没有")
	}
	p := m.Patterns[0]
	if p.Gloss == "" {
		t.Errorf("%s 的第一条句式没有中文读法", m.ID)
	}
	if p.Example == "" {
		t.Errorf("%s 的第一条句式没有例句", m.ID)
	}
}
