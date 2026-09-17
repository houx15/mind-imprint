package api

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// reading_block_grammar.go —— 语法那件工具的卡片。
//
// # 产品负责人 2026-09-17 逐字
//
//	grammar, I hope we can be better, like words, become a card. sentence
//	composition split? grammar points? with highlighting, knowledge point,
//	cases, etc. instead of a large paragraph.
//
// 「像词卡一样」这句话里有一个硬要求，而它决定了整个结构：**要能在句子上标出
// 来**。要标出来，就得知道哪几个字是那一块 —— 一段散文没有这个位置。
//
// 所以 readingGrammarPart.Text 逐字来自原句，和词卡的 term 是同一条判据：
// 回原句里核对，核不上的丢掉。那一次核对同时守着两件事：卡片说的是这一句里
// 真有的东西，以及高亮一定落得下去。

// readingGrammarPart 是句子被切开的一块。
type readingGrammarPart struct {
	// Text 是这一块**在原句里的原样**。高亮就是拿它去找的。
	Text string `json:"text"`
	// Role 是中文语法名（主语 / 状语 / 定语从句 …）。
	Role string `json:"role"`
	// Note 说它挂在哪、修饰哪个词。
	Note string `json:"note"`
}

// readingGrammarPoint 是这一句里值得单独学的一个语法点。
type readingGrammarPoint struct {
	Name      string `json:"name"`
	Why       string `json:"why"`
	Example   string `json:"example"`
	ExampleZh string `json:"exampleZh"`
}

// readingGrammar 是整张语法卡。
type readingGrammar struct {
	// Backbone 是主干：谁 + 做了什么。
	Backbone string                `json:"backbone"`
	Parts    []readingGrammarPart  `json:"parts"`
	Points   []readingGrammarPoint `json:"points"`
	// Meaning 是这一句的意思。
	Meaning string `json:"meaning"`
}

const (
	// 一句切成几块：少于两块就不叫「拆开」，多于五块她看见的是一句话被剁碎。
	grammarPartsMin = 2
	grammarPartsMax = 5
	// 语法点：一句话里真正值得单独学的不会有四个，多出来的是在凑数。
	grammarPointsMax = 3
	// 一块最短两个字符 —— 一个逗号、一个 the 标出来没有意义。
	grammarPartMinRunes = 2
)

// parseGrammarCard 读语法那份回话，并丢掉一切核对不上的。
//
// 丢弃规则，和 parseWordCards 同一条纪律：
//
//  1. text 为空、或者短到没有意义 → 丢。
//  2. **text 在这一句里找不到 → 丢。** 高亮无处可落，而她看到的「这一块」
//     在句子里根本指不出来。
//  3. role 为空 → 丢。一块没有名字的切分，她不必看就知道没用。
//  4. 同一块重复 → 只留第一份。
//  5. 活下来的不足两块 → 整张卡失败，退回散文（调用点据此走失败分支）。
//
// 🚨 找的时候大小写敏感：这一块是从**这一句**里抄的，不是一个词的还原形，
// 差一个大小写就说明它没在抄。词卡那边放宽是因为模型爱把句首那个词还原成
// 小写，而那是一个**词**；这里是一整块原文。
func parseGrammarCard(body, sentence string) (readingGrammar, bool) {
	var got readingGrammar
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		return readingGrammar{}, false
	}
	out := readingGrammar{
		Backbone: strings.TrimSpace(got.Backbone),
		Meaning:  strings.TrimSpace(got.Meaning),
	}
	seen := map[string]bool{}
	for _, p := range got.Parts {
		text := strings.TrimSpace(p.Text)
		role := strings.TrimSpace(p.Role)
		if text == "" || role == "" || utf8.RuneCountInString(text) < grammarPartMinRunes {
			continue
		}
		if !strings.Contains(sentence, text) {
			continue
		}
		if seen[text] {
			continue
		}
		seen[text] = true
		out.Parts = append(out.Parts, readingGrammarPart{
			Text: text, Role: role, Note: strings.TrimSpace(p.Note),
		})
		if len(out.Parts) == grammarPartsMax {
			break
		}
	}
	if len(out.Parts) < grammarPartsMin {
		return readingGrammar{}, false
	}
	for _, pt := range got.Points {
		name := strings.TrimSpace(pt.Name)
		if name == "" {
			continue
		}
		out.Points = append(out.Points, readingGrammarPoint{
			Name: name, Why: strings.TrimSpace(pt.Why),
			Example: strings.TrimSpace(pt.Example), ExampleZh: strings.TrimSpace(pt.ExampleZh),
		})
		if len(out.Points) == grammarPointsMax {
			break
		}
	}
	return out, true
}

// grammarCardAsProse 把一张语法卡写成 body 那一列里的纯文字。
//
// 🚨 和 wordCardsAsProse 同一个理由：它**不是拿来渲染的**（界面渲染的是卡片
// 本身）。它存在是为了让这一行在任何一个不带解析器的地方仍然读得懂 ——
// 日后的报告、教师端、一次 psql 查。一行只有 JSON 的记录在那些地方就是一段
// 乱码。
func grammarCardAsProse(g readingGrammar) string {
	var b strings.Builder
	if g.Backbone != "" {
		b.WriteString("**主干**：" + g.Backbone + "\n\n")
	}
	for _, p := range g.Parts {
		b.WriteString("- **" + p.Role + "**：" + p.Text + "\n")
		if p.Note != "" {
			b.WriteString("  " + p.Note + "\n")
		}
	}
	for _, pt := range g.Points {
		b.WriteString("\n**" + pt.Name + "**")
		if pt.Why != "" {
			b.WriteString(" " + pt.Why)
		}
		b.WriteString("\n")
		if pt.Example != "" {
			b.WriteString("  " + pt.Example + "\n")
		}
		if pt.ExampleZh != "" {
			b.WriteString("  " + pt.ExampleZh + "\n")
		}
	}
	if g.Meaning != "" {
		b.WriteString("\n**这一句的意思**：" + g.Meaning + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
