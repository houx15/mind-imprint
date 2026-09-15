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
		{"two points", func(c *Content, _ *Input) { c.Points = c.Points[:2] }, ReasonPointCount},
		{"six points", func(c *Content, _ *Input) {
			for len(c.Points) < 6 {
				c.Points = append(c.Points, c.Points[2])
			}
		}, ReasonPointCount},
		{"no good point", func(c *Content, _ *Input) { c.Points[0] = c.Points[1] }, ReasonNoGoodPoint},
		{"no issue point", func(c *Content, _ *Input) {
			c.Points[1] = c.Points[0]
			c.Points[2] = c.Points[0]
		}, ReasonNoIssuePoint},
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
	if got := JoinReasons(rs); got != "没有优点意见；意见共 2 条，需要 3 到 5 条" {
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
	// A single Chinese quote inside an otherwise-English sentence must not
	// count toward "mostly Han": the quote is hers, the sentence around it
	// is the model's own prose and that prose is what the check judges. Only
	// the overall comment is set, so it is the sole contributor to the count.
	c := Content{Overall: Overall{
		Comment: "The line 「去年秋天，我在那里摔过一跤。」 opens with a concrete moment, written up in " + englishFiller(10) + ".",
	}}
	rs := Check(c, testInput())
	if !hasCode(rs, ReasonLanguageMismatch) {
		t.Fatalf("a Chinese quote inside English prose still failed to trip the check: %v", codes(rs))
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
