package api

import (
	"regexp"
	"strconv"

	"mindimprint/api/internal/store/sqlc"
)

// reading_coach_align.go —— 印记嘴上领她去读的那几段，和清单上亮着的那一步，
// 必须是同一件事。
//
// 2026-09-17 入口走查，两条路上各录到一次，方向正好相反：
//
//   - 粘贴那篇 18 段的文章：清单停在「通读第9–11段」，印记说「接下来读第12到
//     15段」，advance 给了空。她读完 12–15 段回来，印记又说「你刚才读的那一段
//     是第9到第11段」—— 清单和对话从这里开始各说各的。
//   - 上传那篇中文议论文：清单在「通读第5–6段」，印记说「走，往下读第5到
//     第6段」，advance 却给了 done —— 她一个字没读，第 5–6 段那一步就打了勾，
//     进度盘跳到「通读第7–8段」，而对话还在问第 5–6 段。
//
// 两次都是模型对「当前是哪一步」的理解和服务端差了一格。advance 是模型给的，
// 领她去哪几段也是模型说的，而后者她看得见 —— 所以以话为准，改 advance。
// 判据写在代码里，不再加一段提示词（[[prompt-twice-then-make-it-checkable-2026-09-12]]）。

// readDirective 抓「接下来 / 往下 / 现在 … 读 / 看 第 X（到 / 和 / – 第 Y）段」。
// 只认带方向词的说法：「你读完了第9到11段」「第3段那句」都不是在领她去读。
var readDirective = regexp.MustCompile(
	`(?:接下来|往下|现在|下面|然后|继续|再)[^。？！?!\n]{0,8}?(?:读|看)[^。？！?!\n]{0,4}?第\s*(\d+)\s*段?\s*(?:(?:到|至|和|–|—|-|~|～|、)\s*第?\s*(\d+)\s*)?段`)

// readStepRange 读清单上通读那一步的段号：「通读第12–15段·…」「通读第9段·…」。
var readStepRange = regexp.MustCompile(`^通读第(\d+)(?:–(\d+))?段`)

// directedReadStart 返回这句话最后领她去读的那一部分的起始段号；没有就是 0。
// 取最后一处：前面的「第3段那句」常常是在接住她刚才的话。
func directedReadStart(reply string) int {
	all := readDirective.FindAllStringSubmatch(reply, -1)
	if len(all) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(all[len(all)-1][1])
	return n
}

// alignAdvanceWithReply 按回复里领她去读的那一部分，校正这一轮的 advance。
//
//   - 领她去读的就是**当前这一步**：这一步刚开始，不能是 done（skipped 照放行）。
//   - 领她去读的是**紧接着的下一步**：当前这一步已经结束，advance 给 done。
//   - 别的情况（没领她去读、指向更远的一步、指向做完的一步）：不动。
func alignAdvanceWithReply(tasks []sqlc.ReadingTask, advance, reply string) string {
	start := directedReadStart(reply)
	if start == 0 {
		return advance
	}
	cur := -1
	for i := range tasks {
		if tasks[i].Status == "pending" {
			cur = i
			break
		}
	}
	if cur < 0 {
		return advance
	}
	startOf := func(t sqlc.ReadingTask) int {
		m := readStepRange.FindStringSubmatch(t.Label)
		if m == nil {
			return 0
		}
		n, _ := strconv.Atoi(m[1])
		return n
	}
	if tasks[cur].Kind == string(taskRead) && startOf(tasks[cur]) == start {
		if advance == "done" {
			return ""
		}
		return advance
	}
	if next := cur + 1; next < len(tasks) && tasks[next].Status == "pending" &&
		tasks[next].Kind == string(taskRead) && startOf(tasks[next]) == start && advance == "" {
		return "done"
	}
	return advance
}
