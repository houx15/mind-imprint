package awakening

import "mindimprint/api/internal/prompts"

// title.go —— 给一条兴趣线索起名。
//
// # 为什么线索要有名字
//
// 线索库里停着好几条，她得认出哪条是哪条。「已答 3 / 8 轮」认不出来，
// 「潮汐发电的视频」认得出来。
//
// # 谁来起
//
// 模型给候选，她挑一个（产品负责人 2026-09-21：「model suggest and user
// selects」）。不是模型直接定：一个概括出来的名字可能根本不是她心里那件事，
// 而这条线索是她的。
//
// # 🚨 候选里永远有一个是她的原话
//
// FallbackTitle 从她第一句回答裁出来，不花调用、不会失败。它有两个用处：
// 模型那一次没回上来时用它（绝不编一个名字塞进去 —— memory:
// ai-errors-must-surface-never-fake），以及始终作为一个选项摆在她面前，
// 因为有时候她自己写的那半句就是最准的名字。

import (
	"encoding/json"
	"fmt"
	"strings"
)

// titleMaxRunes 是名字的长度上限。
//
// 线索库一行要放得下，而一个放不下的名字会被界面截断 —— 那就等于又切了她的
// 字。所以在这里就定死，让模型按这个长度写。
const titleMaxRunes = 14

// TitleChoiceCount 是给她看几个候选。
//
// 三个。两个像是二选一的陷阱，五个要她读一会儿 —— 而这一步只是起个名字，
// 不是又一道题。
const TitleChoiceCount = 3

const titleSystemPrompt = prompts.AwakeningTitleSystemPrompt

// BuildTitlePrompt 拼起名那一次调用。语料只有她自己写下的话。
func BuildTitlePrompt(answers []string) (system, user string) {
	var b strings.Builder
	b.WriteString("学生在这条线索上说过的话：\n\n")
	for i, a := range answers {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		fmt.Fprintf(&b, "%d. %s\n\n", i+1, a)
	}
	b.WriteString("请给这条线索起 3 个名字。")
	return titleSystemPrompt, b.String()
}

// ParseTitles 读模型那一句。读不懂就返回空，调用方退回她的原话。
func ParseTitles(reply string) []string {
	var out struct {
		Titles []string `json:"titles"`
	}
	body, err := sliceJSON(reply)
	if err != nil {
		return nil
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil
	}
	seen := map[string]bool{}
	var titles []string
	for _, t := range out.Titles {
		t = CleanTitle(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		titles = append(titles, t)
		if len(titles) == TitleChoiceCount {
			break
		}
	}
	return titles
}

// CleanTitle 把一个候选收拾成能直接显示的样子：去掉引号、收尾标点和多余空白，
// 超长就按上限裁。
//
// 🚨 裁的是**模型写的名字**，不是她写的字。她自己的话一个都不切
// （memory: observation-tool-is-the-bug-2026-09-12）—— FallbackTitle 那条另有
// 交代。
func CleanTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "「」『』\"'“”《》 　")
	s = strings.TrimRight(s, "。．.!！?？,，、;；:：")
	s = foldSpace(s)
	if runeLen(s) > titleMaxRunes {
		r := []rune(s)
		s = string(r[:titleMaxRunes])
	}
	return strings.TrimSpace(s)
}

// FallbackTitle 从她第一句回答裁出一个名字。
//
// 它不花调用、不会失败，所以它既是模型那次失败的退路，也始终是候选之一。
// 裁到上限时**不加省略号** —— 这是一个标签，不是一段被截断的正文；加了反而
// 像是在说「你的话被切了」。
func FallbackTitle(firstAnswer string) string {
	s := foldSpace(strings.TrimSpace(firstAnswer))
	if s == "" {
		return ""
	}
	// 先在第一个句读处断开：她第一句话本身通常就说清了这是关于什么。
	if i := strings.IndexAny(s, "。！？，、；!?,;"); i > 0 {
		if head := strings.TrimSpace(s[:i]); runeLen(head) >= 4 {
			s = head
		}
	}
	if runeLen(s) > titleMaxRunes {
		s = string([]rune(s)[:titleMaxRunes])
	}
	return strings.TrimSpace(s)
}

// TitleChoices 把模型的候选和她的原话合成一张给她挑的表。
//
// 原话那一个**排在最后**：模型的候选通常更像名字，而她的原话是那条永远在的
// 退路。一个都没有时返回空表，界面这时不摆这一步，直接用空名字（线索库会
// 退回显示她的第一句话）。
func TitleChoices(modelTitles []string, firstAnswer string) []string {
	fallback := FallbackTitle(firstAnswer)
	seen := map[string]bool{}
	var out []string
	for _, t := range modelTitles {
		t = CleanTitle(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	if fallback != "" && !seen[fallback] {
		out = append(out, fallback)
	}
	return out
}
