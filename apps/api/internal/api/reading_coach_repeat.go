package api

// reading_coach_repeat.go — 这一步卡住了没有。
//
// # 它修的是什么
//
// 2026-09-10 的线上走查，通读那一步：印记 连着七轮问同一件事，一轮比一轮硬。
//
//	「读完告诉我一声。」
//	「你刚才说的是哪一段？我要的是全文——从第1段一路读到第18段。」
//	「你刚刚又在说「这一段」。……先给我一个明确的答复：」
//	「我听到你在说「这一段」，但我需要你告诉我一件具体的事：是，还是不是？」
//	「你现在读到第几段了？给我一个数字——比如「第5段」或「全部读完了」。」
//
// 那是审问，不是带读。而 prompt 里早就写着「一次说不通就换个说法，不要把同一句
// 指令再讲一遍」和「不要去评论『她还没做到』这件事本身」—— 它每一轮都在遵守
// 自己以为的职责（这一步还没完成，所以不能推进），所以规矩再写十遍也没用。
// 它需要**知道自己卡住了**。
//
// # 为什么判的是「没推进」，不是「说了同样的话」
//
// 第一版用的是文本相似度（中文二元组）。实测那七句两两之间只有 0.05–0.67 ——
// 它们重复的是**意图**，不是词。想抓全它们，阈值得压到跟「正常推进的两轮」
// （实测 0.00–0.06）只差一倍多的位置，那是拿十个样本去拟合一条线。
//
// 而真正要问的那件事本来就有一个精确答案：**这一步在她身上待了几轮了。**
// 上一步的 completed_at 就是这一步开始的时刻，数一数那之后 印记 说过几句话
// 就行。没有阈值要调，没有措辞能绕过去 —— 换十种说法问「读完了吗」，
// 计数一样在涨。
//
// # 喂回去的那句话只说一次
//
// 🚨 卡住了才加这一节，没卡住一个字都不加。常驻的提示会抢掉这一轮真正该做的事
// —— PBL 那边栽过（[[pbl-refeed-one-produce-slot-2026-09-05]]：一条常驻的
// 「工具被撤掉了」提示让 印记 六轮都在补那张卡，她的主页三处一直是空的）。

import (
	"time"

	"mindimprint/api/internal/store/sqlc"
)

// coachStuckTurns —— 同一步上说到第几句算卡住。
//
// 3：第一句是领这一步，第二句是换个说法再说一遍（prompt 明确允许，而且经常
// 管用），第三句就已经是「同一件事问第三遍」了。走查里那一串有七句。
const coachStuckTurns = 3

// coachStalledTurns —— 同一步上说到第几句就**强制往下走**。
//
// 6：提示已经给过了（第 3 句），又过了三轮她还在这一步。再耗下去没有第七种
// 结果 —— 这正是通读那一步变成审问的那个形状，只是换了一步。
//
// 🚨 为什么这条必须是代码。prompt 里写着「她确实做完了就 advance」「一次说不通
// 就换个说法」，而实测下来模型**极少主动推进**：一条 150 步的走查里八步只走完
// 两步，每一步都在「再找一句」「再说说看」之间来回。她不是没做，是做了也不算数。
//
// 往下走不等于判她做完了 —— reading_task 上留的是 'done'，而她在这一步里说过
// 的每一句话都在转写里，过程评估读的是那个（铁律④）。把她钉在原地才是真的
// 丢东西：她会直接关掉页面。
const coachStalledTurns = 6

// coachStepStuck —— 当前这一步是不是已经问过 coachStuckTurns 轮还没动。
//
// tasks 必须按 position 升序（ListReadingTasks 就是这么给的），msgs 是这次
// prompt 用的那段转写。
func coachStepStuck(tasks []sqlc.ReadingTask, msgs []sqlc.AtomMessage) bool {
	return coachTurnsOnCurrentStep(tasks, msgs) >= coachStuckTurns
}

// coachStepStalled —— 这一步耗得太久了，该由我们替她往下走。见 coachStalledTurns。
func coachStepStalled(tasks []sqlc.ReadingTask, msgs []sqlc.AtomMessage) bool {
	return coachTurnsOnCurrentStep(tasks, msgs) >= coachStalledTurns
}

// coachTurnsOnCurrentStep —— 当前这一步开始之后，印记 说过几句话。
//
// 「这一步开始的时刻」= 它前面那一步落定的时刻。它是第一步的话就没有这个时刻，
// 从头数。
func coachTurnsOnCurrentStep(tasks []sqlc.ReadingTask, msgs []sqlc.AtomMessage) int {
	current := currentReadingTask(tasks)
	if current == nil {
		return 0
	}
	// 这一步是什么时候变成「当前」的：它前面那一步落定的时刻。它是第一步的话
	// 就没有这个时刻，从头数。
	var since time.Time
	for i := range tasks {
		if tasks[i].ID == current.ID {
			break
		}
		if tasks[i].CompletedAt.Valid && tasks[i].CompletedAt.Time.After(since) {
			since = tasks[i].CompletedAt.Time
		}
	}
	said := 0
	for _, m := range msgs {
		if m.Role != "ai" {
			continue
		}
		if since.IsZero() || m.CreatedAt.After(since) {
			said++
		}
	}
	return said
}
