package prompts

// Purpose: awakening/selection.go 的固定提示词与条件指令。
// Consumer: internal/awakening/selection.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// AwakeningSelectionRules retains the production text of selectionRules.
// selectionRules 是这一次调用的判据段落。
//
// 和采集那份的区别只有一处，而那一处是反的：采集要防「材料的话题被当成她的
// 兴趣」，这里没有材料，她说的每个字都是她自己敲的。
const AwakeningSelectionRules = `怎么算选中一个领域：
- 依据学生在本次对话中写下的回答，结合所选对象和理由判断关注方向。提到某个对象可以提供线索，具体解释更能说明关注的是哪个方面。
- 判断学生的理由关注哪个方面：同一件事，有人关心它怎么运作，有人关心它对谁有影响，
  有人关心它好不好看。学生反复讨论的方面可以作为判断关注方向的依据。
- 作品名、游戏名、人名本身不是领域（《进击的巨人》不等于「动画」）。
  根据学生解释原因的原话选择对应领域。
- 综合各轮回答；后续明确表达的问题和作品设想可以补充或更新最初的兴趣描述。`

// AwakeningSelectionPromptTail retains the production text of selectionPromptTail.
const AwakeningSelectionPromptTail = `
字段要求：
- id：从上表选择，逐字复制，不翻译或新增。
- note：直接展示在学生的报告中。用“你”称呼学生，用一句不超过 40 字的话说明学生表达了对该领域哪一方面的关注，不推断性格或能力。
- evidence：从学生回答中原样摘录一段连续原话，不改写、总结或拼接。找不到支持该领域的原话时不输出该条。

最多选择三个领域，数量取决于依据；没有合适领域时返回空数组。
只输出一个 JSON 对象，不要任何解释：
{"keywords":[{"id":"","note":"","evidence":""}]}`

// AwakeningSelectionPromptHead retains the production text of selectionPromptHead.
const AwakeningSelectionPromptHead = `学生完成了一次兴趣探询。下面提供了学生关于兴趣对象、具体细节、相关经历、疑问、阅读方向、当前想法和作品设想的回答。请据此识别学生关注的领域。

判断学生正在关心哪几个领域，最多三个。

`
