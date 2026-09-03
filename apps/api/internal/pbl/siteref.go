package pbl

// siteref.go — 第二关：她贴一个网址，服务端去读那一页，印记回一张卡。
//
// 产品负责人 2026-09-03：「collect data about the websites they love. let them
// search, and give AI the urls.」
//
// 这一关她**一个字都不用打**。她在自己的浏览器里搜（搜索这件事本来就该在浏览器
// 里做，我们没有比它更好的搜索），把网址粘进来。剩下的是服务端的活。
//
// 卡分三句，不是一段摘要。理由见 migration 0129：这一关之后她要用它们做两件不同
// 的事——照着 structure 搭自己的结构，照着 best 想「哪里可以是我独有的」。混成
// 一段，这两件事都得靠她重读一遍自己拆。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"mindimprint/api/internal/gateway"
)

// SiteRefCard 是读完一页之后印记说的那三句。
type SiteRefCard struct {
	// Title 是那一站的名字。取页面自己的标题，模型只在标题为空时才补。
	Title string `json:"title"`
	// What：这一站在做什么。一句。
	What string `json:"what"`
	// Structure：它由哪几块组成，按顺序。她搭自己结构时照着这个看。
	Structure string `json:"structure"`
	// Best：它最值得学的一处。她想「哪里可以是我独有的」时照着这个想。
	Best string `json:"best"`
}

// NormalizeSiteURL 把她粘进来的东西整理成一个可比较的网址。
//
// 归一化是去重的前提：`example.com`、`https://example.com` 和
// `https://example.com/` 是同一站，而她多半会用不同的形式各贴一次。
//
// 只做安全的整理——去空白、补 scheme、小写 host、去掉末尾单独的斜杠。不动路径
// 大小写（很多站的路径是大小写敏感的），不去 query（有些个人站靠它路由）。
func NormalizeSiteURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errors.New("pbl: empty url")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("pbl: unsupported scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return "", errors.New("pbl: url has no host")
	}
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	if u.Path == "/" {
		u.Path = ""
	}
	return u.String(), nil
}

const siteRefSystem = `你在帮一个中学生看别人的个人网站。他正在做自己的第一个个人
网站，这一步他要**看懂真站的做法**，不是读摘要。

给你一页的正文。回三句话，每一句都短、都具体、都指着这一页上真有的东西：

what      —— 这一站在做什么。谁写的、写什么、给谁看。
structure —— 它由哪几块组成，按页面上的先后顺序说。用真实个人站的说法：
             身份块、文章列表、站点信息、页脚这一类。他等下要照着这个搭自己的。
best      —— 这一站最值得学的一处，以及为什么。挑一处具体的做法，不要夸它好看。

标题为空时才补一个 title，正文里有标题就原样用。

只返回一个 JSON 对象，不要别的字：
{"title": "", "what": "", "structure": "", "best": ""}`

// maxSitePageRunes 是喂给模型的正文上限。
//
// digest 档就是「长输入短输出」，但长也有个头：个人站的首页通常几千字，而一篇
// 长文页可能几万字。截断到前一万字，结构信息（身份块、导航、列表的头几条）全在
// 前面，而尾巴上多半是评论和页脚。
const maxSitePageRunes = 10000

// ReadSiteRef 读一页，回一张卡。
func ReadSiteRef(
	ctx context.Context, prov gateway.Provider, resolved gateway.Resolved,
	pageTitle, body string,
) (SiteRefCard, gateway.ChatUsage, error) {
	if r := []rune(body); len(r) > maxSitePageRunes {
		body = string(r[:maxSitePageRunes])
	}
	user := fmt.Sprintf("页面标题：%s\n\n正文：\n%s", strings.TrimSpace(pageTitle), strings.TrimSpace(body))

	res, err := gateway.Collect(ctx, prov, resolved, gateway.ChatRequest{
		MaxTokens: 1024,
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: siteRefSystem},
			{Role: gateway.RoleUser, Content: user},
		},
	})
	// 用量照回，哪怕这一次失败了：token 已经花掉了，不计就等于账对不上。
	if err != nil {
		return SiteRefCard{}, res.Usage, err
	}
	card, perr := ParseSiteRefCard(res.Text)
	if perr != nil {
		return SiteRefCard{}, res.Usage, perr
	}
	if strings.TrimSpace(card.Title) == "" {
		card.Title = strings.TrimSpace(pageTitle)
	}
	return card, res.Usage, nil
}

// ParseSiteRefCard 读模型返回的 JSON。
//
// 🚨 解析不出来就报错，不给一句像样的兜底文案。一张编出来的卡片会让她照着一个
// 从来没人读过的「结构」去搭自己的页面，而她没有任何办法发现这件事。
// 见 memory · ai-errors-must-surface-never-fake。
func ParseSiteRefCard(raw string) (SiteRefCard, error) {
	s := strings.TrimSpace(raw)
	if i := strings.Index(s, "{"); i > 0 {
		s = s[i:]
	}
	if j := strings.LastIndex(s, "}"); j >= 0 && j < len(s)-1 {
		s = s[:j+1]
	}
	var c SiteRefCard
	if err := json.Unmarshal([]byte(s), &c); err != nil {
		return SiteRefCard{}, fmt.Errorf("pbl: site card is not JSON: %w", err)
	}
	c.Title = strings.TrimSpace(c.Title)
	c.What = strings.TrimSpace(c.What)
	c.Structure = strings.TrimSpace(c.Structure)
	c.Best = strings.TrimSpace(c.Best)
	if c.What == "" || c.Structure == "" || c.Best == "" {
		return SiteRefCard{}, errors.New("pbl: site card is missing one of what/structure/best")
	}
	return c, nil
}

// SiteRefsWanted 是第二关解锁下一步要的站数。
//
// 三个。一个是偶然，两个还看不出共同点，三个才够她自己说出「它们都……」——而
// 那一句正是这一关要她产出的东西。这道闸是真的（服务端拦着），不是置灰按钮。
const SiteRefsWanted = 3
