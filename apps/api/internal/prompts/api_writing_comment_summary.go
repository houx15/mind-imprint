package prompts

// Writing prompt text. Builders choose the applicable instructions; output contracts remain in each prompt.
// Comments are maintenance notes and are not sent to the model.
const WritingSummaryAbsenceNudge = `上一份 summary 包含缺失表述「%s」，不符合该字段的要求。
summary 只概述文章已呈现的内容和表达效果；需要补充或修改的内容放在带有原文引句和具体建议的 points 中。
请重新输出完整 JSON，仅修改 summary，保留 points 不变。`
const WritingNoPointNudge = `上一份结果的 verdict 为 polish 或 revise，但没有通过校验的 issue。请核对判断与建议是否一致，重新输出完整 JSON：
- 有实际修改问题时，至少提供一条 kind 为 issue 的意见，包含独立的 quote 字段（逐字引自正文）、有效 symptom id、说明问题的 text，以及可执行的 action。
- 没有需要修改的问题时，将 verdict 改为 pass，summary 概述文章已有的内容和表达效果。
反馈会直接展示给学生，使用「你」称呼学生。`
const WritingCommentUnparseableNudge = `上一份结果无法解析为 JSON。请按原要求重新输出一个完整 JSON 对象。
仅输出该对象，不附解释或代码块标记。字符串中的引号须转义，数组与对象括号须正确配对。`
