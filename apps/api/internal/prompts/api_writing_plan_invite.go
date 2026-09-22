package prompts

// Writing prompt text. Builders choose the applicable instructions; output contracts remain in each prompt.
// Comments are maintenance notes and are not sent to the model.
const WritingPlanInviteNudge = `服务端检查显示，这份计划已有中心论点，分论点与材料数量也已满足本次篇幅的要求。
上一轮仍在提问，请将回复调整为开始写作的邀请。

本轮帮助学生从规划转入写作。请重新输出完整 JSON：

- reply：先说清现有内容为什么足以开始写作（具体到学生写下的那几句，不要说「很完整」
  这种空话），然后请学生开始写。使用说明和邀请结束本轮，不再提出问题。
- add：可以是空数组。这一轮不必再加节点。
- ready：true。

只描述已有观点、理由和材料的对应关系，让学生能看出开始写作的具体依据。其他建议留到段落写出后，再结合具体文字反馈。`
