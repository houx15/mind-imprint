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
		t.Fatal("十二段里四段核心，应该收下")
	}
	if got := out.coreBlockIDs(blocks); len(got) != 4 || got[0] != "b1" || got[3] != "b12" {
		t.Fatalf("核心段的顺序应该跟着正文走，拿到 %v", got)
	}
}

// 核心段那条线在**一半**上，而不是三分之一。
//
// 🚨 这条测试记的是一次实测。`LIVE_LLM=1 ... TestLiveEnglishReadingPlanParses`
// 在一篇 12 段的英文社论上跑，模型反复给出 5/12 段核心 —— 而旧的线是 12/3=4，
// 于是整份导读被丢掉（第一轮 6 次里丢了 3 次）。那几份**别的什么毛病都没有**。
// 而 2026-09-17 起切法和这份导读同生共死，切法又是通读那一步的台阶，所以那一段
// 的代价是她的通读退回「读完告诉我一声」。见 coreShareCap。
func TestOutlineCoreShareLineIsHalf(t *testing.T) {
	blocks := outlineBlocks(12)
	core := func(n int) map[string]string {
		load := fullLoad(12, loadSupport)
		for i := 1; i <= n; i++ {
			load[blocks[i-1].ID] = loadCore
		}
		return load
	}
	for _, n := range []int{1, 4, 5, 6} {
		out, why := validateOutlineWhy(
			readingOutline{OneLine: "这篇在问什么", Shape: "问题 → 证据 → 结论", Load: core(n)}, blocks)
		if why != outlineOK {
			t.Errorf("%d/12 段核心被丢掉了（%s）—— 连切法一起没了", n, why)
		}
		if len(out.coreBlockIDs(blocks)) != n {
			t.Errorf("%d/12: 核心段数对不上", n)
		}
	}
	// 七段就是一半以上了：那份分类确实没在分类。
	if _, why := validateOutlineWhy(
		readingOutline{OneLine: "这篇在问什么", Load: core(7)}, blocks); why != outlineRejectTooMuchCore {
		t.Errorf("7/12 段核心应该被丢掉，拿到 %q", why)
	}
}

// 四种作废理由各自说得出自己是谁 —— 线上只有一行日志，分不出是哪一条的时候
// 只能猜，而四条的修法完全不同。
func TestOutlineRejectReasonsAreDistinct(t *testing.T) {
	blocks := outlineBlocks(6)
	cases := []struct {
		name string
		got  readingOutline
		want outlineReject
	}{
		{"英文", readingOutline{OneLine: "Is breakfast a moral duty?", Load: fullLoad(6, loadCore)}, outlineRejectNotCJK},
		{"全是核心", readingOutline{OneLine: "这篇在问什么", Load: fullLoad(6, loadCore)}, outlineRejectTooMuchCore},
		{"一段核心都没有", readingOutline{OneLine: "这篇在问什么", Load: fullLoad(6, loadSupport)}, outlineRejectNoCore},
	}
	for _, c := range cases {
		if _, why := validateOutlineWhy(c.got, blocks); why != c.want {
			t.Errorf("%s: why = %q, want %q", c.name, why, c.want)
		}
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

func TestOutlineRejectsAnEnglishOneLine(t *testing.T) {
	// 🚨 线上第一次跑就撞上的：文章是英文的，模型顺着文章的语言把导读也写成了
	// 英文。那份导读摆在中文界面上，对她等于不存在。
	blocks := outlineBlocks(9)
	load := fullLoad(9, loadSupport)
	load["b2"] = loadCore
	got := readingOutline{
		OneLine: "War is escalating — can aid groups still reach the people trapped inside?",
		Shape:   "outbreak → blockade → aid scramble → war",
		Load:    load,
	}
	if _, ok := validateOutline(got, blocks); ok {
		t.Fatal("一个汉字都没有的导读应该被整份丢掉")
	}
}

func TestOutlineKeepsEnglishProperNouns(t *testing.T) {
	// 判据取最宽的那一个：**一个汉字都没有**才算没写中文。人名、地名、机构名
	// 照抄原文是对的做法，不该被误伤。
	blocks := outlineBlocks(9)
	load := fullLoad(9, loadSupport)
	load["b2"] = loadCore
	got := readingOutline{
		OneLine: "在 Gaza，援助进得去吗",
		Shape:   "冲突 → 封锁 → 救援 → 未解决",
		Load:    load,
	}
	if _, ok := validateOutline(got, blocks); !ok {
		t.Fatal("带英文专有名词的中文导读应该收下")
	}
}

/* ── 分几部分（2026-09-16） ────────────────────────────────────────────────
 *
 * 「通读全文」对一个没读过这篇的学生来说不是一个动作，是一整件事。切成几个
 * 部分之后它才有台阶 —— 而带读会**照着这份切法一部分一部分地走**，所以一份
 * 坏掉的切法不是难看，是把带读断在一个不存在的段落上。
 *
 * 判据全在 validateParts 里，每一条都对应一种真的会发生的坏切法。整份丢掉
 * 而不是修补：一份被修补过的切法在屏幕上和一份真的切法长得一模一样。
 * ------------------------------------------------------------------------ */

func goodParts() []readingPart {
	return []readingPart{
		{Title: "提出争议", From: "b1", To: "b3", Does: "摆出两方的说法"},
		{Title: "实测数据", From: "b4", To: "b8", Does: "用一组数据支持前面那个判断"},
		{Title: "还没定的部分", From: "b9", To: "b12", Does: "说清楚哪些还没有答案"},
	}
}

func TestValidatePartsKeepsAGoodSplit(t *testing.T) {
	got := validateParts(goodParts(), outlineBlocks(12))
	if len(got) != 3 {
		t.Fatalf("好好的三部分被丢了：%+v", got)
	}
	if got[0].Title != "提出争议" || got[2].To != "b12" {
		t.Errorf("切法被改坏了：%+v", got)
	}
}

func TestValidatePartsDropsASplitThatPointsAtNothing(t *testing.T) {
	// 模型自己数段号，数错过不止一次（b13 写成 b31）。指着不存在的段落的台阶
	// 比没有台阶更糟 —— 她点下去，屏幕上什么都不会发生。
	bad := goodParts()
	bad[1].To = "b31"
	if got := validateParts(bad, outlineBlocks(12)); got != nil {
		t.Errorf("段号不存在的切法被留下了：%+v", got)
	}
}

func TestValidatePartsDropsOverlappingAndReversedRanges(t *testing.T) {
	cases := map[string]func([]readingPart){
		"两部分重叠": func(p []readingPart) { p[1].From = "b2" },
		"头尾写反":  func(p []readingPart) { p[1].From, p[1].To = "b8", "b4" },
		"顺序乱了":  func(p []readingPart) { p[0], p[2] = p[2], p[0] },
	}
	for name, breakIt := range cases {
		bad := goodParts()
		breakIt(bad)
		if got := validateParts(bad, outlineBlocks(12)); got != nil {
			t.Errorf("%s：这份切法该被整个丢掉，却留下了 %+v", name, got)
		}
	}
}

// 🚨 不从第一段开始，意味着前面那几段她读到的时候屏幕上什么提示都没有 ——
// 而那几段往往正是导语。
func TestValidatePartsRequiresStartingAtTheFirstParagraph(t *testing.T) {
	bad := goodParts()
	bad[0].From = "b2"
	if got := validateParts(bad, outlineBlocks(12)); got != nil {
		t.Errorf("从第二段开始的切法被留下了：%+v", got)
	}
}

// 一整篇算一部分，等于没切 —— 那一步又回到「通读全文，读完告诉我」。
func TestValidatePartsRejectsASingleChunk(t *testing.T) {
	one := []readingPart{{Title: "全文", From: "b1", To: "b12"}}
	if got := validateParts(one, outlineBlocks(12)); got != nil {
		t.Errorf("一整篇算一部分该被丢掉：%+v", got)
	}
	if got := validateParts(nil, outlineBlocks(12)); got != nil {
		t.Errorf("没给切法时该返回 nil：%+v", got)
	}
}

// 中间**允许有缝**：模型漏了一段，代价远小于整份丢掉。缝里的段落照常显示，
// 只是不属于任何一部分。
func TestValidatePartsToleratesAGapBetweenParts(t *testing.T) {
	gappy := []readingPart{
		{Title: "提出争议", From: "b1", To: "b3"},
		{Title: "实测数据", From: "b5", To: "b8"},
	}
	if got := validateParts(gappy, outlineBlocks(12)); len(got) != 2 {
		t.Errorf("中间少一段不该让整份切法作废：%+v", got)
	}
}

// 没有名字的部分在界面上是一行空白 —— 比没有这一行更糟。
func TestValidatePartsDropsASplitWithAnUnnamedPart(t *testing.T) {
	bad := goodParts()
	bad[1].Title = "   "
	if got := validateParts(bad, outlineBlocks(12)); got != nil {
		t.Errorf("有一部分没名字，该整份丢掉：%+v", got)
	}
}

// 切法坏掉不该让整份导读作废 —— 导读的其余几样（在问什么 / 中心思想 / 承重）
// 仍然有用，而通读那一步本来就有「没有分部分的时候」那条后路。
func TestOutlineSurvivesABadSplit(t *testing.T) {
	blocks := outlineBlocks(12)
	load := fullLoad(12, loadSupport)
	load["b4"] = loadCore
	got := readingOutline{
		OneLine: "这篇在问什么", Gist: "作者认为这件事划算", Shape: "问题 → 证据 → 结论",
		Load: load, Parts: []readingPart{{Title: "全文", From: "b1", To: "b99"}},
	}
	out, ok := validateOutline(got, blocks)
	if !ok {
		t.Fatal("切法坏了，整份导读也被丢掉了")
	}
	if len(out.Parts) != 0 {
		t.Errorf("坏掉的切法被留下了：%+v", out.Parts)
	}
	if out.Gist != "作者认为这件事划算" {
		t.Errorf("中心思想没留住：%q", out.Gist)
	}
}

// 中心思想超长就截断，不作废 —— 它是加分项，不是导读成立的条件。
func TestOutlineTrimsAnOverlongGist(t *testing.T) {
	blocks := outlineBlocks(12)
	load := fullLoad(12, loadSupport)
	load["b4"] = loadCore
	long := ""
	for i := 0; i < 200; i++ {
		long += "长"
	}
	out, ok := validateOutline(readingOutline{
		OneLine: "这篇在问什么", Gist: long, Shape: "问题 → 结论", Load: load,
	}, blocks)
	if !ok {
		t.Fatal("中心思想太长不该让整份导读作废")
	}
	if n := len([]rune(out.Gist)); n != outlineGistMaxRunes {
		t.Errorf("中心思想 %d 字，应该截到 %d", n, outlineGistMaxRunes)
	}
}
