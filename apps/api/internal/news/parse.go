package news

import (
	"encoding/xml"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
)

// parse.go —— 把 RSS 2.0 与 Atom 读成同一种 Item。
//
// 自己解析而不是引一个 feed 库：这两种格式我们只用四个字段（标题、链接、摘要、
// 时间），而一个第三方 feed 库会把「日期解析失败」这类事默默吞掉 —— 而日期正是
// 这里唯一真正重要的判据（见 sources.go 的 StaleAfter：**200 不等于还活着**）。

// Item 是一条候选新闻。
type Item struct {
	Title string
	Link  string
	// Summary 是 feed 里那段短的（RSS 的 description / Atom 的 summary）。
	// 它是**给人看的一两句**，长度按源从一百到八千字符不等。
	Summary string
	// Body 是 feed 自己带的正文（RSS 的 content:encoded / Atom 的 content）。
	//
	// 🚨 2026-09-08 实测：我们一直只读 description，而**很多源的正文就在
	// content:encoded 里，被我们整段丢掉**。同一条 JSTOR Daily，description
	// 192 字符，content:encoded 9694 字符 —— 我们留下 2%。MIT Tech Review
	// 355 / 5692，Colossal 381 / 3000，都是现有源。
	//
	// 它有两种用法，规矩完全不同，别混：
	//   - **选星时喂给模型**：让它有真材料可判，写得出像样的摘要和钩子。这是
	//     内部处理。
	//   - **填进阅读室当正文**：那是转载，要看源的许可。**今天不做** —— 等
	//     产品负责人对 NC 类许可给出结论之前，这个字段不进任何一张表。
	//
	// 因此在解析处就截断（bodyRuneCap）：我们只留用得上的那一段，不把一整篇
	// 没有许可的文章拿在手里。
	Body      string
	Published time.Time
	// Source / Field 由抓取方按源填上，解析器不管。
	Source string
	Field  string
}

// bodyRuneCap 是 Body 保留多长。
//
// 选星的 prompt 每条只用得到几百字（见 BuildSelectPrompt），留 4000 已经绰绰
// 有余。上限本身也是一条纪律：我们没有转载许可，就不该把整篇文章拿在手里。
const bodyRuneCap = 4000

/* ── XML 形状 ───────────────────────────────────────────────────────────── */

type rssFeed struct {
	XMLName xml.Name  `xml:"rss"`
	Items   []rssItem `xml:"channel>item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	// content:encoded —— WordPress 系的源（Grist / JSTOR Daily / Colossal /
	// MIT Tech Review / 对话地球 / designboom …）把**整篇正文**放在这里，
	// description 里只留一句导语。少了这一行，那些源我们每天都在丢掉九成内容。
	//
	// 命名空间要写全 URI，不能写 `content:encoded` —— encoding/xml 认的是
	// 「空间 URI + 本地名」，写前缀会匹配不上，而且**不报错**，只是永远是空串。
	Encoded string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	PubDate string `xml:"pubDate"`
	// RSS 1.0 / RDF 用 dc:date；Nature 的几个源就是这样。
	DCDate string `xml:"http://purl.org/dc/elements/1.1/ date"`
}

// rdfFeed —— RSS 1.0（RDF）。`item` 是**根的直接子元素**，不在 `channel` 里面，
// 按 RSS 2.0 的路径去取会得到零条，而且不报错。
//
// Nature 那几个源走的就是这条路（它们的时间也在 dc:date 上）。arXiv 原来也是
// RDF，2026-09-08 因为常年零条被拿掉了（见 sources.go），但这条分支和它无关，
// 不能跟着删。
type rdfFeed struct {
	XMLName xml.Name  `xml:"RDF"`
	Items   []rssItem `xml:"item"`
}

type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title   string     `xml:"title"`
	Links   []atomLink `xml:"link"`
	Summary string     `xml:"summary"`
	Content string     `xml:"content"`
	Updated string     `xml:"updated"`
	Published string   `xml:"published"`
}

// 🚨 `,attr`。少了它，encoding/xml 会去找**子元素** `<href>` 而不是属性，
// 于是每条 Atom 条目的链接都是空字符串 —— 不报错，只是每颗星球都点不开。
type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

/* ── 解析 ───────────────────────────────────────────────────────────────── */

// Parse 读一份 feed。三种格式都试，全都读不出条目才报错。
//
// 报错而不是返回空切片：一个「解析成功但零条」的结果，和「这个源今天没更新」
// 长得一模一样，而这两件事需要的处理完全不同。
func Parse(body []byte) ([]Item, error) {
	if out, err := parseRSS(body); err == nil && len(out) > 0 {
		return out, nil
	}
	if out, err := parseRDF(body); err == nil && len(out) > 0 {
		return out, nil
	}
	if out, err := parseAtom(body); err == nil && len(out) > 0 {
		return out, nil
	}
	return nil, fmt.Errorf("feed 里没有解析出任何条目（RSS / RDF / Atom 都试过）")
}

func parseRSS(body []byte) ([]Item, error) {
	var f rssFeed
	if err := xml.Unmarshal(body, &f); err != nil {
		return nil, err
	}
	return fromRSSItems(f.Items), nil
}

func parseRDF(body []byte) ([]Item, error) {
	var f rdfFeed
	if err := xml.Unmarshal(body, &f); err != nil {
		return nil, err
	}
	return fromRSSItems(f.Items), nil
}

func fromRSSItems(items []rssItem) []Item {
	out := make([]Item, 0, len(items))
	for _, it := range items {
		title := clean(it.Title)
		if title == "" {
			continue
		}
		out = append(out, Item{
			Title:     title,
			Link:      strings.TrimSpace(it.Link),
			Summary:   clean(it.Description),
			Body:      truncRunes(clean(it.Encoded), bodyRuneCap),
			Published: parseTime(it.PubDate, it.DCDate),
		})
	}
	return out
}

func parseAtom(body []byte) ([]Item, error) {
	var f atomFeed
	if err := xml.Unmarshal(body, &f); err != nil {
		return nil, err
	}
	out := make([]Item, 0, len(f.Entries))
	for _, e := range f.Entries {
		title := clean(e.Title)
		if title == "" {
			continue
		}
		// 取 rel="alternate"（或没有 rel 的那条）—— rel="self" 指向 feed 自己，
		// 用它会让每颗星球都链回 feed 而不是文章。
		link := ""
		for _, l := range e.Links {
			if l.Rel == "" || l.Rel == "alternate" {
				link = l.Href
				break
			}
		}
		summary := e.Summary
		if strings.TrimSpace(summary) == "" {
			summary = e.Content
		}
		out = append(out, Item{
			Title:   title,
			Link:    strings.TrimSpace(link),
			Summary: clean(summary),
			// Atom 这一侧对应 content:encoded 的是 <content>。summary 为空时
			// 它已经被当成摘要用了，那时两个字段一样长 —— 不必再分。
			Body:      truncRunes(clean(e.Content), bodyRuneCap),
			Published: parseTime(e.Published, e.Updated),
		})
	}
	return out, nil
}

// timeLayouts 覆盖实测里见过的几种。RFC1123Z 是 RSS 的常规写法，
// `-0400` 与 `EDT` 两种时区写法都出现过（ScienceDaily 用 EDT）。
var timeLayouts = []string{
	time.RFC1123Z,
	time.RFC1123,
	time.RFC3339,
	"2006-01-02T15:04:05Z0700",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 MST",
	"Mon, 02 Jan 2006 15:04:05 Z",
}

// parseTime 依次试每个候选字符串、每种格式。全都失败返回零值。
//
// 🚨 零值**不是**「现在」。把解析不出的时间当成现在，等于让一条日期坏掉的旧新闻
// 永远显示为今天的新闻 —— 而 StaleAfter 那道判据就是靠这个时间生效的。
func parseTime(candidates ...string) time.Time {
	for _, s := range candidates {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		for _, layout := range timeLayouts {
			if t, err := time.Parse(layout, s); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

var tagRE = regexp.MustCompile(`<[^>]*>`)
var spaceRE = regexp.MustCompile(`\s+`)

// clean 把一段 feed 里的 HTML 摘要压成一行纯文本。
//
// 顺序有讲究：先反转义**再**去标签。反过来的话，一段被转义成 `&lt;p&gt;` 的
// HTML 会先躲过去标签、再被反转义成真的标签留在文本里。WHO 的 feed 就是这样
// 双重转义的。
func clean(s string) string {
	s = html.UnescapeString(s)
	s = tagRE.ReplaceAllString(s, " ")
	s = html.UnescapeString(s) // 双重转义的源（WHO）需要第二遍
	s = spaceRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
