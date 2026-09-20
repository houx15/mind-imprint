package api

import (
	"strings"
	"testing"
)

// 🚨 这一条守的是走查本身，不是产品：如果这两个 scenario 的 prompt 一样大，
// 那条 A/B 就什么也没量到，而它看起来照样是绿的。
// [[fixture-told-coach-session-over-2026-09-14]] 的教训 —— 用例的形状不对，
// 量出来的东西就不是产品的。
func TestLongReadWalkActuallyNarrows(t *testing.T) {
	enableDisclosure(t)
	full := newLongReadWalkDriver(false)
	scoped := newLongReadWalkDriver(true)

	if n := len(full.blocks); n < 20 {
		t.Fatalf("长文走查的文章只有 %d 段 —— 换一篇更长的，否则测不出收窄", n)
	}
	articleRunes := 0
	for _, b := range full.blocks {
		articleRunes += len([]rune(b.Text))
	}
	if articleRunes < readingDisclosureMinRunes {
		t.Fatalf("文章 %d 字，低于收窄门槛 %d —— 这条走查触发不了渐进披露，"+
			"正是它要测的那件事", articleRunes, readingDisclosureMinRunes)
	}

	f := len([]rune(full.Request().Messages[1].Content))
	s := len([]rune(scoped.Request().Messages[1].Content))
	if s >= f {
		t.Fatalf("收窄那一支没有更小：full=%d scoped=%d", f, s)
	}
	t.Logf("article %d runes over %d blocks; user block %d -> %d (-%.0f%%)",
		articleRunes, len(full.blocks), f, s, 100*float64(f-s)/float64(f))

	// 承重段必须还在：它们是这篇文章的骨架，收窄不该把它们收走。
	body := scoped.Request().Messages[1].Content
	for id, load := range scoped.outline.Load {
		if load != loadCore {
			continue
		}
		var text string
		for _, b := range scoped.blocks {
			if b.ID == id {
				text = b.Text
			}
		}
		head := string([]rune(strings.TrimSpace(text))[:20])
		if !strings.Contains(body, head) {
			t.Errorf("承重段 %s 被收起来了，它应该永远展开", id)
		}
	}
}
