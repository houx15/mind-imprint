package api

import "strings"

// writing_plan_edit_claim.go —— 印记 不能说自己改了图上已有的那一条。
//
// # 为什么存在（2026-09-23，产品负责人第 6 条）
//
//	「during writing, if we talked with AI that we want to change the central
//	  topic texts. the mindmap is not modified.」
//
// 她说得对，而且这**不是一个可以直接修掉的 bug** —— 规划这一路本来就改不动
// 任何既有节点。writing_plan.go 的文件头把它写成第一条硬规则：
//
//	「**只加，不改不删。** 这一路唯一能碰提纲的写操作是
//	  InsertWritingOutlineNode……那是她自己的编辑权。这不是 prompt 里的请求，
//	  是这个文件里根本没有那两个调用。」
//
// 模型的回复里甚至拿不到节点 id（【当前的图】那一栏只渲染文字），
// 所以就算有那个调用，它也指不到任何一条。
//
// # 那么真正坏掉的是哪一件
//
// 是**她和印记谈好了，然后什么都没发生**。印记说「好，我们把中心论点改成
// ……」，图纹丝不动，她得自己猜是不是坏了。
// 一句做不到的承诺，比一句「这个你自己在图上点一下就能改」糟得多
//（memory `ai-errors-must-surface-never-fake`：模型给一句听起来对的话，
// 而那件事没发生，她面对的是一个死掉的终端）。
//
// 所以这里做两件事，一件都不越过那条硬规则：
//
//  1. 判据（这个文件）：回复声称自己改了/更新了图上某一条时，这一轮重来一次。
//     形状照 liteWorkspaceHome.falseClaim ——「回复让老师点下方按钮，
//     而本轮没有调用 open_page，下方没有按钮」是同一类失败。
//  2. 提示词里一句陈述句，告诉它这种情况该怎么做：把改法交给她。
//
// # 判据收得紧：要「改」这个动作 **加上** 「已经做完」或者「我来做」
//
// 只占一样的话一律放过：
//   - 「要不要把它换成一句更具体的？」—— 有动作，是在问她。
//   - 「我已经记下来了」—— 有完成态，说的不是改图。
//   - 「我建议你把它改成……」—— 有「我」也有动作，但动手的是她。
//     所以收的**不是裸的「我」**，是「我来 / 我把 / 我帮你」这几种直接带着
//     那个动作的说法。
//
// 按**句**判，不按整段判：一段里分别出现「已经」和「改成」多半是两件事。

// planEditVerbs —— 改动图上已有那一条的说法。
var planEditVerbs = []string{
	"改成", "改为", "换成", "换为", "更新为", "更新成", "修改为", "修改成",
	"替换成", "替换为", "调整为", "调整成",
}

// planDoneMarkers —— 「这件事已经做完了」的说法。
var planDoneMarkers = []string{
	"已经", "已为", "已帮", "已替", "已把", "已将", "已更新", "已修改", "已调整",
}

// planAgentMarkers —— 「**我**来做这件事」的说法。
//
// 和完成态分开一张表，因为它们挡的是两种不同的话：
//   - 完成态：「已经改成了」—— 说一件没发生的事已经发生。
//   - 主语是我：「我来把它改成……」—— 许一个它做不到的承诺。
//
// 🚨 **不收裸的「我」。**「我建议你把它改成……」里也有「我」，而那一句是对的：
// 动手的是她。收的是「我」直接带着那个动作的几种说法。
var planAgentMarkers = []string{
	"我帮你", "我替你", "我给你", "我为你", "帮你把", "替你把", "给你把",
	"我来", "我把", "我将", "我已",
}

// writingPlanClaimsAnEdit 返回回复里第一句声称改过图的话，没有就返回 ""。
//
// 按句判：整段里分别出现「已经」和「改成」两个词不算 —— 那多半是两件事。
func writingPlanClaimsAnEdit(reply string) string {
	for _, line := range strings.Split(reply, "\n") {
		for _, sent := range splitCJKSentences(line) {
			if sentenceClaimsAnEdit(sent) {
				return strings.TrimSpace(sent)
			}
		}
	}
	return ""
}

func sentenceClaimsAnEdit(sent string) bool {
	var verb bool
	for _, v := range planEditVerbs {
		if strings.Contains(sent, v) {
			verb = true
			break
		}
	}
	if !verb {
		return false
	}
	for _, d := range planDoneMarkers {
		if strings.Contains(sent, d) {
			return true
		}
	}
	for _, m := range planAgentMarkers {
		if strings.Contains(sent, m) {
			return true
		}
	}
	return false
}

// writingPlanEditClaimNudge —— 重问那一轮加在后面的那句话。
//
// 说的是**真正发生的那件事**，不是一句泛泛的「请重写」：memory
// `judge-the-delivered-artifact-2026-09-21` 里那条重问原来写「points 却是
// 空的」，而里面有一条 good，模型对不上号就原样再回了一遍。
const writingPlanEditClaimNudge = `你上一轮的回复说你已经改了图上的某一条，但这一轮没有、也不可能改动图上已有的节点：
这条路只能新增节点，改写和删除是学生自己在图上做的。她刚刚读到的是一句没有发生的事。

请重写这一轮的回复：不说自己改过任何一条；她想改图上已有的那一条时，说清楚在图上点那一条就能改，
并把改好之后那一句写出来供她参考。add 照常只放她这一轮新说出来的内容。`
