package api

import (
	"strings"

	"mindimprint/api/internal/quotematch"
)

// reading_pick_match.go —— 她在文章里划的那一句，怎么才算「找到了」。
//
// # 为什么存在（2026-09-23，产品负责人第 1 条）
//
//	「AI提问某些答案的判定有点死板……学生划线句包括答案句，
//	  AI就识别不出来，必须得一个字不差。」
//
// 在这之前，一条 pick 要活下来必须同时满足三件事：
//
//  1. 它自己报的那个段号要对得上（`byID[p.BlockID]`）；
//  2. 这句话是那一段的**字节级**子串（`strings.Contains`）；
//  3. 整句话落在**同一段**里。
//
// 三条里任何一条不成立，这条 pick 就被**静默丢掉** —— 于是 印记 根本看不见
// 【她在文章里点出来的句子】那一栏，它只剩她打的字。从她那一侧看，就是
// 「我明明划了，它说没有」。
//
// 三条各自的翻车方式都是真的：
//
//   - **标点漂移。** 浏览器的选区带上/落下一个句号、一个半角逗号、一个从 PDF
//     粘来的不断行空格 —— 字节比对当场失败。写作面 2026-09-18 已经栽过同一次：
//     「它引的那句和原文只差一个句号」，当时的结论写在 quotematch.Locate 上面：
//     **「逐字」应该由我们从原文里取，不该要求一个标点不差。**
//     阅读面一直没用上那个包。
//   - **段号报错。** 一句逐字正确的原话配了一个错的 blockId，整条丢掉。
//     她选的是对的，错的是客户端算出来的那个号。
//   - **跨段落划。** SplitBlocks 只按空行切，所以一条跨了空行的选区
//     **不是任何一段的子串**，一个字都进不来。
//
// # 修法：先按 quotematch 的规矩找，再把原文那一段取回来
//
// `quotematch.Locate` 正是为这件事写的：按归一化去找，返回**原文里逐字的那一段**。
// 所以下游拿到的仍然是原文的字（高亮、引用、语料都照旧逐字），只是**找**的那一步
// 不再要求她一个标点不差。
//
// 🚨 **不放宽「必须是原文」这条。** 放宽的只有「怎么找」，不是「找到了算不算」：
// 归一化之后仍然要在某一段里连续出现，否则照旧丢掉。一句她自己写的话不会
// 因为这个改动变成原文（memory `detector-must-target-the-real-failure`：
// 绝不放宽比对）。

// locateReadingPick 在整篇里找这一句，返回它落在哪一段、以及原文里逐字的那一段。
//
// 找的顺序是有意的：
//
//  1. 先在她自己报的那一段里找 —— 段号对的时候这是唯一正确的答案，
//     同一句话在两段里都出现时也不会被挪到另一段去。
//  2. 再按顺序找别的段 —— 段号报错了，但她划的确实是原文。
//
// 找不到就返回 ok=false。
func locateReadingPick(blocks []Block, blockID, quote string) (id, span string, ok bool) {
	q := normalizeCardAnswerText(quote)
	if q == "" {
		return "", "", false
	}
	for _, b := range blocks {
		if b.ID != blockID {
			continue
		}
		if span, found := quotematch.Locate(b.Text, q); found {
			return b.ID, span, true
		}
		break
	}
	for _, b := range blocks {
		if b.ID == blockID {
			continue
		}
		if span, found := quotematch.Locate(b.Text, q); found {
			return b.ID, span, true
		}
	}
	return "", "", false
}

// splitReadingPickAcrossBlocks 把一条**跨段落**的选区拆成每段一条。
//
// 🚨 拆而不是丢：她拖过一个空行，意思是「这两段我都要」，不是「我什么都没划」。
// 原来这种选区一个字都进不来（它不是任何一段的子串），于是既不算她点过，
// 也不进 印记 看得见的那一栏。
//
// 只在整条找不到的时候才走这一步，而且每一段仍然各自要过 Locate —— 拆开之后
// 每一条都还是某一段里逐字的一句话，不变量没有松。
//
// 按空行切，和 SplitBlocks 同一条规矩；切完为空的段落丢掉。
func splitReadingPickAcrossBlocks(blocks []Block, quote string) []readingPick {
	parts := strings.Split(normalizeCardAnswerText(quote), "\n\n")
	if len(parts) < 2 {
		return nil
	}
	out := make([]readingPick, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if id, span, ok := locateReadingPick(blocks, "", part); ok {
			out = append(out, readingPick{BlockID: id, Quote: span})
		}
	}
	// 一条都没落地就当没拆过 —— 半条不如没有。
	if len(out) == 0 {
		return nil
	}
	return out
}
