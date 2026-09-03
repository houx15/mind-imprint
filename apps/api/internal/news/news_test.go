package news

import (
	"strings"
	"testing"
	"time"
)

// news_test —— 新闻管线里「读代码看不出对错」的那几处。
//
// 这一层几乎全是静默失败的地方：解析器少认一种格式会得到零条（不报错）、
// 政治过滤误伤会砍掉气候整根枝（不报错）、日期解析失败会让旧闻冒充今日
// （不报错）。所以测试压在这三件事上。

/* ── 解析 ───────────────────────────────────────────────────────────────── */

const rss20 = `<?xml version="1.0"?>
<rss version="2.0"><channel>
  <title>Test</title>
  <item>
    <title>Deep-sea corals survived 30°C water</title>
    <link>https://example.org/a</link>
    <description>&lt;p&gt;A reef in the Red Sea &lt;b&gt;did not&lt;/b&gt; bleach.&lt;/p&gt;</description>
    <pubDate>Wed, 02 Sep 2026 11:04:05 -0400</pubDate>
  </item>
  <item>
    <title>Second story</title>
    <link>https://example.org/b</link>
    <description>Plain text summary.</description>
    <pubDate>Tue, 01 Sep 2026 09:00:00 GMT</pubDate>
  </item>
</channel></rss>`

// arXiv 用 RSS 1.0（RDF）：item 是根的直接子元素，不在 channel 里。按 RSS 2.0
// 的路径去取会得到**零条而且不报错** —— 这正是这个测试存在的理由。
const rdf10 = `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns="http://purl.org/rss/1.0/"
         xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel><title>arXiv</title></channel>
  <item>
    <title>A new estimate of the Hubble constant</title>
    <link>https://arxiv.org/abs/1</link>
    <description>We report a measurement.</description>
    <dc:date>2026-09-02</dc:date>
  </item>
</rdf:RDF>`

const atom = `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <title>An atom entry</title>
    <link rel="self" href="https://example.org/feed"/>
    <link rel="alternate" href="https://example.org/real"/>
    <summary>Summary here.</summary>
    <updated>2026-09-02T10:00:00Z</updated>
  </entry>
</feed>`

func TestParseRSS20(t *testing.T) {
	got, err := Parse([]byte(rss20))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("解析出 %d 条，want 2", len(got))
	}
	if got[0].Title != "Deep-sea corals survived 30°C water" {
		t.Errorf("标题不对：%q", got[0].Title)
	}
	// 摘要里的 HTML 必须被压平；`&lt;p&gt;` 这种双重转义也要处理掉。
	if strings.ContainsAny(got[0].Summary, "<>") {
		t.Errorf("摘要里还留着标签：%q", got[0].Summary)
	}
	if !strings.Contains(got[0].Summary, "did not bleach") {
		t.Errorf("摘要正文丢了：%q", got[0].Summary)
	}
	if got[0].Published.IsZero() {
		t.Error("pubDate 没解析出来")
	}
}

func TestParseRDFBecauseArxivUsesIt(t *testing.T) {
	got, err := Parse([]byte(rdf10))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("RSS 1.0 解析出 %d 条，want 1 —— item 在根下不在 channel 里", len(got))
	}
	if got[0].Published.IsZero() {
		t.Error("dc:date 没解析出来")
	}
}

func TestParseAtomTakesTheAlternateLink(t *testing.T) {
	got, err := Parse([]byte(atom))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// rel="self" 指向 feed 自己。取错的话每颗星球都链回 feed 而不是文章。
	if got[0].Link != "https://example.org/real" {
		t.Errorf("取到的链接是 %q，应该是 alternate 那条", got[0].Link)
	}
}

func TestParseRejectsGarbageInsteadOfReturningNothing(t *testing.T) {
	// 「解析成功但零条」和「这个源今天没更新」长得一样，处理方式却完全不同。
	if _, err := Parse([]byte("<html><body>not a feed</body></html>")); err == nil {
		t.Error("一份 HTML 应该报错，而不是安静地返回零条")
	}
}

func TestParseTimeReturnsZeroNotNowOnFailure(t *testing.T) {
	// 🚨 解析不出的时间当成「现在」，会让一条日期坏掉的旧闻永远是今日新闻。
	if got := parseTime("not a date", ""); !got.IsZero() {
		t.Errorf("解析失败应该给零值，得到 %v", got)
	}
}

/* ── 政治过滤 ───────────────────────────────────────────────────────────── */

func TestIsPoliticalCatchesTheRealThing(t *testing.T) {
	for _, s := range []string{
		"Election results reshape the senate",
		"Ceasefire holds after missile strike",
		"New sanctions target the chip industry",
		"总统宣布新一轮制裁",
		"议会通过弹劾动议",
	} {
		if !IsPolitical(s, "") {
			t.Errorf("没挡住：%q", s)
		}
	}
}

// 🚨 这是这个文件里最重要的一组。一个宁枉勿纵的政治过滤器会把气候科学整根枝
// 砍掉 —— 而气候恰恰是这个产品最想给学生的东西之一。"war" 不加词边界就会命中
// "warming"、"warn"、"forward"。
func TestIsPoliticalDoesNotEatScience(t *testing.T) {
	for _, s := range []string{
		"Global warming pushed the reef past its threshold",
		"Early warning system detects the quake 12 seconds sooner",
		"A step forward for fusion confinement",
		"Warm water corals adapt faster than expected",
		"Congressional Budget Office data used in the model", // congress 有词边界，但 congressional 没有
		"The award-winning telescope opens its eye",
		"全球变暖让珊瑚越过了临界点",
		"预警系统提前 12 秒发出警报",
	} {
		if IsPolitical(s, "") {
			t.Errorf("误伤了一条科学新闻：%q", s)
		}
	}
}

func TestIsPoliticalReadsTheSummaryToo(t *testing.T) {
	if !IsPolitical("A neutral title", "The election was contested in three states.") {
		t.Error("摘要里的政治词没有被看到")
	}
}

/* ── 去重 ───────────────────────────────────────────────────────────────── */

func TestDedupeCollapsesTheSameStudyFromThreeAggregators(t *testing.T) {
	// 同一项研究会被 Phys.org / ScienceDaily 同一天各发一遍，标题只差标点。
	got := Dedupe([]Item{
		{Title: "Corals survive 30°C water", Link: "https://a.org/1", Source: "Nature"},
		{Title: "Corals survive 30 °C water!", Link: "https://b.org/2", Source: "Phys.org"},
		{Title: "A different finding", Link: "https://c.org/3", Source: "ScienceDaily"},
	})
	if len(got) != 2 {
		t.Fatalf("去重后 %d 条，want 2：%+v", len(got), got)
	}
	// 先出现的那条留下 —— 调用方按源的可信度排序，Nature 该赢过转载它的聚合站。
	if got[0].Source != "Nature" {
		t.Errorf("留下的是 %q，应该留先出现的那条", got[0].Source)
	}
}

func TestDedupeCatchesTheSameLinkUnderDifferentTitles(t *testing.T) {
	got := Dedupe([]Item{
		{Title: "One title", Link: "https://a.org/1"},
		{Title: "Another title entirely", Link: "https://a.org/1"},
	})
	if len(got) != 1 {
		t.Fatalf("同一个链接没有被去掉：%d 条", len(got))
	}
}

func TestDedupeDropsEmptyTitles(t *testing.T) {
	if got := Dedupe([]Item{{Title: "   ", Link: "https://a.org/1"}}); len(got) != 0 {
		t.Errorf("空标题应该被丢掉，得到 %d 条", len(got))
	}
}

/* ── 新鲜度 ─────────────────────────────────────────────────────────────── */

func TestFreshWithinDropsZeroTimes(t *testing.T) {
	// 🚨 日期解析不出来的条目没法判断新鲜度。当成今天的，就是让一条来路不明的
	// 旧闻混进「今日新闻」。
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	got := FreshWithin([]Item{
		{Title: "no date"},
		{Title: "today", Published: now.Add(-2 * time.Hour)},
		{Title: "last week", Published: now.Add(-8 * 24 * time.Hour)},
	}, now, 48*time.Hour)

	if len(got) != 1 || got[0].Title != "today" {
		t.Fatalf("留下的是 %+v", got)
	}
}

func TestFreshWithinKeepsFutureStamps(t *testing.T) {
	// 时区写错、或标着明天日期的文章，不该被丢。
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	got := FreshWithin([]Item{{Title: "tomorrow", Published: now.Add(20 * time.Hour)}}, now, 48*time.Hour)
	if len(got) != 1 {
		t.Error("未来时间戳被丢掉了")
	}
}

// 🚨 WHO 的 feed 返回 200、140 KB，最新一条是六个月前。只看状态码的健康检查
// 会一直报绿，而星图上会出现半年前的「今日新闻」。
func TestSourceIsAliveIgnoresHTTPStatusAndLooksAtDates(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)

	dead := []Item{{Title: "old", Published: now.Add(-180 * 24 * time.Hour)}}
	if SourceIsAlive(dead, now) {
		t.Error("半年没更新的源被判成活着")
	}
	alive := []Item{{Title: "old", Published: now.Add(-180 * 24 * time.Hour)},
		{Title: "recent", Published: now.Add(-3 * 24 * time.Hour)}}
	if !SourceIsAlive(alive, now) {
		t.Error("三天前更新过的源被判成死的")
	}
	if SourceIsAlive(nil, now) {
		t.Error("空源不该被判成活着")
	}
}

func TestSortByPublishedPutsNewestFirstAndZeroLast(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	items := []Item{
		{Title: "zero"},
		{Title: "old", Published: now.Add(-48 * time.Hour)},
		{Title: "new", Published: now.Add(-1 * time.Hour)},
	}
	SortByPublished(items)
	if items[0].Title != "new" || items[1].Title != "old" || items[2].Title != "zero" {
		t.Errorf("排序错了：%v", []string{items[0].Title, items[1].Title, items[2].Title})
	}
}

/* ── 选星 ───────────────────────────────────────────────────────────────── */

func candidates(n int) []Item {
	out := make([]Item, n)
	for i := range out {
		out[i] = Item{Title: "story", Link: "https://x/", Source: "Nature"}
	}
	return out
}

func TestBuildSelectPromptCarriesTheWholeDisciplineTable(t *testing.T) {
	_, user := BuildSelectPrompt(candidates(3))
	// 主枝本身是待判定的，所以候选学科给全表 —— 限定候选等于替模型先做了那个
	// 判断。抽查两条分属不同主枝的。
	for _, id := range []string{"statistical-inference", "climate-ocean", "media-literacy"} {
		if !strings.Contains(user, id) {
			t.Errorf("学科 %s 没进 prompt", id)
		}
	}
}

func TestBuildSelectPromptCapsTheCandidates(t *testing.T) {
	_, user := BuildSelectPrompt(candidates(200))
	if strings.Contains(user, "[40]") {
		t.Error("候选没有被截到 40 条 —— 模型在第四十条之后就开始敷衍，而每条都在花钱")
	}
}

func TestParseSelectReplyHappyPath(t *testing.T) {
	raw := "```json\n" + `{"planets":[
		{"index":1,"titleZh":"深海珊瑚在 30 度水里活下来了","titleEn":"Corals survive",
		 "summary":"红海北端一片珊瑚没有白化。","hook":"四平方公里，能代表一整片海吗？",
		 "field":"science","disciplineId":"climate-ocean","keyword":"样本代表性"}]}` + "\n```"
	got, err := ParseSelectReply(raw, candidates(3))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 || got[0].Index != 1 || got[0].Keyword != "样本代表性" {
		t.Fatalf("解析结果不对：%+v", got)
	}
}

func TestParseSelectReplyDropsOutOfRangeIndex(t *testing.T) {
	raw := `{"planets":[{"index":99,"titleZh":"x","hook":"y？","field":"science"}]}`
	if _, err := ParseSelectReply(raw, candidates(3)); err == nil {
		t.Error("指向不存在候选的星球被留下了")
	}
}

func TestParseSelectReplyNeedsAHook(t *testing.T) {
	// 一颗没有钩子的星球是一条只能被记住、不能被追问的新闻 —— 这一屏的全部
	// 意义就是那个问题。
	raw := `{"planets":[{"index":0,"titleZh":"x","hook":"","field":"science"}]}`
	if _, err := ParseSelectReply(raw, candidates(3)); err == nil {
		t.Error("没有钩子的星球被留下了")
	}
}

func TestParseSelectReplyKeepsThePlanetButDropsABadDiscipline(t *testing.T) {
	// 学科连错比没连上糟；但为了一条连错的边扔掉一条好新闻更糟。
	raw := `{"planets":[{"index":0,"titleZh":"x","hook":"y？","field":"science","disciplineId":"astrology"}]}`
	got, err := ParseSelectReply(raw, candidates(3))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("星球被整颗丢掉了")
	}
	if got[0].DisciplineID != "" {
		t.Errorf("不存在的学科 id 被留下了：%q", got[0].DisciplineID)
	}
}

func TestParseSelectReplyDropsUnknownField(t *testing.T) {
	raw := `{"planets":[{"index":0,"titleZh":"x","hook":"y？","field":"magic"}]}`
	if _, err := ParseSelectReply(raw, candidates(3)); err == nil {
		t.Error("野主枝被留下了")
	}
}

func TestParseSelectReplyDedupesIndexAndCapsAtFive(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"planets":[`)
	for i := 0; i < 12; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		// 前两条故意用同一个 index。
		idx := i
		if i == 1 {
			idx = 0
		}
		fmtPlanet(&b, idx)
	}
	b.WriteString("]}")

	got, err := ParseSelectReply(b.String(), candidates(20))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != PlanetCount {
		t.Fatalf("回了 %d 颗星，want %d", len(got), PlanetCount)
	}
	seen := map[int]bool{}
	for _, p := range got {
		if seen[p.Index] {
			t.Errorf("同一个 index 出现了两次：%d", p.Index)
		}
		seen[p.Index] = true
	}
}

func fmtPlanet(b *strings.Builder, idx int) {
	b.WriteString(`{"index":`)
	b.WriteString(itoa(idx))
	b.WriteString(`,"titleZh":"标题","titleEn":"t","summary":"s","hook":"为什么？",`)
	b.WriteString(`"field":"science","disciplineId":"climate-ocean","keyword":"关键词"}`)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

// 解析不出来时**报错**，让今天没有星图 —— 绝不编五条新闻，也绝不用昨天的
// 冒充今天的。见 memory: ai-errors-must-surface-never-fake。
func TestParseSelectReplyErrorsOnGarbage(t *testing.T) {
	for _, raw := range []string{"", "抱歉，我无法完成这个请求。", "{not json"} {
		if _, err := ParseSelectReply(raw, candidates(3)); err == nil {
			t.Errorf("垃圾输入 %q 没有报错", raw)
		}
	}
}
