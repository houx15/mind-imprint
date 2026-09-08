package news

// readable_summary_test.go —— 「每条至少有可读摘要」这条硬规则。
//
// 产品负责人 2026-09-08：只有标题和链接的条目直接排除。
//
// 在这之前**没有任何一道过滤在管这件事**：Dedupe 只挡空标题，FreshWithin 只挡
// 时间，剩下的一路走到 prompt。一条只有标题的候选被选中，模型只能照着标题猜，
// 而她在星球上看到的就是那句猜出来的摘要。

import (
	"strings"
	"testing"
	"time"
)

func item(title, summary, body string) Item {
	return Item{
		Title: title, Link: "https://example.org/" + title,
		Summary: summary, Body: body,
		Published: time.Now(),
	}
}

func TestWithReadableSummaryDropsTitleOnlyItems(t *testing.T) {
	long := "这一条有完整的两句摘要，说清楚了发生了什么，也说清楚了为什么值得知道，够模型判断也够她读。"
	in := []Item{
		item("只有标题", "", ""),
		item("摘要太短", "一句话。", ""),
		item("摘要够长", long, ""),
	}
	out := WithReadableSummary(in)
	if len(out) != 1 {
		t.Fatalf("留下 %d 条，应该只留「摘要够长」那一条：%+v", len(out), titlesOf(out))
	}
	if out[0].Title != "摘要够长" {
		t.Errorf("留错了：%s", out[0].Title)
	}
}

// 🚨 正文在 content:encoded 里的源（Grist description 只有 120 字符），
// 摘要短不等于这条候选空。判据必须看 Body，否则这条规则会把最好的几个源
// 整个砍掉 —— 那正好和它想做的事相反。
func TestWithReadableSummaryKeepsItemsWhoseTextIsInTheBody(t *testing.T) {
	thin := "A short teaser."
	full := strings.Repeat("The grid is the real bottleneck, not the panels. ", 6)
	out := WithReadableSummary([]Item{item("Grist 那种源", thin, full)})
	if len(out) != 1 {
		t.Fatal("description 短、但正文齐全的条目被误杀了 —— 这会砍掉 Grist / JSTOR Daily 这一类源")
	}
}

// 按符文不按字节：一个中文字三个字节，用字节判会让中文轻松过线、英文被砍。
func TestWithReadableSummaryCountsRunesNotBytes(t *testing.T) {
	zhShort := strings.Repeat("短", 20) // 20 个字 = 60 字节：字数不够，字节够
	if got := WithReadableSummary([]Item{item("中文短摘要", zhShort, "")}); len(got) != 0 {
		t.Errorf("20 个中文字被当成够长了（按字节算就会这样）")
	}
	enOK := strings.Repeat("word ", 20) // 100 字符
	if got := WithReadableSummary([]Item{item("英文长摘要", enOK, "")}); len(got) != 1 {
		t.Errorf("一句正常长度的英文摘要被砍掉了")
	}
}

// 🚨 一句真实长度的中文导语必须留得住。中文比英文密得多 —— 这条线要是按英文的
// 手感定（一百来个字符），中文源会被整批误杀，而中文源正是这一轮要补的。
func TestWithReadableSummaryKeepsARealChineseLede(t *testing.T) {
	// 对话地球那种一句话导语，45 个字左右，信息完整。
	zh := "过去十年全球太阳能装机增长约十倍，但真正的瓶颈已经从发电成本转移到储能与电网调度。"
	out := WithReadableSummary([]Item{item("对话地球那种", zh, "")})
	if len(out) != 1 {
		t.Fatalf("一句 %d 个字的中文导语被砍掉了 —— 门槛是按英文手感定的", len([]rune(zh)))
	}
}

/* ── 送进 prompt 的那一段 ───────────────────────────────────────────────── */

// feed 带了正文就把正文给模型看。原来它只看得到 description —— 对 Grist 那种源
// 就是 120 个字符，等于让它看着一句导语决定这条值不值得上星图，还要写摘要和钩子。
func TestPromptShowsTheFullTextWhenTheFeedCarriedIt(t *testing.T) {
	body := "Solar output peaks at noon while demand peaks in the evening, and storage is how that gap gets closed."
	_, user, _ := BuildSelectPrompt([]Item{item("The grid", "A short teaser.", body)})
	if !strings.Contains(user, "demand peaks in the evening") {
		t.Fatalf("正文没有进 prompt，模型还是只看得到那句导语：\n%s", user)
	}
}

// 有些源（IEEE Spectrum 实测 8156 字符）把全文放在 description 里，Body 是空的。
// 取长的那个，不是永远取 Body。
func TestPromptFallsBackToTheSummaryWhenThereIsNoBody(t *testing.T) {
	summary := strings.Repeat("IEEE 把整篇都放在 description 里。", 4)
	_, user, _ := BuildSelectPrompt([]Item{item("Spectrum", summary, "")})
	if !strings.Contains(user, "description 里") {
		t.Fatalf("Body 为空时摘要没进 prompt：\n%s", user)
	}
}

// prompt 里那一段是有上限的：四十条候选，每条无限长会把整个 prompt 撑爆。
func TestPromptExcerptIsCapped(t *testing.T) {
	huge := strings.Repeat("很长的正文。", 2000)
	_, user, _ := BuildSelectPrompt([]Item{item("超长", "teaser", huge)})
	if n := len([]rune(user)); n > 20000 {
		t.Errorf("一条候选就让 prompt 到了 %d 个字，摘录没有被截断", n)
	}
}

func titlesOf(items []Item) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Title)
	}
	return out
}
