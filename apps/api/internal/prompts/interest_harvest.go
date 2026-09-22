package prompts

// Purpose: interest/harvest.go 的固定提示词与条件指令。
// Consumer: internal/interest/harvest.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// InterestHarvestSystemPromptHead retains the production text of harvestSystemPromptHead.
// harvestSystemPromptHead 是候选清单之前的那一段。清单由 interests.PromptList()
// 在 BuildHarvestPrompt 里拼进来，不写死在常量里 —— 词表改了 prompt 要跟着改，
// 而两份手抄的清单一定会漂。
const InterestHarvestSystemPromptHead = `学生完成了一次阅读、写作或项目活动。请根据学生自己的文字，识别其中体现的关注方向，用于更新兴趣树。

从下表选择对应的领域，不新增表外名称或 id。结果中的 note 会直接展示给学生，用“你”称呼学生，只说明本次表达体现的关注方向。

`

// InterestHarvestSelectionRules retains the production text of harvestSelectionRules.
const InterestHarvestSelectionRules = `怎么算选中一个领域：
- 以学生自己的文字为依据，区分阅读材料的主题与学生实际表达的关注。
- 材料中出现某个话题，不足以判断学生对它感兴趣。学生对具体内容的提问、评价或进一步了解的意愿，可以作为依据。例如：“我想知道抽卡概率是怎样计算的”。`

// InterestQuizSelectionRules retains the production text of quizSelectionRules.
const InterestQuizSelectionRules = `怎么算选中一个领域：
- 输入包含学生主动选择的作品及理由。结合两者判断关注方向，尤其关注学生解释为什么选择该作品的部分。
- 判断学生的理由关注哪个方面：同一个角色，有人关注角色的处境，有人关注角色的选择，也有人关注画风。学生给出的理由是判断关注方向的依据。
- 作品名本身不是领域（《进击的巨人》不等于「动画」）；根据学生给出的理由选择对应领域。`

// InterestHarvestSystemPromptTail retains the production text of harvestSystemPromptTail.
const InterestHarvestSystemPromptTail = `
字段要求：
- id：**上表里的 id 原样照抄**，不要改写，不要翻译，不要自己发明。
- note：一句话，对学生说，说明学生关注该领域的哪一方面。不超过 40 字。
- evidence：**学生自己写的原话**，原样摘录，不要改写、不要总结。
  找不到能作为根据的原话，就不要输出这一条。

最多选择三个领域，数量取决于学生原话中的依据；没有合适领域时返回空数组。

只输出一个 JSON 对象，不要任何解释：
{"keywords":[{"id":"","note":"","evidence":""}]}`
