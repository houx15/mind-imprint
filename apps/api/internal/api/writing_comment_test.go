package api

import "testing"

// The quote IS the trace. A point whose quote is not literally in her text would
// send her to a sentence she never wrote — "specific, confident and wrong", the
// one failure mode this room refuses. Drop it; never guess.
func TestValidateCommentPoints_DropsQuotesSheNeverWrote(t *testing.T) {
	source := "我家楼下那条路就是这样，两排树掘得密密麻麻，长了几年也还是瘦瘦的一根杆。"
	in := []CommentPoint{
		{Text: "这个例子是过密，不是数量多。", Quote: "两排树掘得密密麻麻"},
		{Text: "这里缺出处。", Quote: "根据 2019 年的一项研究"},
		{Text: "空引用。", Quote: ""},
	}
	got := validateCommentPoints(in, source)
	if len(got) != 1 {
		t.Fatalf("kept %d points %v, want only the one whose quote is really in her text", len(got), got)
	}
	if got[0].Quote != "两排树掘得密密麻麻" {
		t.Fatalf("kept the wrong point: %+v", got[0])
	}
}

func TestValidateCommentPoints_KeepsEveryRealQuote(t *testing.T) {
	source := "第一句。第二句。第三句。"
	in := []CommentPoint{{Text: "a", Quote: "第一句。"}, {Text: "b", Quote: "第三句。"}}
	if got := validateCommentPoints(in, source); len(got) != 2 {
		t.Fatalf("kept %d, want 2", len(got))
	}
}
