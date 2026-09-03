package pbl

// look.go — 第三关：给网站定调子。
//
// 产品负责人 2026-09-03：「what is color palette and your choice? style? hero
// image, do you need? can generate it here - your website is becoming real.」
//
// ## 配色是从她的关键词派生的，不是一排色卡
//
// 「挑一个你喜欢的颜色」是一道和这个项目无关的题——她凭直觉点一个，页面就多了
// 一个她说不出理由的决定。这一关问的是另一件事：**哪一组颜色配得上你说的那个
// 读者**。所以生成的输入是第一关留下的关键词，每一组配色都要说清楚它为什么配
// 那几个词。她挑的时候，理由已经写在那儿了。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"mindimprint/api/internal/gateway"
)

// Palette 是一组配色。
//
// 三个颜色，不是五个：SiteTheme 只认 paper / ink / accent 三格（字体跟着版式走，
// 生成的字体栈只会挑出一个中文缺字的字体）。多给的颜色没有地方放，只会让她在
// 一个看不见效果的东西上做选择。
type Palette struct {
	// Label 是这组配色的名字，一个短词。
	Label string `json:"label"`
	// Why 是它为什么配她那几个关键词。一句。
	Why string `json:"why"`
	// Paper 是底色，Ink 是正文色，Accent 是那一点强调色。
	Paper  string `json:"paper"`
	Ink    string `json:"ink"`
	Accent string `json:"accent"`
}

// PalettesWanted 是一次给几组。
const PalettesWanted = 3

const paletteSystem = `你在帮一个中学生给他自己的个人网站定配色。

给你的是他自己定下的关键词——那是他希望这一页给读者的感觉。你要给三组配色，
每一组都**配得上其中某几个词**，而且三组之间要真的不一样（不是同一个色相的三
个深浅）。

每一组三个颜色：

paper   底色。整页最大的一块，所以它决定第一眼的感觉。
ink     正文颜色。必须和 paper 有足够的对比度，正文要能读。
accent  一点强调色。链接、日期、小标记用它。

label   这组配色的名字，一个短词。
why     它为什么配那几个关键词。一句话，指名道姓地说是哪几个词。

三条硬规矩：

🚨 颜色一律写成 #RRGGBB 六位十六进制，不要写颜色名、不要写 rgb()。
🚨 ink 和 paper 的对比度要够读正文。浅底配浅字是这一关最容易犯、而且她当场
   看不出来（预览很小）的错。
🚨 不要三组都是暖米色。她给的词如果冷、如果硬，就给冷的、硬的。

只返回一个 JSON 对象，不要别的字：
{"palettes":[{"label":"","why":"","paper":"#","ink":"#","accent":"#"}]}`

// GeneratePalettes 从她留下的关键词派生几组配色。
func GeneratePalettes(
	ctx context.Context, prov gateway.Provider, resolved gateway.Resolved,
	keywords []string, feeling string,
) ([]Palette, gateway.ChatUsage, error) {
	kw := trimAll(keywords)
	if len(kw) == 0 {
		return nil, gateway.ChatUsage{}, errors.New("pbl: no keywords to derive a palette from")
	}
	user := "他的关键词：" + strings.Join(kw, "、")
	if t := strings.TrimSpace(feeling); t != "" {
		user += "\n他希望这一页给读者的感觉：" + t
	}
	res, err := gateway.Collect(ctx, prov, resolved, gateway.ChatRequest{
		MaxTokens: 1024,
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: paletteSystem},
			{Role: gateway.RoleUser, Content: user},
		},
	})
	if err != nil {
		return nil, res.Usage, err
	}
	out, perr := ParsePalettes(res.Text)
	return out, res.Usage, perr
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// ParsePalettes 读模型返回的 JSON。
//
// 🚨 颜色格式在这里**验**，不只在 prompt 里要求。一个 "warm beige" 或者
// "rgb(240,235,220)" 会原样流进 CSS 变量，浏览器把整条声明丢掉，她看到的是一个
// 没有变化的预览——而没有任何一层报过错。规矩要能在代码里验（memory ·
// prompt-output-must-be-verifiable-2026-09-03）。
func ParsePalettes(raw string) ([]Palette, error) {
	s := strings.TrimSpace(raw)
	if i := strings.Index(s, "{"); i > 0 {
		s = s[i:]
	}
	if j := strings.LastIndex(s, "}"); j >= 0 && j < len(s)-1 {
		s = s[:j+1]
	}
	var out struct {
		Palettes []Palette `json:"palettes"`
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("pbl: palettes are not JSON: %w", err)
	}

	kept := make([]Palette, 0, len(out.Palettes))
	for _, p := range out.Palettes {
		p.Label = strings.TrimSpace(p.Label)
		p.Why = strings.TrimSpace(p.Why)
		p.Paper = strings.TrimSpace(p.Paper)
		p.Ink = strings.TrimSpace(p.Ink)
		p.Accent = strings.TrimSpace(p.Accent)
		if p.Label == "" || !ValidPalette(p) {
			continue
		}
		kept = append(kept, p)
		if len(kept) == PalettesWanted {
			break
		}
	}
	if len(kept) == 0 {
		return nil, errors.New("pbl: no usable palette came back")
	}
	return kept, nil
}

// ValidPalette 报告三个颜色都是 #RRGGBB。
//
// 服务端保存她那一次选择时也走这一条：她提交的东西同样不可信，而一个坏颜色存进
// 去之后，坏的是她已经发布出去的那一页。
func ValidPalette(p Palette) bool {
	return hexColor.MatchString(p.Paper) &&
		hexColor.MatchString(p.Ink) &&
		hexColor.MatchString(p.Accent)
}

// HeroPrompt 把她的关键词变成画头图的那句话。
//
// 🚨 三条是我们加的，不由模型决定：
//
//  1. **不要文字。** 生成模型往图里写字几乎必然出错（尤其中文），而头图上一行
//     乱码是整页最显眼的一处坏。
//  2. **不要人脸。** 头图是她这一页的封面，不是一张肖像；一张陌生人的脸放在
//     一个未成年人的主页顶上，会被读成「这是她」。
//  3. **抽象、留白。** 头图底下压着她的名字和那一句话，图太满就读不清了。
func HeroPrompt(keywords []string, feeling string) string {
	kw := trimAll(keywords)
	base := strings.Join(kw, "、")
	if t := strings.TrimSpace(feeling); t != "" {
		base += "；整体感觉：" + t
	}
	if strings.TrimSpace(base) == "" {
		base = "安静、留白"
	}
	return "一张个人网站的横幅头图，抽象几何构图，表达这些词：" + base +
		"。大量留白，柔和配色，不要任何文字，不要人脸，不要具体的可辨认物体。"
}
