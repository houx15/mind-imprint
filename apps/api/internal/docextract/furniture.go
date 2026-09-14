package docextract

import (
	"regexp"
	"strings"
)

// 页眉、页脚、页码 —— 一份 PDF 里那些「不是文章」的字。
//
// 🚨 这件事只有在这里做得成：**页的边界只在这一层存在**。文本一旦拼成一整篇，
// 「这一行是第 3 页的页眉」和「这一句她写了两遍」就再也分不开了。
//
// 两处都吃这个亏：
//   - 阅读室按空行切块，一行「第 3 页」会变成**一个独立的段落块**，而 印记 会
//     把它当一句话来引、来摆板、来让她标角色。
//   - 写作那边的通篇审阅有一个「重复说法」的计数（另一条会话 2026-09-12 加的）：
//     它只看得到最终文本，看不到页边界，于是每页都出现的页眉会被如实数成
//     「她重复了 12 次」，然后 印记 会为此说她。
//
// 判据是**位置 + 重复**，不是长相：一行文字出现在很多页的同一个位置（第一行或
// 最后一行），它就是页眉页脚。靠长相猜（「短的、带数字的」）会误伤正文里真正的
// 短句和数据行。

// pageNumberOnly 认**整行只有一个页码**：`12`、`- 12 -`、`第 12 页`、`Page 12`、
// `12 / 30`。
//
// 🚨 判的是「整行」。正文里当然到处是数字，但一行只剩一个数字的时候，它不是
// 句子 —— 这正是页码在 PDF 里的样子。
var pageNumberOnly = regexp.MustCompile(
	`^(?:[-—–]\s*)?(?:[Pp]age\s+)?(?:第\s*)?[0-9]{1,4}(?:\s*[页頁])?(?:\s*[-—–])?(?:\s*[/／]\s*[0-9]{1,4})?$`)

// furnitureMinPages 是「重复到什么程度才算页眉」的门槛。
//
// 两页的文档里一行出现两次，很可能只是巧合（或者真的是她写了两遍）；三页起，
// 同一行出现在多数页的同一个位置，就不是巧合了。
const furnitureMinPages = 3

// stripFurniture 去掉每页重复的页眉页脚和纯页码行，返回干净的每页文本。
//
// 页数少于 furnitureMinPages 的时候只去纯页码行 —— 「重复」这个判据在两页上
// 立不住，而页码行本身靠长相就认得出来，不需要重复来佐证。
func stripFurniture(pages []string) []string {
	// 每页的第一行和最后一行各数一次。
	firstCount := map[string]int{}
	lastCount := map[string]int{}
	for _, p := range pages {
		lines := nonEmptyLines(p)
		if len(lines) == 0 {
			continue
		}
		firstCount[lines[0]]++
		if len(lines) > 1 {
			lastCount[lines[len(lines)-1]]++
		}
	}

	// 过半的页都以同一行起手（或收尾）—— 那是页眉（页脚）。
	need := furnitureMinPages
	if half := len(pages)/2 + 1; half > need {
		need = half
	}
	isHeader := func(counts map[string]int, line string) bool {
		return len(pages) >= furnitureMinPages && counts[line] >= need
	}

	out := make([]string, 0, len(pages))
	for _, p := range pages {
		lines := nonEmptyLines(p)
		kept := make([]string, 0, len(lines))
		for i, ln := range lines {
			// 纯页码行：任何位置都去掉，页数多少都算。
			if pageNumberOnly.MatchString(ln) {
				continue
			}
			if i == 0 && isHeader(firstCount, ln) {
				continue
			}
			if i == len(lines)-1 && len(lines) > 1 && isHeader(lastCount, ln) {
				continue
			}
			kept = append(kept, ln)
		}
		if len(kept) > 0 {
			out = append(out, strings.Join(kept, "\n"))
		}
	}
	return out
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			out = append(out, t)
		}
	}
	return out
}
