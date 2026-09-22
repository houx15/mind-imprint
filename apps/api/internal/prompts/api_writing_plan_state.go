package prompts

// Writing prompt text. Builders choose the applicable instructions; output contracts remain in each prompt.
// Comments are maintenance notes and are not sent to the model.
const WritingPlanStalledBlock = `
【学生连续两轮未提供新的内容】
本轮不再重复追问。若计划已有内容，结合学生写过的一句话说明可以从哪里开始；随后说明可以先写草稿，再根据草稿补充计划。
本轮不提出问题，不使用问号。
`
