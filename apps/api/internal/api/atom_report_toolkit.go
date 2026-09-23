package api

import (
	"encoding/json"
	"strings"

	"mindimprint/api/internal/store/sqlc"
)

// atom_report_toolkit.go —— 阅读报告上「段落工具」那一节。
//
// 产品负责人 2026-09-17：「and also, these things, students' actions, their
// learns, the shadow writing, the words. can be revealed in their reading
// report.」
//
// 全部是**确定性**的：数的是已经存下来的行（reading_block_note、atom_message），
// 不花一次模型调用，也就不会编。四样东西：
//
//	tools     她在几段上用过哪几件工具
//	words     她学过的词（关键单词 + 她自己点的查词），词卡原样
//	grammar   她拆过的句子和那几个语法点的名字
//	writings  她在想一想 / 仿写 底下**自己写的**那几段
//
// 🚨 writings 里只放她写的那一段（answer.Choice）。工具那一行（「仿写 · 第3段：
// 先给场景再讲原理」）是 印记 的话，单独放在 Prompt 里，界面据此标清谁说的
// （R4：报告上每一段引文都要说清是谁的话）。

type reportToolUse struct {
	Label string `json:"label"`
	// Blocks 是她在几段上用过这件工具（同一段用两次算一段）。
	Blocks int `json:"blocks"`
}

type reportGrammar struct {
	// Sentence 是她拆过的那一句，原文。
	Sentence string `json:"sentence"`
	// Points 是那一句的语法点名字。
	Points []string `json:"points"`
}

type reportToolWriting struct {
	// Tool 是「想一想」或「仿写」。
	Tool string `json:"tool"`
	// Prompt 是 印记 那一行（工具名 + 段号 + 题目/写法）。不是她的话。
	Prompt string `json:"prompt"`
	// Text 是她写的那一段，原样。
	Text string `json:"text"`
}

type reportToolkit struct {
	Tools    []reportToolUse     `json:"tools,omitempty"`
	Words    []readingWord       `json:"words,omitempty"`
	Grammar  []reportGrammar     `json:"grammar,omitempty"`
	Writings []reportToolWriting `json:"writings,omitempty"`
}

const (
	// 一份报告上的生词最多摆这么多张。多了是一本词典，不是她这一篇的收获。
	reportToolkitWordsMax    = 12
	reportToolkitGrammarMax  = 6
	reportToolkitWritingsMax = 6
)

// buildReportToolkit 把她在段落工具上做过的事收成一节。什么都没做就是 nil ——
// 这一节整个不显示，而不是一个空框。
func buildReportToolkit(lang string, notes []sqlc.ReadingBlockNote, msgs []sqlc.AtomMessage) *reportToolkit {
	labels := map[string]string{}
	for _, t := range readingBlockToolsAll() {
		labels[t.ID] = t.Label
	}

	out := &reportToolkit{}

	// tools：按工具目录的顺序摆，数的是段数。目录里没有的工具（比如并掉的
	// 「把握度」）不摆 —— 没有名字的一行比没有这一行更糟。
	blocksByTool := map[string]map[string]bool{}
	for _, n := range notes {
		if _, ok := labels[n.Tool]; !ok {
			continue
		}
		if blocksByTool[n.Tool] == nil {
			blocksByTool[n.Tool] = map[string]bool{}
		}
		blocksByTool[n.Tool][n.BlockID] = true
	}
	for _, t := range readingBlockToolsAll() {
		if bs := blocksByTool[t.ID]; len(bs) > 0 {
			out.Tools = append(out.Tools, reportToolUse{Label: t.Label, Blocks: len(bs)})
		}
	}

	// words / grammar：从笔记的 data 那一列读回来，和阅读室读的是同一份。
	seenWord := map[string]bool{}
	seenSentence := map[string]bool{}
	for _, n := range notes {
		if len(n.Data) == 0 {
			continue
		}
		var payload struct {
			Words   []readingWord   `json:"words"`
			Grammar *readingGrammar `json:"grammar"`
		}
		if err := json.Unmarshal(n.Data, &payload); err != nil {
			continue
		}
		for _, w := range payload.Words {
			key := strings.ToLower(strings.TrimSpace(w.Term))
			if key == "" || seenWord[key] || len(out.Words) >= reportToolkitWordsMax {
				continue
			}
			seenWord[key] = true
			out.Words = append(out.Words, w)
		}
		if g := payload.Grammar; g != nil && n.Subject != "" && !seenSentence[n.Subject] &&
			len(out.Grammar) < reportToolkitGrammarMax {
			seenSentence[n.Subject] = true
			rg := reportGrammar{Sentence: n.Subject}
			// 报告上这一句挂的名字：从句的种类、时态，老卡片的语法点。去重。
			seenName := map[string]bool{}
			addName := func(name string) {
				if name = strings.TrimSpace(name); name != "" && !seenName[name] {
					seenName[name] = true
					rg.Points = append(rg.Points, name)
				}
			}
			for _, c := range g.Clauses {
				addName(c.Label)
			}
			for _, tn := range g.Tenses {
				addName(tn.Label)
			}
			for _, pt := range g.Points {
				addName(pt.Name)
			}
			out.Grammar = append(out.Grammar, rg)
		}
	}

	// writings：她在想一想 / 仿写 底下交给 印记 的那几段。
	for _, m := range msgs {
		if m.Role != "student" || len(out.Writings) >= reportToolkitWritingsMax {
			continue
		}
		ans := coachAnswerFromPayload(m.Payload)
		if ans == nil || ans.Type != blockToolAnswerType {
			continue
		}
		text := strings.TrimSpace(ans.Choice)
		if text == "" {
			continue
		}
		tool := ""
		for _, name := range []string{"想一想", "仿写"} {
			if strings.HasPrefix(ans.Prompt, name) {
				tool = name
			}
		}
		out.Writings = append(out.Writings, reportToolWriting{Tool: tool, Prompt: ans.Prompt, Text: text})
	}

	if len(out.Tools) == 0 && len(out.Words) == 0 && len(out.Grammar) == 0 && len(out.Writings) == 0 {
		return nil
	}
	return out
}
