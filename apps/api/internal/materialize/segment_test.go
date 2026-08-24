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
	// Multiple spaces and tabs within one line collapse to single spaces.
	got := Segment("a   b\tc")
	if len(got) != 1 || got[0].Text != "a b c" {
		t.Fatalf("want single collapsed block 'a b c', got %+v", got)
	}
}

// Paste with paragraphs separated by SINGLE newlines and no blank lines (the
// common copy-out-of-a-PDF/Word/chat shape) must become one block per
// paragraph, not one giant wall block. Regression guard for the reading-room
// paste bug.
func TestSegmentSingleNewlineParagraphs(t *testing.T) {
	in := "First paragraph, one full sentence of it.\nSecond paragraph here.\nThird and final paragraph."
	got := Segment(in)
	if len(got) != 3 {
		t.Fatalf("want 3 blocks from single-newline paste, got %d: %+v", len(got), got)
	}
	if got[0].Text != "First paragraph, one full sentence of it." || got[2].Text != "Third and final paragraph." {
		t.Fatalf("unexpected block text: %+v", got)
	}
}

// When blank lines DO exist, they remain the boundary — a paragraph hard-wrapped
// with internal single newlines stays one block (its newlines collapse to
// spaces), and the PDF-page / extracted-HTML shape is unaffected.
func TestSegmentBlankLineBoundaryWins(t *testing.T) {
	in := "Line one of para one\nwrapped onto a second line.\n\nParagraph two stands alone."
	got := Segment(in)
	if len(got) != 2 {
		t.Fatalf("want 2 blocks (blank line is the boundary), got %d: %+v", len(got), got)
	}
	if got[0].Text != "Line one of para one wrapped onto a second line." {
		t.Fatalf("para one should collapse its internal newline: %+v", got)
	}
}

func TestSegmentEmptyInputYieldsNone(t *testing.T) {
	if got := Segment("   \n\n  "); len(got) != 0 {
		t.Fatalf("want 0 blocks, got %+v", got)
	}
}
