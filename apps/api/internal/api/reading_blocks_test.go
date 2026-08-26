package api_test

import (
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
)

func TestSplitBlocks_ParagraphsGetStableIDs(t *testing.T) {
	blocks := SplitBlocks("第一段。\n\n第二段，稍长一些。\n\n\n第三段。")
	if len(blocks) != 3 {
		t.Fatalf("got %d blocks, want 3: %+v", len(blocks), blocks)
	}
	if blocks[0].ID != "b1" || blocks[1].ID != "b2" || blocks[2].ID != "b3" {
		t.Fatalf("ids = %s/%s/%s, want b1/b2/b3", blocks[0].ID, blocks[1].ID, blocks[2].ID)
	}
	if blocks[1].Text != "第二段，稍长一些。" {
		t.Fatalf("block 2 text = %q", blocks[1].Text)
	}
}

func TestSplitBlocks_IgnoresBlankAndTrims(t *testing.T) {
	blocks := SplitBlocks("  \n\n  正文  \n\n   \n\n 尾段 \n")
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2: %+v", len(blocks), blocks)
	}
	if blocks[0].Text != "正文" || blocks[1].Text != "尾段" {
		t.Fatalf("blocks = %+v, want trimmed text", blocks)
	}
}

func TestSplitBlocks_EmptyBodyYieldsNone(t *testing.T) {
	if got := SplitBlocks("   \n\n  "); len(got) != 0 {
		t.Fatalf("got %d blocks, want 0", len(got))
	}
}

// Block ids must not shift when the body is re-split — a hanging card stores
// its block id, so renumbering would move every card to the wrong paragraph.
func TestSplitBlocks_IsDeterministic(t *testing.T) {
	body := strings.Repeat("一段。\n\n", 5)
	a, b := SplitBlocks(body), SplitBlocks(body)
	if len(a) != len(b) {
		t.Fatalf("lengths differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Text != b[i].Text {
			t.Fatalf("block %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}
