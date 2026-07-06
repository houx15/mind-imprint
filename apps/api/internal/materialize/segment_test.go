package materialize

import "testing"

func TestSegmentSplitsParagraphsAndDropsBlanks(t *testing.T) {
	in := "第一段。\n\n  \n第二段，有点长。\n\n\n第三段。"
	got := Segment(in)
	if len(got) != 3 {
		t.Fatalf("want 3 blocks, got %d: %+v", len(got), got)
	}
	if got[0].ID != "b0" || got[1].ID != "b1" || got[2].ID != "b2" {
		t.Fatalf("ids not sequential: %+v", got)
	}
	if got[0].Text != "第一段。" || got[2].Text != "第三段。" {
		t.Fatalf("unexpected text: %+v", got)
	}
}

func TestSegmentCollapsesInternalWhitespace(t *testing.T) {
	got := Segment("a   b\n c\td")
	if len(got) != 1 || got[0].Text != "a b c d" {
		t.Fatalf("want single collapsed block 'a b c d', got %+v", got)
	}
}

func TestSegmentEmptyInputYieldsNone(t *testing.T) {
	if got := Segment("   \n\n  "); len(got) != 0 {
		t.Fatalf("want 0 blocks, got %+v", got)
	}
}
