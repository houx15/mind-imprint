package prompts

// Purpose: api/reading_coach_repeat.go 的固定提示词与条件指令。
// Consumer: internal/api/reading_coach_repeat.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// CoachStuckNudge retains the production text of coachStuckNudge.
// coachStuckNudge 是卡住时加进 prompt 的那一节。
//
// 它说的是**做什么**，不是「你错了」：给三条具体的出路。只说「别重复」的话，
// 模型会把这一节也当成一条要遵守的禁令，然后继续卡在原地 —— 换一个说法接着问。
const CoachStuckNudge = `
【这一步你已经带了三轮以上，她还停在这儿】
再说一遍不会有第四种结果。这一轮**换一件事做**，三选一：

- 出一张卡片，让她点着做。她答了，这一步就算做完 —— 这是最好的一条。
- 换一个更小、更具体的动作，指名到某一段：「第 3 段有三个数字，先看带百分号的那个。」
- 直接 advance "done" 往下走。这一步她已经做过一遍了，硬要一个标准答复不值得，
  下一步自然会暴露她有没有真读。

🚨 不要再问她「读完了吗」「是还是不是」「读到第几段了」这类要她报告进度的话。
你没有办法核实她的回答，所以问下去只会变成审问。
`
