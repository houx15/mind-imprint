package api

// Prompt assembly for reading_block.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"

	"fmt"

	"strings"

)

// readingBlockSystemFor 把字数上限填进去。
//
// 🚨 上限**按工具**给，不再是一个写死的 200。2026-09-17 把「把握度」并进
// 「写作解析」之后，那一件要在同一段话里讲两件事（这一段在干什么 + 作者说得
// 有多满），200 字装不下；而别的工具一个字都不该多写 —— 讲解越长她越不读。
func readingBlockSystemFor(t readingBlockTool) string {
	cap := t.MaxRunes
	if cap <= 0 {
		cap = readingBlockDefaultMaxRunes
	}
	return fmt.Sprintf(readingBlockSystem, cap) + t.Instruction + readingShapedSuffix[t.Shape]
}

const readingBlockSystem = prompts.ReadingBlockSystem

// buildReadingBlockPromptFor 按这件工具讲的是哪一级来搭上下文。
//
// 「查词」（Subject == "word"）时，subject 是她点的那一个词：给的是那个词和它
// 所在的整段 —— 一个词在这里是什么意思，只有看着它那一句才说得准。
func buildReadingBlockPromptFor(t readingBlockTool, title string, blocks []Block, idx int, subject string) string {
	if t.Subject != "word" || subject == "" {
		return buildReadingBlockPrompt(title, blocks, idx, subject)
	}
	var b strings.Builder
	if tt := strings.TrimSpace(title); tt != "" {
		b.WriteString("文章标题：" + tt + "\n")
	}
	b.WriteString("\n【要讲解的这一个词】\n" + subject + "\n")
	b.WriteString("\n【它所在的那一段】\n" + strings.TrimSpace(blocks[idx].Text) + "\n")
	return b.String()
}

// sentence 非空时，讲的是这一段里的**那一句**（语法那件工具）。段落仍然给，
// 因为一个代词指的是谁、一个省略省掉了什么，只有把上一句读了才说得清 ——
// 但要讲的是哪一句必须写死，否则模型会顺手把整段都讲一遍。
func buildReadingBlockPrompt(title string, blocks []Block, idx int, sentence string) string {
	var b strings.Builder
	if t := strings.TrimSpace(title); t != "" {
		b.WriteString("文章标题：" + t + "\n")
	}
	if sentence != "" {
		b.WriteString("\n【要讲解的这一句】\n" + sentence + "\n")
		b.WriteString("\n【它所在的那一段（只作参考，不要讲解整段）】\n" +
			strings.TrimSpace(blocks[idx].Text) + "\n")
		return b.String()
	}
	b.WriteString("\n【要讲解的这一段】\n" + strings.TrimSpace(blocks[idx].Text) + "\n")

	// Neighbours, nearest first, until the budget runs out. Position is what
	// 结构解析 / 写作解析 are ABOUT, so a paragraph with no neighbours would
	// make those two tools guess.
	var before, after []string
	budget := readingBlockContextRunes
	for step := 1; budget > 0 && (idx-step >= 0 || idx+step < len(blocks)); step++ {
		if i := idx - step; i >= 0 {
			t := strings.TrimSpace(blocks[i].Text)
			if n := len([]rune(t)); n > 0 && n <= budget {
				before = append([]string{t}, before...)
				budget -= n
			}
		}
		if i := idx + step; i < len(blocks) {
			t := strings.TrimSpace(blocks[i].Text)
			if n := len([]rune(t)); n > 0 && n <= budget {
				after = append(after, t)
				budget -= n
			}
		}
	}
	if len(before) > 0 {
		b.WriteString("\n【它前面的内容（只作参考，不要讲解）】\n" + strings.Join(before, "\n") + "\n")
	} else {
		b.WriteString("\n（这是文章的第一段。）\n")
	}
	if len(after) > 0 {
		b.WriteString("\n【它后面的内容（只作参考，不要讲解）】\n" + strings.Join(after, "\n") + "\n")
	} else {
		b.WriteString("\n（这是文章的最后一段。）\n")
	}
	return b.String()
}
