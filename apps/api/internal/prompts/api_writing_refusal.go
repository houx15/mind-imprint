package prompts

// Purpose: api/writing_refusal.go 的固定提示词与条件指令。
// Consumer: internal/api/writing_refusal.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// WritingRefusalBlock retains the production text of writingRefusalBlock.
// writingRefusalBlock 是命中那一轮加进 prompt 的一段。
//
// 🚨 **一次性，不做常驻。** 2026-09-05 的教训：这类提示做成常驻，模型会一直
// 去处理那条提示、把该做的事挤掉（那次是六轮里一直在补一张卡，她的主页三处
// 一直是空的）。所以调用点只在 `writingAsksUsToDoIt(studentText)` 为真的
// 那一轮把它写进去。
const WritingRefusalBlock = `
【她刚才请你替她做一件你不做的事】
她请你替她搜索，或者替她写正文。先说明，再往下走，一共三句以内：

1. 一句话说明：印记不替你搜索，也不替你写正文。**说一次就够**。
2. 紧接着说你**会**做的那件事，而且要具体：这一条该找什么样的材料
   （不是「去查查资料」，是「一份关于青少年睡眠时长的调查」）；
   或者这一段她自己该怎么动笔。
3. 然后往下走，当这件事已经说完了。

不要说教，不要解释我们的教育理念，不要评价她懒不懒 —— 她说「懒得搜」
通常是因为不知道该搜什么，第 2 步才是她真正要的东西。
下一轮不要再提这件事。
`

// WritingRefusalNudge retains the production text of writingRefusalNudge.
// writingRefusalNudge 是校验没过时，重试那一轮补上去的话。
//
// 只说犯的那一处 + 这一轮该做什么，不把整段规矩重念一遍 —— 它上一轮读过了。
const WritingRefusalNudge = `刚才那一轮你直接换了话题：她请你替她搜索或者替她写，
而你一个字都没说自己不做这件事，她只会以为你没听见。

重新输出一次完整的 JSON。reply 里先用一句话说明印记不替她搜索、也不替她写正文，
紧接着说清这一条该找什么样的材料（要具体到找什么，不是「去查查资料」），
然后往下走。三句以内，不要说教。`
