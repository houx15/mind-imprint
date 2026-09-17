package api

import (
	"strings"

	"mindimprint/api/internal/store/sqlc"
)

// atom_report_boards.go —— 阅读报告上「阅读成果」那一节：她摆过的板，原样留下。
//
// 同事 2026-09-17 的阅读模块 PRD：
//
//	阅读结束保留可编辑成果：议论文为论证图，说明文为知识结构／流程图，
//	记叙文为事件／人物变化图，新闻报道为事件与来源对照。
//
// 这一节是那份成果的第一半：**她摆的东西留下来**，按体裁给它一个名字。
// 和段落工具那一节（atom_report_toolkit.go）一样是**确定性**的 —— 数的是
// 已经存下来的 atom_message 行，不花一次模型调用，也就不会编。
//
// 🚨 板上的每一句都是**文章的原话**，不是她写的句子。报告上每一段引文都要说清
// 是谁的话（R4），所以这一节的字段名和界面上的标题都说「你摆的」，而不是
// 「你写的」——她做的是判断，判断本身才是她的产出。

// reportBoardGroup 是一块标注板上的一个格子。
type reportBoardGroup struct {
	Bin    string   `json:"bin"`
	Quotes []string `json:"quotes"`
}

// reportBoard 是她摆完的一块板。
type reportBoard struct {
	// Kind: "label"（标注板）或 "order"（排序板）。
	Kind string `json:"kind"`
	// Title 按体裁来：议论文是论证图，报道是事实与来源对照……
	Title string `json:"title"`
	// Groups 是标注板的那几个格子；Order 是排序板上她排的先后。
	Groups []reportBoardGroup `json:"groups,omitempty"`
	Order  []string           `json:"order,omitempty"`
}

// reportBoardTitles —— 每种体裁的标注板在报告上叫什么。PRD 的原话。
var reportBoardTitles = map[string]string{
	genreArgument:  "论证图",
	genreReport:    "事实与来源对照",
	genreExplain:   "知识结构",
	genreNarrative: "人物与描写",
}

const reportOrderBoardTitle = "事件时间线"

// 一份报告上最多留两块板：一篇文章里本来就最多摆两块（maxLabelBoards）。
const reportBoardsMax = 4

// buildReportBoards 把她摆过的板收成一节。一块都没摆就是 nil —— 这一节整个
// 不显示，而不是一个空框。
func buildReportBoards(msgs []sqlc.AtomMessage, genre string) []reportBoard {
	title := reportBoardTitles[validateGenre(genre)]
	if title == "" {
		// 体裁认不出来（老数据、模型漏填）：说「你摆的板」，不猜。
		title = "你摆的板"
	}
	out := make([]reportBoard, 0, 2)
	for _, m := range msgs {
		if m.Role != "student" || len(m.Payload) == 0 {
			continue
		}
		ans := coachAnswerFromPayload(m.Payload)
		if ans == nil {
			continue
		}
		switch strings.TrimSpace(ans.Type) {
		case coachCardLabelRoles:
			if groups := parseLabelBoardGroups(ans.Choice); len(groups) > 0 {
				out = append(out, reportBoard{Kind: "label", Title: title, Groups: groups})
			}
		case coachCardOrderEvents:
			if order := parseOrderBoardLines(ans.Choice); len(order) > 0 {
				out = append(out, reportBoard{Kind: "order", Title: reportOrderBoardTitle, Order: order})
			}
		}
		if len(out) == reportBoardsMax {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseLabelBoardGroups 读她那条作答：一行「格子名：」，下一行是那句原文
// （composeBoardAnswer 写的格式）。格子按第一次出现的顺序摆。
func parseLabelBoardGroups(choice string) []reportBoardGroup {
	lines := strings.Split(choice, "\n")
	order := make([]string, 0, 4)
	byBin := map[string][]string{}
	for i := 0; i+1 < len(lines); i++ {
		head := strings.TrimSpace(lines[i])
		if !strings.HasSuffix(head, "：") {
			continue
		}
		bin := strings.TrimSuffix(head, "：")
		if !isAnyBoardLabel(bin) {
			continue
		}
		quote := strings.TrimSpace(lines[i+1])
		if quote == "" {
			continue
		}
		if _, seen := byBin[bin]; !seen {
			order = append(order, bin)
		}
		byBin[bin] = append(byBin[bin], quote)
	}
	out := make([]reportBoardGroup, 0, len(order))
	for _, bin := range order {
		out = append(out, reportBoardGroup{Bin: bin, Quotes: byBin[bin]})
	}
	return out
}

// parseOrderBoardLines 读排序板的作答：一行「第N：」，下一行是那句原文
// （composeOrderAnswer 写的格式）。
func parseOrderBoardLines(choice string) []string {
	lines := strings.Split(choice, "\n")
	out := make([]string, 0, coachOrderMaxOptions)
	for i := 0; i+1 < len(lines); i++ {
		head := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(head, "第") || !strings.HasSuffix(head, "：") {
			continue
		}
		if quote := strings.TrimSpace(lines[i+1]); quote != "" {
			out = append(out, quote)
		}
	}
	return out
}
