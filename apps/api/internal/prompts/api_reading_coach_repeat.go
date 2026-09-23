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
【当前步骤已持续三轮以上】
请结合学生已完成的内容选择下一种帮助方式，避免重复相同提问：
- 当前没有打开的卡片且需要练习时，提供一张对应当前任务的卡片；学生提交后按该任务的完成条件处理。
- 学生尚需帮助时，把当前问题缩小到一个具体观察动作，并指出相关段落。
- 学生已经完成本步要求、只是表达不同于预设答案时，advance="done"，进入下一步。
不要通过反复询问「读完了吗」或阅读进度来判断理解。明确跳过、索答及提示请求仍按对应优先规则处理；持续轮数本身不代表任务已完成。
`
