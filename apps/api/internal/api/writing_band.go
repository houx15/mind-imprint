package api

import "fmt"

// writing_band.go —— 通篇审阅那一条上的「当前档位」。
//
// # 为什么有它，以及它为什么不是模型给的
//
// 产品负责人 2026-09-25 选了「档要给学生看」。我当时提的顾虑是它撞上批改
// 提示词里那两句「不打分，不给等级」；她看过顾虑之后仍然要，所以做。
//
// 🚨 但做法不是让模型去评分 —— **档是服务端从这一轮真正挑出来的毛病算的**，
// 是 layerVerdictsOf 的纯函数。这样一来：
//
//   - 「不打分，不给等级」那两句**一个字都不用改**：它们管的是模型，而模型
//     仍然只负责找毛病，不负责发等级。
//   - 档是确定的、可解释的、可测的。模型评分会有第二套失败方式
//     （编一个分数、同一篇两次给两个分），而这里一个都没有。
//   - 她问「凭什么是这一档」，答案就在她眼前那几条意见上。
//
// # 只挂在通篇那一条上
//
// 🚨 五档是给**一整篇**用的尺子。把它套到一个自然段上没有意义 ——
// 一段写得再好也不构成一篇及格的文章，反过来也一样。所以 scope="block"
// 那一路不带档位（commentOnSnippet 不调这里）。
//
// # 这把尺子量的是什么
//
// 批改的筛子保证一轮只出**同一层**的问题（层序：立意 → 材料 → 结构 → 字句），
// 所以「现在卡在哪一层」这件事本身就是一个序。档位就是把它写出来：
//
//	5  四层都没挑出问题
//	4  只剩字句要磨
//	3  卡在结构
//	2  卡在材料
//	1  卡在立意
//
// 越靠上的层越值钱，这正是源材料那条性价比排序（先补要点 > 修 A 级错误 >
// 加高级句式）。所以档位和「下一步该做什么」是同一件事的两面 ——
// 它永远和这一轮那条意见一起出现，不单独露脸。

const (
	writingBandTop = 5
	writingBandMin = 1
)

// writingBandOf 算这一篇现在在第几档。
//
// points 是**她真的会看到的那几条**（validateCommentPoints 之后的），
// 不是模型原样回的 —— 服务端丢掉的那些不该影响她的档位。
func writingBandOf(points []CommentPoint) int {
	layers := layerVerdictsOf(Comment{Points: points})
	// 从最值钱的一层往下找：第一层有问题就卡在第一层。
	for i, layer := range []int{
		writingLayerClaim, writingLayerMaterial, writingLayerStructure, writingLayerSentence,
	} {
		switch layers[layer] {
		case writingVerdictPolish, writingVerdictRevise:
			// i=0（立意）→ 1 档，i=3（字句）→ 4 档。
			return writingBandMin + i
		}
	}
	return writingBandTop
}

// writingBandLabel 是她在屏幕上读到的那一句。
//
// 🚨 它必须说清**这把尺子量的是什么**，否则「第 3 档」会被读成一个考试分数。
// 界面文案规则：标签是名词，说明是一句话，不抒情、不安慰。
func writingBandLabel(band int) string {
	switch band {
	case 5:
		return "四层都没有挑出问题"
	case 4:
		return "内容和结构已经立住，剩下字句"
	case 3:
		return "内容立住了，结构还要理"
	case 2:
		return "观点清楚，材料还不够"
	case 1:
		return "先把观点定下来"
	}
	return ""
}

// writingBandScaleNote 是档位旁边那一行小字：这把尺子是怎么来的。
//
// 学生看见一个数字会默认它是分数。这一行的作用是把它改读成
// 「我现在卡在哪一层」——而那是她下一步能动手的东西。
const writingBandScaleNote = "档位按「现在卡在哪一层」算：立意 → 材料 → 结构 → 字句，越靠前越要紧。它不是分数。"

// writingBandText 把档位拼成一句完整的话，给不解析 JSON 的地方用
//（报告、教师端、一次 psql）。
func writingBandText(band int) string {
	if band < writingBandMin || band > writingBandTop {
		return ""
	}
	return fmt.Sprintf("第 %d 档 · %s", band, writingBandLabel(band))
}
