package litegrade

import (
	"testing"

	"mindimprint/api/internal/liteassign"
)

// 2026-09-18 写作入口走查：两份批改都因为可以不失败的原因失败了。

// 说明里一句转述被加了引号，两次重试都这样，整份批改失败。
func TestUnwrapUnfoundQuotationsKeepsTheWordsDropsTheClaim(t *testing.T) {
	c := validContent()
	c.Points[1].Text = "这里写的是「雨水花园能解决后门积水」，但文中没有说明为什么。"
	in := testInput()
	rs := Check(c, in)
	if !OnlyUnfoundQuotations(rs) {
		t.Fatalf("want only quotation_not_in_body, got %v", codes(rs))
	}
	fixed := UnwrapUnfoundQuotations(c, in)
	if rs := Check(fixed, in); len(rs) != 0 {
		t.Fatalf("still rejected after unwrap: %v", codes(rs))
	}
	if got := fixed.Points[1].Text; got != "这里写的是雨水花园能解决后门积水，但文中没有说明为什么。" {
		t.Fatalf("text = %q", got)
	}
	// A real quotation of hers keeps its marks.
	if fixed.Overall.Comment != c.Overall.Comment {
		t.Fatalf("overall changed: %q", fixed.Overall.Comment)
	}
	// The input is not mutated.
	if c.Points[1].Text == fixed.Points[1].Text {
		t.Fatal("input content was modified")
	}
	// The anchor quote is never repaired.
	c.Points[0].Quote = sp("去年冬天，我在那里摔过一跤。")
	if len(Check(UnwrapUnfoundQuotations(c, in), in)) == 0 {
		t.Fatal("a bad anchor quote must stay a failure")
	}
}

func TestOnlyUnfoundQuotationsNeedsOnlyThatReason(t *testing.T) {
	if OnlyUnfoundQuotations(nil) {
		t.Fatal("no reasons is not 'only quotations'")
	}
	if OnlyUnfoundQuotations([]Reason{{Code: ReasonQuotationNotInBody}, {Code: ReasonPointCount}}) {
		t.Fatal("mixed reasons must not qualify")
	}
}

// 模型在数组里多写了一个引号：`},"{"name"`。
func TestParseRepairsAStrayQuoteBeforeAnObject(t *testing.T) {
	reply := `{"overall":{"grade":"A-","comment":"好"},"dimensions":[{"name":"内容","grade":"A-","comment":"a"},"{"name":"结构","grade":"B+","comment":"b"}],"points":[]}`
	c, err := Parse(reply)
	if err != nil {
		t.Fatalf("not repaired: %v", err)
	}
	if len(c.Dimensions) != 2 || c.Dimensions[1].Name != "结构" {
		t.Fatalf("dimensions = %+v", c.Dimensions)
	}
	// A quote inside a string is untouched: this one parses as-is.
	ok := `{"overall":{"grade":"A","comment":"写了 },\"{ 这几个字"},"dimensions":[],"points":[]}`
	c, err = Parse(ok)
	if err != nil || c.Overall.Comment != `写了 },"{ 这几个字` {
		t.Fatalf("valid reply changed: %q %v", c.Overall.Comment, err)
	}
	// Still broken after the repair → the original error.
	if _, err := Parse(`{"overall":{"grade":"A"},"dimensions":[{"name":"a"} {"name":"b"}]}`); err == nil {
		t.Fatal("broken JSON accepted")
	}
}

// 模型回了 6 条意见，整份批改失败。
func TestNormalizeAICapsPointsKeepingAGoodAndAnIssue(t *testing.T) {
	c := validContent()
	issue := c.Points[1]
	good := c.Points[0]
	c.Points = []Point{issue, issue, issue, issue, issue, good}
	out := NormalizeAI(c, liteassign.DefaultRubric("zh"))
	if len(out.Points) != MaxPoints {
		t.Fatalf("points = %d, want %d", len(out.Points), MaxPoints)
	}
	if out.Points[len(out.Points)-1].Kind != KindGood {
		t.Fatalf("the good point was dropped: %+v", out.Points)
	}
	if rs := Check(out, testInput()); hasCode(rs, ReasonPointCount) || hasCode(rs, ReasonNoGoodPoint) {
		t.Fatalf("capped content still rejected: %v", codes(rs))
	}
	// A teacher's edit is never capped.
	if n := len(NormalizeTeacher(c, liteassign.DefaultRubric("zh")).Points); n != 6 {
		t.Fatalf("teacher points = %d, want 6", n)
	}
}
