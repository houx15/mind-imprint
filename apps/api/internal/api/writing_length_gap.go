package api

import (
	"strconv"
	"strings"

	"mindimprint/api/internal/store/sqlc"
)

// 她离目标篇幅还差多少，以及**差这么多的时候该给什么样的动作**。
//
// 🚨 **一句话的动作，补不了两百字的缺口。**
//
// 2026-09-12 第三十三轮走查，中文那一路四条卡壳里三条是同一件事，她说得比我
// 清楚：
//
//	「它说这样能帮我凑够两百字，但就加一句话怎么可能多出两百字啊……」
//	「它只让我在第四段加一个场景，但我还差两百多字，不知道除了加场景还能在
//	  哪扩充，怕加了还是不够字数。」
//	「印记只说了第二段不用改了，但没说接下来怎么办。字数还没到800，是让我自己
//	  随便加，还是点完成这篇？」
//
// 上文里本来只有一行「目标篇幅：约 800 字」—— 一个目标，没有**她现在在哪儿**，
// 更没有**还差多少**。于是陪练每一轮都在给「改一句」这种尺寸的动作，而她手上
// 的缺口是两百字。她照做了，缺口一点没动，只好自己猜「是不是随便加就行」。
//
// 这和产品负责人同一天提的第六条是同一个盲点，只是晚了一步：规划那边我按篇幅
// 把判据放开了（writingPlanNeedOf），而真正写的时候，陪练照样不知道差多少。
//
// 服务端数得出来的事实，就不要让模型去猜 —— 同 writingPlanShape.promptBlock
// 那一条。给数字，并且按缺口的大小说清这一轮该给多大的动作。
//
// 🚨 不要把它变成催她凑字数。铁律①：正文是她写的，我们只说该往哪儿使劲。
// 所以「还差多少」后面跟的永远是**去哪里深下去**，不是「再写够两百字」。

// writingLengthNearEnough 差多少以内就算到了。
//
// 一成。差 5% 去提醒她「还差 40 字」，只会让她去凑；差得远的时候不说，她就
// 一直在拿一句话的动作填一段的缺口。
const writingLengthNearEnough = 0.10

func writingLengthGapBlock(wr sqlc.Writing, draftBody string) string {
	if wr.TargetWords == nil {
		return ""
	}
	target := int(*wr.TargetWords)
	if target <= 0 {
		return ""
	}
	body := strings.TrimSpace(draftBody)
	if body == "" {
		// 还没开始写。这时候说「差 800 字」是废话，而且像催。
		return ""
	}
	now := countWordsForLang(body, wr.Lang)
	unit := lengthUnit(wr.Lang)
	gap := target - now

	var b strings.Builder
	b.WriteString("学生现在的正文：" + strconv.Itoa(now) + " " + unit +
		"（离目标还差 " + strconv.Itoa(gap) + " " + unit + "）\n")

	switch {
	case float64(gap) <= float64(target)*writingLengthNearEnough:
		// 够了（含已经超出）。要说出口 —— 她不知道自己可以收工。
		b.WriteString("正文已达到目标篇幅附近，请明确告知学生。无需为字数继续扩写；" +
			"如有实际表达问题，再据此提出修改建议。\n")
	case gap > target/4:
		// Focus expansion on missing substance, preserving the measured threshold.
		b.WriteString("正文与目标篇幅相差较多。请结合稿件指出一处值得充分展开的内容，" +
			"例如尚未解释的理由、未展开的真实材料或材料与观点的联系，并说明展开目的与大致篇幅。\n" +
			"由学生补充内容，不代写，也不以重复或无关内容增加字数。\n")
	default:
		b.WriteString("篇幅差距较小，可结合实际内容建议简短补充；不要保证固定句数一定达到目标。\n")
	}
	return b.String()
}
