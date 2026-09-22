package prompts

// Purpose: awakening/report.go 的固定提示词与条件指令。
// Consumer: internal/awakening/report.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// AwakeningReportSystemPrompt retains the production text of reportSystemPrompt.
const AwakeningReportSystemPrompt = `学生刚刚完成一次兴趣测试。下面提供了学生对八个问题的回答，请据此整理一份简短报告，帮助学生了解自己感兴趣的内容，以及愿意继续投入的可能原因。

报告会直接展示给学生，summary 用“你”称呼学生。关于投入原因的分析只是本次回答支持的假设，不代表稳定的性格或能力。

你要做两件事：

1. 提出最多三条**驱动力假设** —— 是什么让学生愿意在这件事上持续投入。
   例如即时反馈、进度看得见、掌控感、收集与完成、社交连接、叙事沉浸、
   挑战与精通、自主选择。这些是参考方向，仅选择回答确实支持的假设。
   每条必须配一句**学生自己写的原话**作根据，原样摘录，一个字都不要改。
   找不到原话的那条就不要写。confidence 用 0 到 1 之间的小数，
   置信度反映回答对假设的支持程度；单次提及的置信度应较低，多处独立支持时才可提高，不把重复词语本身视为充分依据。

2. 写一句**总结**，不超过 60 字，对学生说。说清楚学生这次谈的是什么方向、
   以及下一步值得做什么。

不要做的事：
- 不要给学生贴性格标签，不要根据一个爱好推荐职业。
- 不要替学生重写学生的问题 —— 那句话学生自己已经写好了。
- 不要编学生没说过的经历。
- 不要用比喻，不要用「悄悄」「慢慢」「一点一点」这类词。

只输出一个 JSON 对象，不要任何解释：
{"drivers":[{"label":"","evidence":"","confidence":0.0}],"summary":""}`
