package liteworkspace

import (
	"strings"
	"testing"
)

// The passage from the 2026-09-17 live run (scenario 2): it carries curly
// quotes, and the model sent them back as straight quotes every time.
const anchorPassage = `一位负责维护的工人说：“铺的时候大家都很积极，后来没人管，效果就慢慢没了。”` +
	`所以，一座城市能不能真正“吸水”，不只取决于用了什么材料，还取决于有没有人长期维护。`

func TestAnchoredSpanFoldsQuotesAndStoresHerRunes(t *testing.T) {
	typed := "这次的阅读材料就用我贴的这段，请把它设为材料：\n\n" + anchorPassage
	got, err := AnchoredSpan(typed,
		`一位负责维护的工人说："铺的时候`, // straight quote where she typed “
		`不能真正"吸水"，不只取决于用了什么材料，还取决于有没有人长期维护。`)
	if err != nil {
		t.Fatalf("AnchoredSpan: %v", err)
	}
	if got != anchorPassage {
		t.Fatalf("span = %q\nwant her original passage %q", got, anchorPassage)
	}
	if !strings.Contains(got, "“") || strings.Contains(got, `"`) {
		t.Fatalf("span must keep her curly quotes, got %q", got)
	}
	if !strings.Contains(typed, got) {
		t.Fatal("span must be a literal substring of what she typed")
	}
}

func TestAnchoredSpanLongPassage(t *testing.T) {
	var b strings.Builder
	b.WriteString("开头第一句是这篇文章的起点。")
	for b.Len() < 9000 { // about 3,000 runes of three-byte text
		b.WriteString("城市的排水系统需要长期维护，透水砖、下沉绿地和雨水花园各有用途。")
	}
	b.WriteString("最后一句是这篇文章的终点。")
	passage := b.String()
	if n := len([]rune(passage)); n < 2900 {
		t.Fatalf("test setup: passage is %d runes, want about 3000", n)
	}
	typed := "请用这篇：\n" + passage + "\n谢谢"
	got, err := AnchoredSpan(typed, "开头第一句是这篇文章的起点", "最后一句是这篇文章的终点。")
	if err != nil {
		t.Fatalf("AnchoredSpan: %v", err)
	}
	if got != passage {
		t.Fatalf("span has %d runes, want %d", len([]rune(got)), len([]rune(passage)))
	}
}

func TestAnchoredSpanStartNotFound(t *testing.T) {
	_, err := AnchoredSpan(anchorPassage, "这句话她根本没有写过", "还取决于有没有人长期维护。")
	if err == nil || !strings.Contains(err.Error(), "startAnchor 在老师这一轮的消息里找不到") {
		t.Fatalf("err = %v, want the start-anchor-not-found error", err)
	}
}

func TestAnchoredSpanEndNotFound(t *testing.T) {
	_, err := AnchoredSpan(anchorPassage, "一位负责维护的工人说", "这句话她根本没有写过")
	if err == nil || !strings.Contains(err.Error(), "endAnchor 在老师这一轮的消息里找不到") {
		t.Fatalf("err = %v, want the end-anchor-not-found error", err)
	}
}

func TestAnchoredSpanEndBeforeStart(t *testing.T) {
	// The anchors swapped: the end anchor exists, but only before the start.
	_, err := AnchoredSpan(anchorPassage, "还取决于有没有人长期维护。", "一位负责维护的工人说")
	if err == nil || !strings.Contains(err.Error(), "endAnchor 只出现在 startAnchor 之前") {
		t.Fatalf("err = %v, want the end-before-start error", err)
	}
}

func TestAnchoredSpanEndInsideStartIsRejected(t *testing.T) {
	// The end anchor occurs only inside the start anchor, so it cannot end the
	// passage after the start anchor does.
	typed := "甲乙丙丁戊己庚辛壬癸子丑寅卯"
	_, err := AnchoredSpan(typed, "甲乙丙丁戊己庚辛壬癸", "乙丙丁戊己庚辛壬")
	if err == nil || !strings.Contains(err.Error(), "endAnchor 只出现在 startAnchor 之前") {
		t.Fatalf("err = %v, want the end-before-start error", err)
	}
}

func TestAnchoredSpanShortPassageAnchorsOverlap(t *testing.T) {
	// A 12-rune passage named by two 10-rune anchors that share 8 runes.
	typed := "说明：甲乙丙丁戊己庚辛壬癸子丑。"
	got, err := AnchoredSpan(typed, "甲乙丙丁戊己庚辛壬癸", "丙丁戊己庚辛壬癸子丑")
	if err != nil {
		t.Fatalf("AnchoredSpan: %v", err)
	}
	if got != "甲乙丙丁戊己庚辛壬癸子丑" {
		t.Fatalf("span = %q", got)
	}
}

func TestAnchoredSpanFirstOccurrenceWins(t *testing.T) {
	// Both anchors appear twice. The start is the first occurrence, and the
	// end is the first occurrence after it, so the span is the first copy.
	one := "雨水穿过砖缝渗进土里。中间这一句只在第一段里。暴雨时积水的时间变短了。"
	two := "雨水穿过砖缝渗进土里。中间这一句只在第二段里。暴雨时积水的时间变短了。"
	typed := one + "\n" + two
	got, err := AnchoredSpan(typed, "雨水穿过砖缝渗进土里", "暴雨时积水的时间变短了。")
	if err != nil {
		t.Fatalf("AnchoredSpan: %v", err)
	}
	if got != one {
		t.Fatalf("span = %q, want the first copy %q", got, one)
	}
}

func TestAnchoredSpanTooShortAnchor(t *testing.T) {
	_, err := AnchoredSpan(anchorPassage, "一位负责", "还取决于有没有人长期维护。")
	if err == nil || !strings.Contains(err.Error(), "startAnchor 太短") {
		t.Fatalf("err = %v, want the short-anchor error", err)
	}
	_, err = AnchoredSpan(anchorPassage, "一位负责维护的工人说", "长期维护。")
	if err == nil || !strings.Contains(err.Error(), "endAnchor 太短") {
		t.Fatalf("err = %v, want the short-anchor error", err)
	}
}

func TestAnchoredSpanCornerBracketsAreNotFolded(t *testing.T) {
	typed := "工人说：「铺的时候大家都很积极，后来没人管。」然后就没有了。"
	_, err := AnchoredSpan(typed, `工人说："铺的时候大家`, "后来没人管。」然后就没有了。")
	if err == nil || !strings.Contains(err.Error(), "startAnchor") {
		t.Fatalf("err = %v, want a not-found error: 「 is not folded to \"", err)
	}
}

func TestAnchoredSpanRuneIndicesWithMixedWidth(t *testing.T) {
	// ASCII and three-byte runes mixed before the passage: a byte offset
	// would cut in the wrong place.
	typed := "abc 中文 NASA “x” 前言。Data from NASA shows “sea level” rising 3 mm a year，海平面在上升。"
	want := "Data from NASA shows “sea level” rising 3 mm a year，海平面在上升。"
	got, err := AnchoredSpan(typed, `Data from NASA shows "sea`, `mm a year，海平面在上升。`)
	if err != nil {
		t.Fatalf("AnchoredSpan: %v", err)
	}
	if got != want {
		t.Fatalf("span = %q, want %q", got, want)
	}
	if !strings.Contains(typed, got) {
		t.Fatal("span must be a literal substring of what she typed")
	}
}
