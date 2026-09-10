package api

import "testing"

// 导读的三条不变量。它们全是「一句 prompt 里的必须」被翻译成代码的结果 ——
// [[prompt-output-must-be-verifiable-2026-09-03]]：验不了的必须只是期望。

func outlineBlocks(n int) []Block {
	out := make([]Block, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, Block{ID: "b" + itoaSmall(i), Text: "这是第几段的正文，长度足够。"})
	}
	return out
}

func fullLoad(n int, kind string) map[string]string {
	m := map[string]string{}
	for i := 1; i <= n; i++ {
		m["b"+itoaSmall(i)] = kind
	}
	return m
}

func TestOutlineRejectsWhenEverythingIsCore(t *testing.T) {
	// 🚨 这是这份校验里唯一真正重要的一条。模型很容易把每一段都标成核心 ——
	// 那份导读在屏幕上仍然长得像一份导读，但它一个字的信息都没有，而带读的
	// 节奏会退回「每段都停」，也就是没有节奏。
	blocks := outlineBlocks(12)
	got := readingOutline{OneLine: "这篇在问什么", Shape: "问题 → 证据 → 结论", Load: fullLoad(12, loadCore)}
	if _, ok := validateOutline(got, blocks); ok {
		t.Fatal("十二段全标成核心，这份导读应该被整份丢掉")
	}
}

func TestOutlineRejectsWhenNothingIsCore(t *testing.T) {
	blocks := outlineBlocks(9)
	got := readingOutline{OneLine: "这篇在问什么", Load: fullLoad(9, loadSupport)}
	if _, ok := validateOutline(got, blocks); ok {
		t.Fatal("一段核心都没有，等于没有分类")
	}
}

func TestOutlineAcceptsAThirdAsCore(t *testing.T) {
	blocks := outlineBlocks(12)
	load := fullLoad(12, loadSupport)
	for _, id := range []string{"b1", "b5", "b9", "b12"} {
		load[id] = loadCore
	}
	out, ok := validateOutline(readingOutline{OneLine: "屋顶光伏到底划不划算", Shape: "问题 → 数据 → 让步 → 结论", Load: load}, blocks)
	if !ok {
		t.Fatal("十二段里四段核心，正好在上限上，应该收下")
	}
	if got := out.coreBlockIDs(blocks); len(got) != 4 || got[0] != "b1" || got[3] != "b12" {
		t.Fatalf("核心段的顺序应该跟着正文走，拿到 %v", got)
	}
}

func TestOutlineUnknownLoadFallsBackToSupport(t *testing.T) {
	// 模型写错、漏写的那一段退回「支撑」——最不打扰的一档：它不会让带读在一段
	// 普通证据上停下来，也不会让一段真正的核心被跳过去。
	blocks := outlineBlocks(6)
	out, ok := validateOutline(readingOutline{
		OneLine: "这篇在问什么",
		Load:    map[string]string{"b1": loadCore, "b2": "skeleton", "b3": ""},
	}, blocks)
	if !ok {
		t.Fatal("这一份应该收下")
	}
	if len(out.Load) != 6 {
		t.Fatalf("每一段都要有标签，拿到 %d 条", len(out.Load))
	}
	for _, id := range []string{"b2", "b3", "b4", "b5", "b6"} {
		if out.Load[id] != loadSupport {
			t.Fatalf("%s 应该退回 support，拿到 %q", id, out.Load[id])
		}
	}
}

func TestOutlineTrimsLongFields(t *testing.T) {
	blocks := outlineBlocks(6)
	long := ""
	for i := 0; i < 200; i++ {
		long += "字"
	}
	out, _ := validateOutline(readingOutline{
		OneLine: long, Shape: long, Load: map[string]string{"b1": loadCore},
	}, blocks)
	if n := len([]rune(out.OneLine)); n != outlineOneLineMaxRunes {
		t.Fatalf("oneLine 没有被截断：%d", n)
	}
	if n := len([]rune(out.Shape)); n != outlineShapeMaxRunes {
		t.Fatalf("shape 没有被截断：%d", n)
	}
}

func TestDecodeOutlineSurvivesGarbage(t *testing.T) {
	// 一份坏掉的导读不该让整篇文章打不开。
	for _, raw := range []string{"", "{", "null", `{"load":"not a map"}`} {
		o := decodeOutline([]byte(raw))
		if o.Load == nil {
			t.Fatalf("decodeOutline(%q) 的 Load 是 nil —— 调用方会在读它的时候崩", raw)
		}
	}
}
