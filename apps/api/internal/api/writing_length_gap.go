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
	b.WriteString("她现在的正文：" + strconv.Itoa(now) + " " + unit +
		"（离目标还差 " + strconv.Itoa(gap) + " " + unit + "）\n")

	switch {
	case float64(gap) <= float64(target)*writingLengthNearEnough:
		// 够了（含已经超出）。要说出口 —— 她不知道自己可以收工。
		b.WriteString("篇幅已经到了。**这一轮要说清楚这件事**：不要再让她扩写，" +
			"接下来只谈哪里还不够好；她要是问「够了吗」，就直接回答够了。\n")
	case gap > target/4:
		// 缺口比四分之一还大：一句话的动作在这儿是没用的。
		b.WriteString("🚨 这个缺口**一句话补不上**。不要给「加一个场景」「改一句」" +
			"这种一句话尺寸的动作然后说它能补上这些字 —— 她上一轮照做了，缺口一点没动。\n" +
			"这一轮要指出**哪一段还欠一整层**（哪条理由底下没有她见过的事、" +
			"哪个例子只讲了发生什么没讲为什么），并说清那一层展开之后大概能占多少。\n" +
			"🚨 仍然不许替她写，也不要让她凑字数：说的是往哪儿深下去，不是再写够多少字。\n")
	default:
		b.WriteString("缺口不大，一两句具体的展开就能补上。\n")
	}
	return b.String()
}
