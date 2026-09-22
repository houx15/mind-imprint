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
- 她在这场谈话里**自己写下的文字**就是根据。这里没有别人指定的材料，
  每一句都是她选择说出来的，所以不需要再去分辨「这是材料的话题还是她的兴趣」。
- 看她的理由**落在哪一层**：同一件事，有人关心它怎么运作，有人关心它对谁有影响，
  有人关心它好不好看。她反复回到的那一层就是她在关心的方向。
- 作品名、游戏名、人名本身不是领域（《进击的巨人》不等于「动画」）。
  把她**说明原因的那些话**落到表里。
- 她后面几轮写的那个问题和那件作品设想，比第一轮的「我喜欢 X」更能说明方向。`

// AwakeningSelectionPromptTail retains the production text of selectionPromptTail.
const AwakeningSelectionPromptTail = `
字段要求：
- id：**上表里的 id 原样照抄**，不要改写，不要翻译，不要自己发明。
- note：一句话，**直接对她说，用「你」**，讲这个领域在她身上是什么。不超过 40 字。
  🚨 这一句会原样显示在报告里她的名字下面。写成「她从……」是在她面前谈论她，
  照抄上面那句判据的口吻就会写成这样 —— 写「你从……」。
- evidence：**她自己写的原话，原样摘录**，一个字都不要改，不要总结，不要拼接
  两句话。找不到能作为根据的原话，就不要输出这一条。

宁可只给一个，也不要凑满三个。一个都选不出来就返回空数组。

只输出一个 JSON 对象，不要任何解释：
{"keywords":[{"id":"","note":"","evidence":""}]}`

// AwakeningSelectionPromptHead retains the production text of selectionPromptHead.
const AwakeningSelectionPromptHead = `你在读一个中学生刚刚走完的一次兴趣探询。她被连续问了
八个问题：她最近主动靠近的是什么、其中哪个细节抓住了她、这个细节连着她的什么
经历、哪一点让她想不通、她想追问的问题、她还需要读什么、她目前的想法、她想做成
什么作品。

判断她正在关心哪几个领域，最多三个。

`
