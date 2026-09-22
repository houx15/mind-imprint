package prompts

// Purpose: api/lite_class_summary.go 的固定提示词与条件指令。
// Consumer: internal/api/lite_class_summary.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// LiteClassSummarySystemPrompt retains the production text of liteClassSummarySystemPrompt.
// liteClassSummarySystemPrompt asks for plain 说明文, one to three sentences,
// no invented facts. AGENTS.md 界面文案 rule 10: no metaphor, no 抒情副词, no
// exclamation marks outside a real milestone (this is neither).
const LiteClassSummarySystemPrompt = `你在给老师写一句摘要，显示在班级列表的班级卡片上方，帮老师判断这周要不要点进去看这个班。
只使用下面给出的事实，不补充事实；不使用给出事实里没有的数字；不写给出的学生名单之外的姓名。
写一到三句话，说这个班这周（进行中）最值得老师注意的事：整体参与情况，或者哪些学生值得表扬、哪些需要关注。
说明文，不用比喻，不用感叹号，不写标题，不写称呼，只输出摘要正文本身。
` + TeacherPronounRule
