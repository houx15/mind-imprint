package litegrade

import (
	"strings"
	"testing"

	"mindimprint/api/internal/liteassign"
)

const testBody = "学校后门那片空地一下雨就积水。去年秋天，我在那里摔过一跤。\n\n我读到城市里的雨水花园：用下凹的绿地先把雨水接住。"
const testPrompt = "写一篇关于校园积水的议论文，说明雨水花园是否适合学校。"

func sp(s string) *string { return &s }

func testInput() Input {
	return Input{
		Lang: "zh", Title: "雨水去哪儿了", Body: testBody, AssignedPrompt: testPrompt,
		VersionNumber: 1, Rubric: liteassign.DefaultRubric("zh"),
		PersonJudging: func(s string) bool { return strings.Contains(s, "你很") },
	}
}

func validContent() Content {
	return Content{
		Overall: Overall{Grade: "B+", Comment: "用「去年秋天，我在那里摔过一跤。」引出问题。"},
		Dimensions: []Dimension{
			{Name: "内容", Grade: "B+", Comment: "问题来自亲身经历。"},
			{Name: "结构", Grade: "B", Comment: "两段之间没有过渡句。"},
			{Name: "语言", Grade: "A-", Comment: "表达清楚。"},
			{Name: "书写规范", Grade: "A", Comment: "标点使用正确。"},
		},
		Points: []Point{
			{Kind: KindGood, Quote: sp("去年秋天，我在那里摔过一跤。"), Text: "用具体经历引出问题。", Source: SourceAI},
			{Kind: KindIssue, Quote: sp("我读到城市里的雨水花园：用下凹的绿地先把雨水接住。"), Text: "材料与后门空地之间没有说明联系。", Action: sp("在这句后面写一句说明雨水花园和后门空地的关系。"), Source: SourceAI},
			{Kind: KindIssue, Quote: sp("学校后门那片空地一下雨就积水。"), Text: "积水的程度没有数据。", Action: sp("补充一次积水的深度或持续时间。"), Source: SourceAI},
		},
	}
}

func codes(rs []Reason) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Code)
	}
	return out
}

func hasCode(rs []Reason, code string) bool {
	for _, r := range rs {
		if r.Code == code {
			return true
		}
	}
	return false
}

func TestCheckTeacherDraftAllowsPartialButRejectsInvalidEvidence(t *testing.T) {
	in := testInput()
	c := BlankContent(in.Rubric)
	c.Overall.Comment = "待补充"
	c.Points = []Point{{Kind: KindIssue, Text: "", Quote: nil}}
	if rs := CheckTeacherDraft(c, in); len(rs) != 0 {
		t.Fatalf("partial draft rejected: %v", codes(rs))
	}
	if !hasCode(CheckTeacherEdit(c, in), ReasonGradeOutOfScale) {
		t.Fatal("incomplete draft passed send check")
	}
	c.Points[0].Quote = sp("这句没有出现在原文")
	if !hasCode(CheckTeacherDraft(c, in), ReasonQuoteNotInBody) {
		t.Fatal("fabricated quotation accepted")
	}
	c.Points[0].Quote = nil
	c.Dimensions[0].Grade = "E"
	if !hasCode(CheckTeacherDraft(c, in), ReasonGradeOutOfScale) {
		t.Fatal("invalid grade accepted")
	}
}

func TestCheckAcceptsValidContent(t *testing.T) {
	if rs := Check(validContent(), testInput()); len(rs) != 0 {
		t.Fatalf("valid content rejected: %v", codes(rs))
	}
	// Dimensions in another order are the same rubric.
	c := validContent()
	c.Dimensions[0], c.Dimensions[3] = c.Dimensions[3], c.Dimensions[0]
	if rs := Check(c, testInput()); len(rs) != 0 {
		t.Fatalf("reordered dimensions rejected: %v", codes(rs))
	}
}

func TestCheckRejections(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(c *Content, in *Input)
		code   string
	}{
		{"quote not a substring", func(c *Content, _ *Input) { c.Points[0].Quote = sp("去年冬天，我在那里摔过一跤。") }, ReasonQuoteNotInBody},
		{"quote missing", func(c *Content, _ *Input) { c.Points[1].Quote = nil }, ReasonQuoteMissing},
		{"quote blank", func(c *Content, _ *Input) { c.Points[1].Quote = sp("  ") }, ReasonQuoteMissing},
		{"quote only in the assigned prompt", func(c *Content, _ *Input) { c.Points[1].Quote = sp("说明雨水花园是否适合学校") }, ReasonQuoteFromPrompt},
		{"quotation in overall comment not hers", func(c *Content, _ *Input) { c.Overall.Comment = "可以改成「雨水从后门流走了」。" }, ReasonQuotationNotInBody},
		{"quotation in dimension comment not hers", func(c *Content, _ *Input) {
			c.Dimensions[1].Comment = "第二段开头写「因此学校需要雨水花园」会更顺。"
		}, ReasonQuotationNotInBody},
		{"quotation in point text not hers", func(c *Content, _ *Input) { c.Points[1].Text = "这句应该是「雨水花园能解决积水」。" }, ReasonQuotationNotInBody},
		{"quotation in action not hers", func(c *Content, _ *Input) {
			c.Points[2].Action = sp("把这句改成「后门空地每次积水十厘米」。")
		}, ReasonQuotationNotInBody},
		{"quotation of the assigned prompt", func(c *Content, _ *Input) {
			c.Points[2].Text = "没有回应「说明雨水花园是否适合学校」。"
		}, ReasonQuoteFromPrompt},
		{"overall grade outside the scale", func(c *Content, _ *Input) { c.Overall.Grade = "E" }, ReasonGradeOutOfScale},
		{"dimension grade outside the scale", func(c *Content, _ *Input) { c.Dimensions[2].Grade = "A++" }, ReasonGradeOutOfScale},
		{"points grade above max", func(c *Content, in *Input) {
			in.Rubric = liteassign.Rubric{Scale: liteassign.ScalePoints, Max: 20, Dimensions: []liteassign.RubricDimension{{Name: "论证"}}}
			c.Overall.Grade = "21"
			c.Dimensions = []Dimension{{Name: "论证", Grade: "15", Comment: "有证据。"}}
		}, ReasonGradeOutOfScale},
		{"dimension renamed", func(c *Content, _ *Input) { c.Dimensions[0].Name = "立意" }, ReasonDimensionNames},
		{"dimension missing", func(c *Content, _ *Input) { c.Dimensions = c.Dimensions[:3] }, ReasonDimensionNames},
		{"dimension added", func(c *Content, _ *Input) {
			c.Dimensions = append(c.Dimensions, Dimension{Name: "创意", Grade: "A", Comment: "有新意。"})
		}, ReasonDimensionNames},
		{"six points", func(c *Content, _ *Input) {
			for len(c.Points) < 6 {
				c.Points = append(c.Points, c.Points[2])
			}
		}, ReasonPointCount},
		{"issue without action", func(c *Content, _ *Input) { c.Points[1].Action = nil }, ReasonIssueWithoutAction},
		{"issue with blank action", func(c *Content, _ *Input) { c.Points[2].Action = sp("   ") }, ReasonIssueWithoutAction},
		{"point judges her as a person", func(c *Content, _ *Input) { c.Points[1].Text = "你很粗心，没有检查。" }, ReasonPersonJudging},
		{"overall comment judges her", func(c *Content, _ *Input) { c.Overall.Comment = "你很懒。" }, ReasonPersonJudging},
		{"unknown point kind", func(c *Content, _ *Input) { c.Points[2].Kind = "note" }, ReasonPointKind},
		{"empty point text", func(c *Content, _ *Input) { c.Points[1].Text = " " }, ReasonEmptyText},
		{"empty overall comment", func(c *Content, _ *Input) { c.Overall.Comment = "" }, ReasonEmptyText},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, in := validContent(), testInput()
			tc.mutate(&c, &in)
			rs := Check(c, in)
			if !hasCode(rs, tc.code) {
				t.Fatalf("codes = %v, want %s", codes(rs), tc.code)
			}
			for _, r := range rs {
				if r.Message() == "" || r.Message() == r.Code {
					t.Fatalf("reason %s has no message", r.Code)
				}
			}
		})
	}
}

func TestCheckPointsScaleAccepts(t *testing.T) {
	c, in := validContent(), testInput()
	in.Rubric = liteassign.Rubric{Scale: liteassign.ScalePoints, Max: 20, Dimensions: []liteassign.RubricDimension{{Name: "论证"}}}
	c.Overall.Grade = "20"
	c.Dimensions = []Dimension{{Name: "论证", Grade: "0", Comment: "没有证据。"}}
	if rs := Check(c, in); len(rs) != 0 {
		t.Fatalf("points content rejected: %v", codes(rs))
	}
}

func TestCheckTeacherEditIsShapeOnly(t *testing.T) {
	in := testInput()
	c := validContent()
	// One teacher point with no quote and no action: fine for a teacher.
	c.Points = []Point{{Kind: KindIssue, Text: "第二段请补充数据来源。", Source: SourceTeacher}}
	c.Overall.Comment = "可以写「雨水花园」的来源。"
	if rs := CheckTeacherEdit(c, in); len(rs) != 0 {
		t.Fatalf("teacher edit rejected: %v", codes(rs))
	}
	bad := []struct {
		name   string
		mutate func(c *Content)
		code   string
	}{
		{"grade out of scale", func(c *Content) { c.Overall.Grade = "E" }, ReasonGradeOutOfScale},
		{"dimension names", func(c *Content) { c.Dimensions[0].Name = "立意" }, ReasonDimensionNames},
		{"teacher quote not a substring", func(c *Content) { c.Points[0].Quote = sp("雨一直下。") }, ReasonQuoteNotInBody},
		{"empty point text", func(c *Content) { c.Points[0].Text = "" }, ReasonEmptyText},
		{"kind", func(c *Content) { c.Points[0].Kind = "praise" }, ReasonPointKind},
	}
	for _, tc := range bad {
		cc := c
		cc.Points = append([]Point(nil), c.Points...)
		cc.Dimensions = append([]Dimension(nil), c.Dimensions...)
		tc.mutate(&cc)
		if rs := CheckTeacherEdit(cc, in); !hasCode(rs, tc.code) {
			t.Errorf("%s: codes = %v, want %s", tc.name, codes(rs), tc.code)
		}
	}
}

func TestJoinReasonsDedupes(t *testing.T) {
	rs := []Reason{{Code: ReasonNoGoodPoint}, {Code: ReasonNoGoodPoint}, {Code: ReasonPointCount, Detail: "2"}}
	if got := JoinReasons(rs); got != "没有优点意见；意见共 2 条，最多允许 5 条" {
		t.Fatalf("JoinReasons = %q", got)
	}
}

// --- Controller ruling 2026-09-15: feedback must be written in the writing's
// language. Quoted spans of her text (「」) are excluded from the count, so a
// Chinese quote inside an otherwise-English sentence does not save it.

func englishFiller(n int) string {
	words := []string{"clear", "topic", "essay", "point", "reader", "detail", "order", "focus"}
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(words[i%len(words)])
	}
	return b.String()
}

func TestCheckLanguageMismatchZhWritingEnglishFeedback(t *testing.T) {
	c, in := validContent(), testInput() // in.Lang == "zh"
	c.Overall.Comment = "This overall comment is written entirely in " + englishFiller(6) + "."
	for i := range c.Dimensions {
		c.Dimensions[i].Comment = "This dimension comment is also in " + englishFiller(6) + "."
	}
	for i := range c.Points {
		c.Points[i].Text = "This point text is in " + englishFiller(6) + " as well."
		if c.Points[i].Action != nil {
			c.Points[i].Action = sp("Do this next, written in " + englishFiller(6) + ".")
		}
	}
	rs := Check(c, in)
	if !hasCode(rs, ReasonLanguageMismatch) {
		t.Fatalf("codes = %v, want %s", codes(rs), ReasonLanguageMismatch)
	}
}

func TestCheckLanguageMismatchEnWritingChineseFeedback(t *testing.T) {
	c, in := validContent(), testInput()
	in.Lang = "en"
	in.Body = "The vacant lot behind the school floods every time it rains. Last fall I fell there."
	c.Overall.Comment = "这段总评完全用中文写成，用来测试语言检查是否生效。"
	for i := range c.Dimensions {
		c.Dimensions[i].Comment = "这条维度评语也完全用中文写成，用来测试语言检查。"
	}
	for i := range c.Points {
		c.Points[i].Quote = nil // the English body no longer contains her Chinese quotes
		c.Points[i].Text = "这条意见的说明完全用中文写成，用来测试语言检查是否生效。"
		if c.Points[i].Action != nil {
			c.Points[i].Action = sp("这条修改建议也完全用中文写成，用来测试语言检查。")
		}
	}
	rs := Check(c, in)
	if !hasCode(rs, ReasonLanguageMismatch) {
		t.Fatalf("codes = %v, want %s", codes(rs), ReasonLanguageMismatch)
	}
}

func TestCheckLanguageQuotesExcludedFromCount(t *testing.T) {
	// Her body contains a long English sentence (a quoted source, say); a zh
	// comment quotes that sentence in 「」 and adds a short Chinese remark.
	// Counting the quote would make English win and reject a comment that is
	// otherwise entirely in Chinese; stripping the quote first is the only
	// way this passes. (This is what distinguishes this test from a case
	// that would pass either way: without quotematch.StripQuotedSpans in
	// languageReason, this fails — see the fix report for the RED run.)
	englishQuote := "Rainwater gardens reduce flooding by capturing runoff before it reaches the drains"
	in := testInput()
	in.Body = "学校后门那片空地一下雨就积水。" + englishQuote
	c := Content{Overall: Overall{Comment: "「" + englishQuote + "」这点很好。"}}
	rs := Check(c, in)
	if hasCode(rs, ReasonLanguageMismatch) {
		t.Fatalf("a long quoted sentence of hers should not count toward the model's own prose: %v", codes(rs))
	}
}

// --- Controller ruling 2026-09-15, fix round 1: the quote and 「」 checks
// were too strict for real model output. A term or a symptom-catalog name
// in 「」, or a quote that differs from her body only by punctuation width,
// an extra 。, or the paragraph break the model didn't retype, must not fail
// the whole grading; a genuinely invented quotation still must.

func TestCheckAcceptsShortQuotedTermsAndSymptomNames(t *testing.T) {
	cases := []struct{ name, text string }{
		{"a term", "缺少「让步」段落。"},
		{"a symptom-catalog name", "属于「立意不清」的问题。"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, in := validContent(), testInput()
			c.Points[1].Text = tc.text
			rs := Check(c, in)
			if hasCode(rs, ReasonQuotationNotInBody) {
				t.Fatalf("a short quoted span should be skipped, not checked: %v", codes(rs))
			}
		})
	}
}

func TestCheckAcceptsQuotesThatDifferOnlyByNormalizedPunctuationOrSpacing(t *testing.T) {
	cases := []struct{ name, quote string }{
		{"half-width comma instead of full-width", "去年秋天,我在那里摔过一跤。"},
		{"an extra trailing 。", "去年秋天，我在那里摔过一跤。。"},
		{"spans the paragraph break in the source", "去年秋天，我在那里摔过一跤。我读到城市里的雨水花园"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, in := validContent(), testInput()
			c.Points[0].Quote = sp(tc.quote)
			rs := Check(c, in)
			if hasCode(rs, ReasonQuoteNotInBody) {
				t.Fatalf("a quote that is hers once normalized was rejected: %v", codes(rs))
			}
		})
	}
}

func TestCheckStillRejectsAGenuinelyInventedQuotation(t *testing.T) {
	c, in := validContent(), testInput()
	c.Points[1].Text = "没有回应「这句话是彻底编造出来的内容」。" // 13 runes, in neither body nor prompt
	rs := Check(c, in)
	if !hasCode(rs, ReasonQuotationNotInBody) {
		t.Fatalf("an invented long quotation should still be rejected: %v", codes(rs))
	}
}

func TestCheckStillRejectsALongCurlyQuoteRewrite(t *testing.T) {
	c, in := validContent(), testInput()
	c.Points[1].Text = "这句话应该改成“这也是彻底编造出来的一句话”。" // 13 runes, curly quotes, invented
	rs := Check(c, in)
	if !hasCode(rs, ReasonQuotationNotInBody) {
		t.Fatalf("a long invented “” rewrite should still be rejected: %v", codes(rs))
	}
}

// --- Controller ruling 2026-09-15, fix round 2: a catalog symptom name
// quoted with “” (not just 「」 — mainland zh models favour “”) was still
// being rejected when it normalized to 8+ runes, because only length gated
// the check, not catalog membership. Also: a Point.Quote that is nothing
// but punctuation ("……", "?!") normalized to "" and matched every body
// trivially — fixed to report ReasonQuoteMissing instead.

func zhSymptomCatalog() string {
	return "【第 1 层 · 立意】\n" +
		"- topic_without_question（只有主题，没有问题）：材料围绕一个大词堆积；读完不知道你想说的是哪一件事。\n" +
		"- evidence_not_explained（举了例子，没有解释）：例子摆在那里就过去了，没有一句话说清它凭什么支持你的判断。\n"
}

func enSymptomCatalog() string {
	return "【第 1 层 · 立意】\n" +
		"- task_instruction_coverage（没有回答题目问的那件事）：题目要求的动作有一半没做，或者答的是另一个范围。\n" +
		"【第 3 层 · 结构】\n" +
		"- paragraph_function_order（句子的角色和顺序乱了）：一段里主张、证据、解释的先后颠倒。\n"
}

func TestCheckAcceptsCatalogSymptomNameInCurlyQuotes(t *testing.T) {
	// The reviewer's exact probe: 属于"只有主题，没有问题" — a zh catalog
	// name quoted with “”, normalizing to 8 runes (at the MinRunes floor,
	// so the length skip alone doesn't save it; the catalog check does).
	c, in := validContent(), testInput()
	in.SymptomCatalog = zhSymptomCatalog()
	c.Points[1].Text = "属于“只有主题，没有问题”。"
	rs := Check(c, in)
	if hasCode(rs, ReasonQuotationNotInBody) {
		t.Fatalf("a catalog symptom name in “” was rejected: %v", codes(rs))
	}
}

func TestCheckAcceptsEnCatalogLabelInCornerQuotes(t *testing.T) {
	c, in := validContent(), testInput()
	in.SymptomCatalog = enSymptomCatalog()
	c.Points[1].Text = "属于「没有回答题目问的那件事」的问题。" // 11 runes
	rs := Check(c, in)
	if hasCode(rs, ReasonQuotationNotInBody) {
		t.Fatalf("an en-catalog label in 「」 was rejected: %v", codes(rs))
	}
}

func TestCheckStillRejectsAnInventedQuotationNotInTheCatalog(t *testing.T) {
	// The catalog is present (so this isn't passing merely because there is
	// no catalog to check against) but the quoted text names nothing in it.
	c, in := validContent(), testInput()
	in.SymptomCatalog = zhSymptomCatalog()
	c.Points[1].Text = "没有回应「这句话是彻底编造出来的内容」。" // not in body, prompt, or catalog
	rs := Check(c, in)
	if !hasCode(rs, ReasonQuotationNotInBody) {
		t.Fatalf("an invented quotation absent from the catalog should still be rejected: %v", codes(rs))
	}
}

func TestCheckPunctuationOnlyQuoteIsMissingNotAccepted(t *testing.T) {
	cases := []string{"……", "?!", "。。。"}
	for _, q := range cases {
		t.Run(q, func(t *testing.T) {
			c, in := validContent(), testInput()
			c.Points[0].Quote = sp(q)
			rs := Check(c, in)
			if !hasCode(rs, ReasonQuoteMissing) {
				t.Fatalf("a punctuation-only quote should read as missing: %v", codes(rs))
			}
			if hasCode(rs, ReasonQuoteNotInBody) {
				t.Fatalf("a punctuation-only quote should not be reported as merely not-in-body: %v", codes(rs))
			}
		})
	}
}

func TestCheckLanguageThresholdBoundary(t *testing.T) {
	// languageThreshold is 0.6: prose that is exactly 60% Han passes, prose
	// that is 50% Han (just under) is rejected. Only the overall comment is
	// set, so the ratio is exact.
	atThreshold := Content{Overall: Overall{Comment: strings.Repeat("中", 6) + strings.Repeat("x", 4)}} // 6/10 = 0.60
	if rs := Check(atThreshold, testInput()); hasCode(rs, ReasonLanguageMismatch) {
		t.Fatalf("60%% Han prose rejected at the threshold: %v", codes(rs))
	}

	belowThreshold := Content{Overall: Overall{Comment: strings.Repeat("中", 5) + strings.Repeat("x", 5)}} // 5/10 = 0.50
	if rs := Check(belowThreshold, testInput()); !hasCode(rs, ReasonLanguageMismatch) {
		t.Fatalf("50%% Han prose accepted just under the threshold: %v", codes(rs))
	}
}

// Feedback need not invent strengths or problems to meet a category quota.
// Grades, dimension comments, quotations and action checks still apply.
func TestCheckAcceptsEvidenceDrivenPointCounts(t *testing.T) {
	base := validContent()
	for _, points := range [][]Point{{}, {base.Points[0]}, {base.Points[1]}, base.Points[:2]} {
		c := validContent()
		c.Points = points
		if rs := Check(c, testInput()); len(rs) != 0 {
			t.Fatalf("valid %d-point feedback rejected: %v", len(points), rs)
		}
		c.Overall.Grade = "INVALID"
		if rs := Check(c, testInput()); !hasCode(rs, ReasonGradeOutOfScale) {
			t.Fatal("point-count flexibility bypassed grading checks")
		}
	}
	c := validContent()
	c.Points = []Point{c.Points[1]}
	c.Points[0].Action = nil
	if rs := Check(c, testInput()); !hasCode(rs, ReasonIssueWithoutAction) {
		t.Fatal("single issue must retain an action")
	}
	c = validContent()
	c.Points = []Point{c.Points[0]}
	c.Points[0].Quote = sp("正文里不存在的句子。")
	if rs := Check(c, testInput()); !hasCode(rs, ReasonQuoteNotInBody) {
		t.Fatal("praise-only feedback must retain source evidence")
	}
}

// --- SanitizeProvenance: 2026-09-23, points[].dimension / .symptom — the
// modal that tells the teacher where one point came from.

// testSymptomLookup mirrors internal/api's resolveWritingSymptom: it takes an
// id OR a display name and always answers with the id.
func testSymptomLookup(v string) (string, string, bool) {
	for _, row := range [][2]string{
		{"topic_without_question", "只有主题，没有问题"},
		{"evidence_not_explained", "举了例子，没有解释"},
	} {
		if v == row[0] || v == row[1] {
			return row[0], row[1], true
		}
	}
	return "", "", false
}

func TestSanitizeProvenanceKeepsAMatchingDimension(t *testing.T) {
	c := Content{Points: []Point{{Kind: KindGood, Dimension: "内容"}}}
	out := SanitizeProvenance(c, testInput())
	if out.Points[0].Dimension != "内容" {
		t.Fatalf("a dimension matching the rubric must survive, got %q", out.Points[0].Dimension)
	}
}

func TestSanitizeProvenanceClearsAnUnknownDimensionButKeepsThePoint(t *testing.T) {
	c := Content{Points: []Point{{Kind: KindGood, Text: "真实的一条意见", Dimension: "论证深度"}}}
	out := SanitizeProvenance(c, testInput())
	if len(out.Points) != 1 {
		t.Fatalf("an unrecognised dimension must not drop the point, got %d points", len(out.Points))
	}
	if out.Points[0].Dimension != "" {
		t.Fatalf("dimension not in the rubric must be cleared, got %q", out.Points[0].Dimension)
	}
	if out.Points[0].Text != "真实的一条意见" {
		t.Fatalf("the rest of the point must be untouched, got %+v", out.Points[0])
	}
}

func TestSanitizeProvenanceStoresTheSymptomID(t *testing.T) {
	in := testInput()
	in.SymptomLookup = testSymptomLookup
	c := Content{Points: []Point{{Kind: KindIssue, Symptom: "topic_without_question"}}}
	out := SanitizeProvenance(c, in)
	if out.Points[0].Symptom != "topic_without_question" {
		t.Fatalf("the id is what gets stored, got %q", out.Points[0].Symptom)
	}
}

// 🚨 **It runs twice on the same value.** Once at generation, once on every
// teacher PATCH — the client sends the whole content back. The first version
// stored the display NAME and matched on id only, so the second run blanked
// 对应毛病 on every single save (2026-09-23, proven by direct execution).
func TestSanitizeProvenanceIsIdempotent(t *testing.T) {
	in := testInput()
	in.SymptomLookup = testSymptomLookup
	once := SanitizeProvenance(Content{Points: []Point{
		{Kind: KindIssue, Dimension: "内容", Symptom: "topic_without_question"},
	}}, in)
	twice := SanitizeProvenance(once, in)
	if twice.Points[0].Symptom != once.Points[0].Symptom {
		t.Fatalf("second pass changed symptom: %q -> %q", once.Points[0].Symptom, twice.Points[0].Symptom)
	}
	if twice.Points[0].Symptom == "" {
		t.Fatal("a second pass blanked the symptom — this is the teacher-save bug")
	}
	if twice.Points[0].Dimension != "内容" {
		t.Fatalf("second pass changed dimension, got %q", twice.Points[0].Dimension)
	}
}

// A client echoing back the DISPLAY NAME (what the teacher's view renders,
// or a row stored before 2026-09-23) is upgraded to the id, never blanked.
func TestSanitizeProvenanceAcceptsADisplayNameAndCanonicalisesIt(t *testing.T) {
	in := testInput()
	in.SymptomLookup = testSymptomLookup
	out := SanitizeProvenance(Content{Points: []Point{{Kind: KindIssue, Symptom: "只有主题，没有问题"}}}, in)
	if out.Points[0].Symptom != "topic_without_question" {
		t.Fatalf("a display name must canonicalise to the id, got %q", out.Points[0].Symptom)
	}
}

func TestSanitizeProvenanceClearsAnUnknownSymptomButKeepsThePoint(t *testing.T) {
	in := testInput()
	in.SymptomLookup = testSymptomLookup
	c := Content{Points: []Point{{Kind: KindIssue, Text: "真实的一条意见", Symptom: "made_up_id"}}}
	out := SanitizeProvenance(c, in)
	if len(out.Points) != 1 {
		t.Fatalf("an unrecognised symptom must not drop the point, got %d points", len(out.Points))
	}
	if out.Points[0].Symptom != "" {
		t.Fatalf("an id absent from the closed table must be cleared, got %q", out.Points[0].Symptom)
	}
	if out.Points[0].Text != "真实的一条意见" {
		t.Fatalf("the rest of the point must be untouched, got %+v", out.Points[0])
	}
}

func TestSanitizeProvenanceClearsSymptomWhenLookupIsNil(t *testing.T) {
	in := testInput()
	in.SymptomLookup = nil
	c := Content{Points: []Point{{Kind: KindIssue, Symptom: "topic_without_question"}}}
	out := SanitizeProvenance(c, in)
	if out.Points[0].Symptom != "" {
		t.Fatalf("no SymptomLookup must clear rather than guess, got %q", out.Points[0].Symptom)
	}
}

func TestSanitizeProvenanceLeavesBlankFieldsBlank(t *testing.T) {
	in := testInput()
	in.SymptomLookup = testSymptomLookup
	c := Content{Points: []Point{{Kind: KindGood}}}
	out := SanitizeProvenance(c, in)
	if out.Points[0].Dimension != "" || out.Points[0].Symptom != "" {
		t.Fatalf("a point with no provenance must not gain one, got %+v", out.Points[0])
	}
}
