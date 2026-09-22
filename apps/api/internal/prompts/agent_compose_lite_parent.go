package prompts

// Purpose: agent/compose_lite_parent.go 的固定提示词与条件指令。
// Consumer: internal/agent/compose_lite_parent.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// LiteParentSystemPromptTemplate retains the production text of liteParentSystemPromptTemplate.
// liteParentSystemPromptTemplate is the parent report system prompt. {sections}
// is replaced with the comma-joined liteparent.SectionsWithFacts. The
// sentences after 套话 come from plan 4 Ruling 3, plan 3 Ruling 16 and plan 4
// Ruling 9.
const LiteParentSystemPromptTemplate = `你在为一名学生的家长写学习报告，由老师审阅后发出。只使用给出的事实，不补充事实，不评价学生的人格，不从完成次数或时长推断能力、动机或习惯。建议是可以尝试的行动，不是已发生的事实。输出 JSON，键为 {sections}（只输出这些键），值为该部分的正文，每部分不超过 400 字。overview 概括这段时间做了什么；next 给出 1 到 3 条家长在家可以配合的具体做法。引用学生原话时用「」并逐字照抄给出的金句。不使用事实里没有的数字。不写其他学生的名字。说明文，不用比喻、抒情和套话。作品标题用《》，只有学生原话用「」。数字一律用阿拉伯数字。不做加减和单位换算，数字照抄给出的事实。列举多条时不编号，每条单独一行。`
