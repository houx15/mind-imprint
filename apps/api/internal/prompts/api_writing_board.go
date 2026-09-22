package prompts

// Purpose: api/writing_board.go 的固定提示词与条件指令。
// Consumer: internal/api/writing_board.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// WritingRoleBoardNote retains the production text of writingRoleBoardNote.
// writingRoleBoardNote 是标注板摆完的那一轮，加进 projection 的那段话。
//
// 三件事，顺序是有讲究的：先接住她摆的，再说那个缺口，最后给下一步。
// 「先接住」来自 master-writing 的四步反馈（先说哪个动作已经用对）；
// 「一次只说一件」来自那条贯穿整个房间的优先级。
const WritingRoleBoardNote = `
【她刚把一块标注板摆完】
她把自己这一段里的每一句，各自标成了「主张 / 证据 / 解释 / 让步 / 背景」之一。
上面那条消息就是她标的结果，最后一行写着这一段里**没有出现**哪几种。

这一轮：
1. **先接住她标的**，而且要具体到某一句为什么标在那儿。她标得跟你想的不一样，
   先认真看她的理由——一句话在不同读法下确实可以既是证据又是解释。
2. **先认清这一段在提纲里是哪一块、负责什么**（消息第一行写着这一段的标题，
   对照上面的提纲和片段）。空着的格子只有在**这一块本该有**的时候才算缺口：
   - 开头段、提出中心论点的那一段，任务是交代话题、亮出主张，**不需要证据**；
   - 结尾段的任务是收束、回扣主张，不需要新的证据；
   - 提纲里已经把某个例子安排在别的段落，就不要让她把它挪进这一段。
   这一块该有的都有了，就明说「这一段的任务完成了」，并指出下一块该写哪一段。
3. **只说那个缺口里最要紧的一处。** 一段主体论证里没有「证据」，比没有「背景」
   要紧得多；「有例子没有解释」又比两者都要紧。一次只说一处。
4. 以**一句祈使**收尾：她接下来往这一段里加的那一句话是什么（或者下一块去写什么）。
   不要替她写那句话，说清那句话要做成什么事就行。

不要紧接着再给她一件事做，也不要把她标的结果复述一遍——她刚摆完，她记得。
`
