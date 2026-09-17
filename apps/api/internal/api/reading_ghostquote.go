package api

import (
	"strings"

	"mindimprint/api/internal/quotematch"
	"mindimprint/api/internal/store/sqlc"
)

// reading_ghostquote.go —— 印记 在带读里引了一句**文章上、卡片上、她嘴里都
// 没有**的话。
//
// # 产品负责人 2026-09-17 逐字报的第 2 条
//
// 「文本内容和卡片上的内容对应不上。」两张截图：印记 在回复里写
//
//	请你把「Hundreds killed, Thousands wounded.」拖到证据格子下面
//
// 而她屏幕上那块板一共四张卡片，没有这一句；原文里也没有这一句 —— 原文写的是
// 「Hundreds of people have been killed. Thousands have been wounded.」，
// 两句话，被它压成了一句自己的话，再加上引号交给她去找。
//
// 她做不成这件事，而且**做不成的原因她看不见**：屏幕上的板和 印记 的话都
// 长得很正常，只是那句话不在板上。走查里她只会说「我找不到那句」。
//
// # 为什么是判据，不是提示词
//
// 写作室 2026-09-12 已经把同一条纪律走完了一遍（writing_ghostquote.go）：
// 提示词打过两次、第二十四轮反涨到八条，改成可验判据才收住。
// [[prompt-twice-then-make-it-checkable-2026-09-12]]。
//
// 这个房间的语料和写作室不是同一份，所以判据分开写，但三条规矩（归一化、
// 8 rune 下限、只认「」『』“”）共用 internal/quotematch —— 英文直双引号
// 不参与判定，英文文章的正文里本来就到处是它们。
//
// # 语料给得很宽
//
// 判错的方向和写作室一样：**宁可放过，不可误伤**。误判一次是一轮重来
// （花钱、让她多等），漏判一次只是维持现状。所以凡是她此刻在屏幕上读得到的
// 字，全部算数：
//
//   - 文章正文的每一段，还有标题；
//   - 这一轮那张卡片上的每一条（选项、生词），和卡片自己那道题；
//   - 导读卡上的字（在问什么、中心思想、结构、每个部分的名字）；
//   - 清单上每一步的名字和说明 —— 那几行字她在进度盘上读得到；
//   - 她自己说过的话，以及她刚做完的那副透镜的结论。
//
// 唯一**不收**的是 印记 自己说过的话：幻引的来源就是它的上文，收进来等于自证
// （写作室那份注释里写清了这一条）。
func readingQuoteCorpus(
	title string,
	blocks []Block,
	outline readingOutline,
	tasks []sqlc.ReadingTask,
	msgs []sqlc.AtomMessage,
	card *coachCard,
	lensDone *readingLensDone,
) string {
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")
	for _, blk := range blocks {
		b.WriteString(blk.Text)
		b.WriteString("\n")
	}
	b.WriteString(outline.OneLine)
	b.WriteString("\n")
	b.WriteString(outline.Gist)
	b.WriteString("\n")
	b.WriteString(outline.Shape)
	b.WriteString("\n")
	for _, p := range outline.Parts {
		b.WriteString(p.Title)
		b.WriteString("\n")
		b.WriteString(p.Does)
		b.WriteString("\n")
	}
	for _, t := range tasks {
		b.WriteString(t.Label)
		b.WriteString("\n")
		b.WriteString(t.Detail)
		b.WriteString("\n")
	}
	if card != nil {
		b.WriteString(card.Prompt)
		b.WriteString("\n")
		for _, o := range card.Options {
			b.WriteString(o.Quote)
			b.WriteString("\n")
		}
		for _, w := range card.Words {
			b.WriteString(w.Term)
			b.WriteString("\n")
		}
	}
	if lensDone != nil {
		b.WriteString(lensDone.Quote)
		b.WriteString("\n")
		b.WriteString(lensDone.Finding)
		b.WriteString("\n")
		b.WriteString(lensDone.VerdictReason)
		b.WriteString("\n")
	}
	// 她说过的话，包括她在卡片上摆的那一份（作答原样落在 payload 里）——
	// 印记 复述她刚摆的那一句是正常教学，不该判成幻引。
	for _, m := range msgs {
		if m.Role != "student" {
			continue
		}
		b.WriteString(m.Content)
		b.WriteString("\n")
		if ans := coachAnswerFromPayload(m.Payload); ans != nil {
			b.WriteString(ans.Choice)
			b.WriteString("\n")
		}
	}
	return quotematch.Normalize(b.String())
}
