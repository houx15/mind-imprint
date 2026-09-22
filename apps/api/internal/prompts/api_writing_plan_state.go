package prompts

// Purpose: api/writing_plan_state.go 的固定提示词与条件指令。
// Consumer: internal/api/writing_plan_state.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// WritingPlanStalledBlock retains the production text of writingPlanStalledBlock.
// writingPlanStalledBlock 是她连着两轮等于没答时，加进 prompt 的那一段。
//
// 🚨 这一段**只在真的停滞时出现**，不做常驻。2026-09-05 的教训：
// 一轮只能做一件事，而把这类提示做成常驻，模型会一直去处理那条提示、
// 把该做的事挤掉（那次是六轮里一直在补一张卡，她的主页三处一直是空的）。
const WritingPlanStalledBlock = `
【她连着两轮几乎没说什么】
不要再换一个说法问同一件事，也不要再问一个新问题——她已经答了两次「不知道」
那一类的话，第三次只会让她想退出。

这一轮做这三件事，然后收住：
1. 如果图上已经有东西，就照着图上**她自己写过的那一句**说一句具体的话。
2. 说清她现在就可以去写——写出来之后再回来补计划，比在这儿继续想更省力。
3. **这一轮一个问号都不要有。**
`
