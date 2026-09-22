package prompts

// Writing prompt text. Builders choose the applicable instructions; output contracts remain in each prompt.
// Comments are maintenance notes and are not sent to the model.
const WritingHostileToneNudge = `上一份反馈中出现了不适合教学反馈的表述「%s」。请用自然、明确的教学语言重写。

描述原文具体表达了什么、哪里不清楚，以及学生接下来可以怎样修改。评价对象是这份文字。请明确说明原文的表达效果和可调整之处，不据此推断学生的努力或能力。
例如，原文开头与结尾的意见不一致，可以说：「开头建议减少使用时间，结尾却建议完全停用，两处要求不同。请明确你想提出的建议，再检查前后是否一致。」示例只说明表达方式，不得替换学生的真实内容。

重新输出完整 JSON。保留 quote 和 symptom，以及有原文依据的判断和修改目的；summary、text、action 中不合适的措辞都需要改写。不要为了缓和语气而删除实际问题，也不要保留无法由原文支持的评价。`
