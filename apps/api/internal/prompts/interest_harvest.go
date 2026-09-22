package prompts

// Purpose: interest/harvest.go 的固定提示词与条件指令。
// Consumer: internal/interest/harvest.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// InterestHarvestSystemPromptHead retains the production text of harvestSystemPromptHead.
// harvestSystemPromptHead 是候选清单之前的那一段。清单由 interests.PromptList()
// 在 BuildHarvestPrompt 里拼进来，不写死在常量里 —— 词表改了 prompt 要跟着改，
// 而两份手抄的清单一定会漂。
const InterestHarvestSystemPromptHead = `你在读一个中学生刚刚完成的一件事，判断她正在关心
哪几个领域。

**你只能从下面这张表里选，不能自己造词。** 表里没有的东西，哪怕你觉得更贴切，
也不要输出 —— 输出了会被丢掉。

`

// InterestHarvestSelectionRules retains the production text of harvestSelectionRules.
const InterestHarvestSelectionRules = `怎么算选中一个领域：
- 她**自己写下的文字**里能找到根据。文章讲了什么不算，她说了什么才算。
- 是她投入了注意力的方向，不是这篇材料的话题。一篇文章提到游戏，不等于她
  对游戏感兴趣；她写下「抽卡明明知道是坑我还是想抽」才算。`

// InterestQuizSelectionRules retains the production text of quizSelectionRules.
const InterestQuizSelectionRules = `怎么算选中一个领域：
- 她**自己挑了这个作品、并写下了理由**。她挑的东西和她给的理由，本身就是根据 ——
  这里没有别人指定的材料，围栏里的每一个字都是她自己选择写下的。
- 看她的理由**落在哪一层**：同一个角色，有人写他的处境，有人写他的选择，有人写
  他的画风。她写的那一层就是她在关心的方向。
- 作品名本身不是领域（《进击的巨人》不等于「动画」）；把她的**理由**落到表里。`

// InterestHarvestSystemPromptTail retains the production text of harvestSystemPromptTail.
const InterestHarvestSystemPromptTail = `
字段要求：
- id：**上表里的 id 原样照抄**，不要改写，不要翻译，不要自己发明。
- note：一句话，对她说，讲这个领域在她身上是什么。不超过 40 字。
- evidence：**她自己写的原话**，原样摘录，不要改写、不要总结。
  找不到能作为根据的原话，就不要输出这一条。

宁可只给一个，也不要凑满三个。一个都选不出来就返回空数组。

只输出一个 JSON 对象，不要任何解释：
{"keywords":[{"id":"","note":"","evidence":""}]}`
