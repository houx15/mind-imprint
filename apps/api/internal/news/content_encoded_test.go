package news

// content_encoded_test.go —— feed 自己带的正文，别再丢掉了。
//
// 2026-09-08 从生产 ECS 实测（每源取中位数，字符）：
//
//	源                     description   content:encoded
//	JSTOR Daily （现有源）        192            9694     ← 我们只留了 2%
//	MIT Tech Review（现有源）     355            5692
//	Colossal      （现有源）      381            3000
//	Grist                       120            6900
//	MIT News · AI               153            6906
//	对话地球 中文                 104            3569
//
// `rssItem` 里没有这个字段，于是三个**已经在跑的源**每天把整篇文章送到我们面前，
// 我们留下一两句就扔掉，然后在她按「现在读」时再去抓一次原页面 —— 被出版方挡掉
// （走查里就是 400 fetch_failed），最后让她自己粘。

import (
	"strings"
	"testing"
)

// WordPress 系的形状：description 一句导语，content:encoded 整篇正文。
const wpFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">
  <channel>
    <item>
      <title>The grid is the bottleneck</title>
      <link>https://example.org/grid</link>
      <description>A short teaser sentence.</description>
      <content:encoded><![CDATA[<p>Solar output peaks at noon while demand peaks in the evening.</p>
      <p>Storage, demand response and transmission are the three ways to close that gap.</p>]]></content:encoded>
      <pubDate>Mon, 08 Sep 2026 09:00:00 +0000</pubDate>
    </item>
  </channel>
</rss>`

func TestParseKeepsTheFullTextTheFeedAlreadySent(t *testing.T) {
	items, err := Parse([]byte(wpFeed))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items", len(items))
	}
	it := items[0]
	// 摘要还是那一句短的 —— 星球卡片上显示的是它，不该突然变成一整篇。
	if it.Summary != "A short teaser sentence." {
		t.Errorf("Summary = %q，摘要不该被正文顶掉", it.Summary)
	}
	if !strings.Contains(it.Body, "demand peaks in the evening") {
		t.Fatalf("content:encoded 丢了，Body = %q", it.Body)
	}
	if !strings.Contains(it.Body, "three ways to close that gap") {
		t.Errorf("正文只留了一半：%q", it.Body)
	}
	// HTML 标签要清掉，不然喂给模型的是一堆 <p>。
	if strings.Contains(it.Body, "<p>") {
		t.Errorf("Body 里还留着标签：%q", it.Body)
	}
}

// 没有 content:encoded 的源（Aeon / Psyche / ScienceDaily / Phys.org 实测都没有）
// 照旧只有摘要 —— Body 是空的，不是把摘要复制一份。两个字段说的是两件事。
func TestParseLeavesBodyEmptyWhenTheFeedHasNoFullText(t *testing.T) {
	plain := `<?xml version="1.0"?><rss version="2.0"><channel><item>
	  <title>Only a summary here</title><link>https://example.org/a</link>
	  <description>Two sentences, and that is all the feed carries.</description>
	  <pubDate>Mon, 08 Sep 2026 09:00:00 +0000</pubDate>
	</item></channel></rss>`
	items, err := Parse([]byte(plain))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if items[0].Body != "" {
		t.Errorf("Body 该是空的，得到 %q", items[0].Body)
	}
	if items[0].Summary == "" {
		t.Error("Summary 不该是空的")
	}
}

// 🚨 命名空间必须按 URI 匹配。写成 `content:encoded`（前缀）在 encoding/xml 里
// **匹配不上，而且不报错** —— 只是每一条的 Body 都是空串，和「这个源没有正文」
// 长得一模一样。这条测试用一个不同的前缀来钉住这件事：前缀叫什么无所谓，URI
// 对上就该读到。
func TestParseMatchesContentByNamespaceNotPrefix(t *testing.T) {
	odd := `<?xml version="1.0"?>
	<rss version="2.0" xmlns:c="http://purl.org/rss/1.0/modules/content/">
	  <channel><item>
	    <title>Prefix is not the point</title><link>https://example.org/b</link>
	    <description>teaser</description>
	    <c:encoded>The body arrived under a different prefix.</c:encoded>
	    <pubDate>Mon, 08 Sep 2026 09:00:00 +0000</pubDate>
	  </item></channel></rss>`
	items, err := Parse([]byte(odd))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !strings.Contains(items[0].Body, "different prefix") {
		t.Errorf("按前缀而不是按命名空间匹配了：Body = %q", items[0].Body)
	}
}

// 正文在解析处就截断：选星只用得到几百字，而我们没有转载许可，不该把整篇
// 文章拿在手里。
func TestParseCapsTheBody(t *testing.T) {
	long := strings.Repeat("这是一段很长的正文。", 900) // 远超 bodyRuneCap
	feed := `<?xml version="1.0" encoding="UTF-8"?>
	<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">
	  <channel><item>
	    <title>A very long piece</title><link>https://example.org/c</link>
	    <description>teaser</description>
	    <content:encoded><![CDATA[` + long + `]]></content:encoded>
	    <pubDate>Mon, 08 Sep 2026 09:00:00 +0000</pubDate>
	  </item></channel></rss>`
	items, err := Parse([]byte(feed))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n := len([]rune(items[0].Body)); n > bodyRuneCap+1 {
		t.Errorf("Body 有 %d 个字，上限是 %d", n, bodyRuneCap)
	}
}

// 🚨 XML 声明前面的空白 / BOM。Dezeen 就是这样（实测 942 KB，Python 的解析器
// 直接报 "XML or text declaration not at start of entity"）。Go 认不认，靠这条
// 测试说了算，而不是靠猜别的语言的行为。
func TestParseSurvivesLeadingWhitespaceAndBOM(t *testing.T) {
	// Go 源码里写不了 BOM 字面量（编译器报 illegal byte order mark），用转义。
	const bom = "\uFEFF"
	for name, prefix := range map[string]string{
		"前导空白": "\n  ",
		"BOM":  bom,
		"两者都有": bom + "\n ",
	} {
		t.Run(name, func(t *testing.T) {
			items, err := Parse([]byte(prefix + wpFeed))
			if err != nil {
				t.Fatalf("%s 让整个 feed 读不出来：%v", name, err)
			}
			if len(items) != 1 {
				t.Fatalf("got %d items", len(items))
			}
		})
	}
}
