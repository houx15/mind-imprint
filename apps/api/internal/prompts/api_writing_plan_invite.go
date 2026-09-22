package prompts

// Purpose: api/writing_plan_invite.go 的固定提示词与条件指令。
// Consumer: internal/api/writing_plan_invite.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// WritingPlanInviteNudge retains the production text of writingPlanInviteNudge.
// writingPlanInviteNudge 是重试那一轮补上去的话。
//
// 只说犯的那一处 + 这一轮该做什么，不把整段规矩重念一遍 —— 它上一轮读过了。
const WritingPlanInviteNudge = `服务端检查显示，这份计划已有中心论点，分论点与材料数量也已满足本次篇幅的要求。
上一轮仍在提问，请将回复调整为开始写作的邀请。

这一轮是请她去写的那一轮。重新输出一次 JSON：

- reply：先说清现有内容为什么足以开始写作（具体到她写下的那几句，不要说「很完整」
  这种空话），然后请她开始写。**一个问号都不要有。**
- add：可以是空数组。这一轮不必再加节点。
- ready：true。

只描述已有观点、理由和材料的对应关系，不用「站得住」等笼统评价或比喻。其他建议留到段落写出后，再结合具体文字反馈。`
