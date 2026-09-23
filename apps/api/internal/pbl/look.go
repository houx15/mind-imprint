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

const paletteSystem = `你在帮助中学生为个人网站选择配色。输入的关键词和感受由学生提供，请据此提出三组有明显区别的配色，供学生比较。差异可以来自色相、明暗或整体对比关系，不只改变同一颜色的深浅。

每组包含：
paper：页面底色。
ink：正文颜色，与 paper 保持足够对比，保证阅读清晰。
accent：链接、日期或标记等重点元素的颜色。
label：配色的简短名称。
why：用一句话解释这组配色与哪些输入关键词相符。

颜色统一使用 #RRGGBB 六位十六进制。根据学生给出的风格选择配色，不默认采用暖米色。label 和 why 会直接显示给学生，请写清具体选择及其理由。
只返回 JSON：
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
	// 同 GeneratePersonas：重试一次。见那边的注释。
	var usage gateway.ChatUsage
	var perr error
	var out []Palette
	for attempt := 0; attempt < 2; attempt++ {
		res, err := gateway.Collect(ctx, prov, resolved, gateway.ChatRequest{
			MaxTokens: 1024,
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: paletteSystem},
				{Role: gateway.RoleUser, Content: user},
			},
		})
		usage.InputTokens += res.Usage.InputTokens
		usage.OutputTokens += res.Usage.OutputTokens
		if err != nil {
			return nil, usage, err
		}
		out, perr = ParsePalettes(res.Text)
		if perr == nil {
			return out, usage, nil
		}
	}
	return nil, usage, perr
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// ParsePalettes 读模型返回的 JSON。
//
// 🚨 颜色格式在这里**验**，不只在 prompt 里要求。一个 "warm beige" 或者
// "rgb(240,235,220)" 会原样流进 CSS 变量，浏览器把整条声明丢掉，她看到的是一个
// 没有变化的预览——而没有任何一层报过错。规矩要能在代码里验（memory ·
// prompt-output-must-be-verifiable-2026-09-03）。
func ParsePalettes(raw string) ([]Palette, error) {
	// 数括号取出第一个配平的对象。见 jsonwire.go —— 「第一个 { 到最后一个 }」
	// 在模型前后还写了话的时候会切出一段坏的。
	s := firstJSONObject(strings.TrimSpace(raw))
	var out struct {
		Palettes []Palette `json:"palettes"`
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		// 🚨 把模型原样回的东西带进错误里（截断）。不带的话，线上只剩一句
		// 「不是 JSON」，而到底是它写了别的、还是我们切错了范围，无从判断。
		return nil, fmt.Errorf("pbl: palettes are not JSON: %w；模型回的是：%s",
			err, clip(raw, 400))
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
