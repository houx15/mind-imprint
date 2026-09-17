package api

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// reading_block_grammar.go —— 语法那件工具的卡片。
//
// # 第二版（产品负责人 2026-09-17 晚些）
//
//	grammar includes 词法，句法，时态.
//	we highlight the key words when we want to illustrate 词法
//	we split the 句子成分, what is 主句，what is从句 during 句法
//	we analyzes the 时态 when we need that.
//	I hope we can use different colors to highlight different parts. and so
//	that the grammar card can be a easy-to-read thing.
//
// 第一版（同一天早些）是「主干 + 切成几块 + 语法点」，块和块之间没有层次：主句、
// 从句、成分混在一张表里。这一版按他说的三件事分层，每一层单独一组高亮：
//
//	clauses   句法 · 这一句里的**从句**（定语从句 / 宾语从句 …）。
//	          🚨 主句**不由模型给**：句子里不属于任何从句的部分就是主句。主句常常
//	          被从句拆成不连续的几截（These feathers, …, might be the key），让模型
//	          逐字抄一段不连续的文字它抄不对；由服务端/界面反推则一定对。
//	parts     句法 · 主句的**句子成分**（主语 / 谓语 / 宾语 / 状语 …）。
//	words     词法 · 值得单独说的**关键词**（词性、词形变化、固定搭配）。
//	tenses    时态 · 需要说的那几个谓语的时态。句子里没有值得说的时态就是空的。
//
// 高亮能落下去，靠的是每一段 text 都逐字来自那一句（parseGrammarCard 回原句核对，
// 核不上的丢掉）——和词卡的 term 同一条判据。
//
// 第一版存下来的卡片（backbone / parts[].role / points）照旧能读：字段都还在，
// 界面看有没有 clauses/words/tenses 决定用哪一种摆法。

// readingGrammarSpan 是句子里被标出来的一段。
type readingGrammarSpan struct {
	// Text 是这一段**在原句里的原样**。高亮就是拿它去找的。
	Text string `json:"text"`
	// Label 是这一段是什么：从句的种类、成分名、词性/词形、时态名。
	Label string `json:"label,omitempty"`
	// Role 是第一版的成分名字段。新卡片不写它；读老卡片时界面用它顶 Label。
	Role string `json:"role,omitempty"`
	// Note 是一句解释。
	Note string `json:"note,omitempty"`
	// Example 只有时态用：一个新造的短例句。
	Example   string `json:"example,omitempty"`
	ExampleZh string `json:"exampleZh,omitempty"`
}

// readingGrammarPart 是第一版的名字，留着给老代码和老测试。
type readingGrammarPart = readingGrammarSpan

// readingGrammarPoint 是第一版的语法点。新卡片不产出它。
type readingGrammarPoint struct {
	Name      string `json:"name"`
	Why       string `json:"why"`
	Example   string `json:"example"`
	ExampleZh string `json:"exampleZh"`
}

// readingGrammar 是整张语法卡。
type readingGrammar struct {
	Clauses []readingGrammarSpan `json:"clauses,omitempty"`
	Parts   []readingGrammarSpan `json:"parts"`
	Words   []readingGrammarSpan `json:"words,omitempty"`
	Tenses  []readingGrammarSpan `json:"tenses,omitempty"`
	// Meaning 是这一句的意思。
	Meaning string `json:"meaning"`

	// 第一版的两项。新卡片不写。
	Backbone string                `json:"backbone,omitempty"`
	Points   []readingGrammarPoint `json:"points,omitempty"`
}

const (
	grammarClausesMax = 4
	grammarPartsMin   = 2
	grammarPartsMax   = 6
	grammarWordsMax   = 5
	grammarTensesMax  = 4
	// 第一版的上限，老测试还在用。
	grammarPointsMax = 3
	// 一段最短两个字符 —— 一个逗号、一个 a 标出来没有意义。
	grammarPartMinRunes = 2
)

// grammarBareConnective 是单独一个连词。实测模型把并列句里的 and 标成「插入语」——
// 连词不是成分，单独标出来就是教错了。
var grammarBareConnective = map[string]bool{
	"and": true, "but": true, "or": true, "so": true, "yet": true, "nor": true, "for": true,
}

// keepVerbatimSpans 留下逐字出现在 sentence 里、带着 label 的那些段，去重、截断。
//
// 🚨 大小写敏感：这一段是从**这一句**里抄的，差一个大小写就说明它没在抄
// （词卡那边放宽，是因为模型爱把句首那个**词**还原成小写；这里的 words 是整句
// 里的原样，同样不放宽 —— 放宽了高亮就可能落在另一个同形词上）。
func keepVerbatimSpans(got []readingGrammarSpan, sentence string, max int) []readingGrammarSpan {
	out := make([]readingGrammarSpan, 0, len(got))
	seen := map[string]bool{}
	for _, s := range got {
		// 两头的逗号去掉（实测模型会交「, which … cold,」）；去掉之后仍是原句的子串。
		text := strings.Trim(s.Text, " \t\n,;:，；：")
		label := strings.TrimSpace(s.Label)
		if label == "" {
			// 第一版的回话把成分名写在 role 里。
			label = strings.TrimSpace(s.Role)
		}
		if text == "" || label == "" || utf8.RuneCountInString(text) < grammarPartMinRunes {
			continue
		}
		if !strings.Contains(sentence, text) || seen[text] || grammarBareConnective[strings.ToLower(text)] {
			continue
		}
		seen[text] = true
		out = append(out, readingGrammarSpan{
			Text: text, Label: label,
			Note:      strings.TrimSpace(s.Note),
			Example:   strings.TrimSpace(s.Example),
			ExampleZh: strings.TrimSpace(s.ExampleZh),
		})
		if len(out) == max {
			break
		}
	}
	return out
}

// dropNestedClauses 去掉被别的从句整段包住的从句，也去掉那个把整句都包进去的
// 「从句」—— 界面一层只画一种底色，套在里面的那一段画不出来；而一个等于整句的
// 从句说明模型把主句也当成了从句。
func dropNestedClauses(cls []readingGrammarSpan, sentence string) []readingGrammarSpan {
	// 先去掉等于整句的那条，再判谁套在谁里面 —— 顺序反过来，整句会把真正的从句
	// 也判成「套在里面」一起丢掉。
	whole := strings.TrimRight(strings.TrimSpace(sentence), ".!?。！？")
	real := make([]readingGrammarSpan, 0, len(cls))
	for _, c := range cls {
		if strings.TrimRight(c.Text, ".!?。！？") != whole {
			real = append(real, c)
		}
	}
	out := make([]readingGrammarSpan, 0, len(real))
	for i, c := range real {
		nested := false
		for j, o := range real {
			if i != j && len(o.Text) > len(c.Text) && strings.Contains(o.Text, c.Text) {
				nested = true
				break
			}
		}
		if !nested {
			out = append(out, c)
		}
	}
	return out
}

// parseGrammarCard 读语法那份回话，并丢掉一切核对不上的。
//
// 丢弃规则，和 parseWordCards 同一条纪律：每一段都要逐字来自那一句、带着名字；
// 核不上的丢掉。
//
// 四层都可以是空的 —— owner 2026-09-17：「not every card need all these. only need
// to highlight those key points.」模型只交这一句值得讲的那一两层。**四层全空才整张
// 作废**。句子成分只剩一段时这一层丢掉（一个孤零零的「主语」讲不出结构）。
func parseGrammarCard(body, sentence string) (readingGrammar, bool) {
	var got readingGrammar
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		return readingGrammar{}, false
	}
	out := readingGrammar{
		Clauses: dropNestedClauses(keepVerbatimSpans(got.Clauses, sentence, grammarClausesMax), sentence),
		Parts:   keepVerbatimSpans(got.Parts, sentence, grammarPartsMax),
		Words:   keepVerbatimSpans(got.Words, sentence, grammarWordsMax),
		Tenses:  keepVerbatimSpans(got.Tenses, sentence, grammarTensesMax),
		Meaning: strings.TrimSpace(got.Meaning),
		// 第一版的回话（老测试、老桩）里有这两项；新的 prompt 不要它们。
		Backbone: strings.TrimSpace(got.Backbone),
	}
	for _, pt := range got.Points {
		if name := strings.TrimSpace(pt.Name); name != "" && len(out.Points) < grammarPointsMax {
			out.Points = append(out.Points, readingGrammarPoint{
				Name: name, Why: strings.TrimSpace(pt.Why),
				Example: strings.TrimSpace(pt.Example), ExampleZh: strings.TrimSpace(pt.ExampleZh),
			})
		}
	}
	if len(out.Parts) < grammarPartsMin {
		out.Parts = []readingGrammarSpan{}
	}
	if len(out.Clauses)+len(out.Parts)+len(out.Words)+len(out.Tenses) == 0 {
		return readingGrammar{}, false
	}
	return out, true
}

// grammarCardAsProse 把一张语法卡写成 body 那一列里的纯文字。
//
// 🚨 和 wordCardsAsProse 同一个理由：它**不是拿来渲染的**（界面渲染的是卡片
// 本身）。它存在是为了让这一行在任何一个不带解析器的地方仍然读得懂 ——
// 日后的报告、教师端、一次 psql 查。
func grammarCardAsProse(g readingGrammar) string {
	var b strings.Builder
	if g.Backbone != "" {
		b.WriteString("**主干**：" + g.Backbone + "\n\n")
	}
	section := func(title string, spans []readingGrammarSpan) {
		if len(spans) == 0 {
			return
		}
		b.WriteString("**" + title + "**\n")
		for _, s := range spans {
			b.WriteString("- **" + s.Label + "**：" + s.Text + "\n")
			if s.Note != "" {
				b.WriteString("  " + s.Note + "\n")
			}
			if s.Example != "" {
				b.WriteString("  " + s.Example + "\n")
			}
			if s.ExampleZh != "" {
				b.WriteString("  " + s.ExampleZh + "\n")
			}
		}
		b.WriteString("\n")
	}
	section("句法 · 从句", g.Clauses)
	section("句法 · 句子成分", g.Parts)
	section("词法", g.Words)
	section("时态", g.Tenses)
	for _, pt := range g.Points {
		b.WriteString("**" + pt.Name + "**")
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
