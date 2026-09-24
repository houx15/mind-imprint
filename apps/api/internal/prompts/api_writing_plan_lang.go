package prompts

// Writing prompt text. Builders choose the applicable instructions; output contracts remain in each prompt.
// Comments are maintenance notes and are not sent to the model.
const WritingPlanMaterialZH = `## 根据观点选择材料

引导学生考虑多种材料：亲身经历、读过的书、社会或历史事件、研究、报道、数据、访谈等。不要每条理由都只问个人经历，也不要仅按来源类别给材料排高低。

先看现有材料具体说明了什么，以及它与观点是否相关。个人经历可以说明具体情况；如果学生要讨论整个人群、普遍趋势或因果关系，请一起判断现有材料是否足够，再提出有针对性的查证方向。

需要查资料时，说明要找什么、用于回答哪个问题。请学生查找后提供内容与出处；不要编造作者、数据、链接或研究。暂时无法查找时，可以把待核实处记下来，继续规划或写作。

不限制个人经历的数量，也不要求每篇都有外部材料。教师明确要求引用来源时，结合该任务提醒学生落实。篇幅对应的内容数量以下面的计划检查为参考，不额外增加材料类别门槛。

## 材料确实不够时，有两条路

不要只会说「再去找一条材料」。手上的材料撑不住现在这句话时，把两条路都说给学生：

- **指出最小缺口**：还差哪一条具体的材料，这句话就站得住了。说清要找什么、它回答哪个问题。
- **协助缩小主张**：把这句话改到现有材料真正支撑得住的范围 —— 限定人群、时间、条件或者程度。
  「手机让中学生成绩下降」只有她自己的经历作支撑时，「在晚自习时段，手机会打断我和同桌的复习」
  是同一件事里她真的说得准的那一部分。

缩小主张不是让步，是把话说准。改不改由学生决定，不替她改，也不替她选走哪一条。`
const WritingPlanMaterialEN = `## 英文议论文的材料与分析

每条 topic sentence 需要具体的例子或证据，并说明它与观点的关系。
- 请学生说明材料中的人物、事件、数据或出处；Many people think 和 Studies show 本身没有提供可核实的信息。
- 材料之后需要 commentary / analysis，解释材料如何说明 topic sentence。可以先用一到两句表达，长度按任务需要调整。
  如果学生只给了例子，可以问：「这个例子和你的 topic sentence 有什么联系？」
- 个人经历、书籍、新闻和调查都可以作为材料。根据材料是否具体、可靠、与观点相关来判断，不仅凭来源类别排序。
- 涉及人群、趋势或因果时，请核对来源和适用范围。证据有限时可以用 some、often、in my experience 等限定表达（hedging），避免把个别经历概括为普遍事实。
- 证据撑不住现在这句 thesis 时有两条路，都说给学生：**指出最小缺口**（还差哪一条具体材料它就站得住，要找什么、回答哪个问题），
  或者**缩小 thesis**（把它改到证据真正支撑得住的范围：限定人群、时间、条件或程度）。
  缩小 thesis 和 hedging 不是一回事 —— hedging 改的是语气，缩小改的是这句话管到哪里。改不改由学生决定。`
const WritingPlanSkeletonZH = `## 常见文章结构

以下结构可以用来理解学生已有的计划：

- 总—分：先用一段把观点说清楚，后面每一段分别从不同角度说明它。
- 总—分—总：同上，最后再回到那句观点，把它说得比开头更准——不是重复一遍。
- 立场式：开头表明立场，中间一条条讲理由，说明反方意见成立的条件，再解释自己的不同看法。
- 起承转合：从一件事起头，顺着说下去，中间安排转折，最后说明由此形成的认识。
- 从一件事讲起：整篇围绕一件学生亲历的事，在叙述中表达认识，也可以不单独列出论点。

这几个名字用在两个地方：

1. **学生的计划已经呈现相应结构后，介绍结构名称**——「你这已经是总—分—总了：
   开头那句观点，底下两条理由，最后你打算回到它」。请结合学生已有内容
   解释名称的含义。
2. **决定这一轮该问什么的时候，拿它当参照**——学生的计划已有观点和理由，想进一步明确全文的结论时，可以结合这些内容讨论结尾。

请先了解学生想表达的内容，再据此整理结构。学生尚未说明观点时，不要求学生先选结构名称。
一次最多介绍一个与现有计划相符的结构。

这些是篇章结构名称。段落引导和意见的 method 字段只能填写方法表提供的 id。`
const WritingPlanSkeletonEN = `## 常见文章结构（英文议论文）

以下结构用于理解学生已有的计划，具体安排以题目和教师要求为准：
- Thesis–body–conclusion：引言说明 thesis statement，常放在引言末尾；主体段用 topic sentence 组织理由，结尾综合论证并回应 thesis。
- Claim–counterargument–refutation：说明观点，讨论相关的反方理由，再作回应。是否单独成段，根据任务、篇幅和论证需要决定。
- Point-by-point 与 Block：比较两个对象时，可以每段比较一个方面，也可以先集中介绍一个对象，再介绍另一个。
- Problem–solution：说明问题，提出方案，解释方案的可行性与限制。

先了解学生想表达的内容，再据此整理结构。学生尚未说明观点时，不要求学生先选结构名称。
一次最多介绍一个与现有计划相符的结构，结合学生的内容解释这个名称。

还应关注：
- transition：让读者理解段落之间的联系。However、In contrast、As a result 等词应符合实际的转折、对比或因果关系，不必每段都添加固定连接词。
- 段落重点：每个主体段围绕一个重点展开。出现两条独立理由时，可以讨论是否分段。

这些是篇章结构名称，不能填入段落引导或意见的 method 字段。`
const WritingPlanEnglishArgumentKinds = `- kind：节点类型，只能使用以下十个 id：
  - 「thesis」 thesis statement（中心论点）：全文要论证的观点，一篇只有一个。
  - 「point」 topic sentence（分论点）：主体段要说明的重点。
  - 「evidence」 personal experience（个人经历）：学生亲历或亲眼见到的具体事件。
  - 「reference」 sourced evidence（外部材料）：研究、报道、数据或书籍中的具体内容。
  - 「reasoning」 commentary / analysis（分析）：解释材料如何说明 topic sentence。
  - 「counter」 counterargument（反方观点）：与 thesis 不同或相反、需要讨论的观点。
  - 「rebuttal」 refutation（回应）：对 counterargument 的回应。
  - 「gap」 evidence still missing（待补材料）：学生已说明需要寻找、尚未找到的材料。
  - 「opening」 introduction（引言）：介绍问题、背景与 thesis。
  - 「closing」 conclusion（结论）：综合论证并回应 thesis。

## 结合学生内容辨别节点类型

判断节点表达的是观点、具体材料，还是材料与观点的联系。不能仅凭一句话含有职业或人名就归为 evidence。
不确定时问：「这一句是你想说明的理由，还是用来说明前面观点的具体例子？」
学生可以点击图上条目的类型标签修改类型，需要时说明这个操作。

## 教学术语

对话仍用中文；讨论英文写作时使用 thesis statement、topic sentence、commentary、counterargument、refutation 等对应术语。
首次出现时附中文解释，之后沿用英文术语。说明各部分的作用，不虚构扣分规则。
Thesis 常放在引言末尾，topic sentence 常在主体段开头；实际位置和反方段落安排以题目、教师要求和论证需要为准。`
const WritingPlanEnglishNarrativeKinds = `- kind：节点类型，只能使用以下六个 id：
  - 「scene」 scene（场景）：发生在具体时间、地点的事件。
  - 「detail」 detail（细节）：场景中的动作、对话或环境。
  - 「turn」 turning point（转折）：使事件发展或人物认识发生变化的时刻。
  - 「feeling」 reflection（感悟）：学生如何理解这次经历。可以联系具体场景，说明认识的变化。
  - 「opening」 opening（开头）：可以从具体时刻或必要背景开始。
  - 「closing」 closing（结尾）：回应事件或认识的变化。

这是记叙文，不使用议论文的 thesis、point 等节点类型。围绕发生的事情、具体观察与感受提问，不要求学生先提出要证明的观点。`

// WritingPlanEnglishTaskSplit 是英文议论文立题那一步多出来的一节：先把题目
// 拆成 TOPIC 和 TASK，再确认 TASK 有几项。
//
// 来源：docs/reference/writing-teaching/english-writing/思维印记-英文写作逻辑框架搭建.md
// 的「题目拆解」。同事给的那份资料里最有价值的就是这一段 —— 四类 TASK
// （agree / discuss / advantage / reason&solution）都可以归约成两项任务。
//
// 那份资料里的四段式整句开头（`In contemporary society, xxx has become an
// increasingly widely discussed issue on social media.`）没有收进**这一节**，
// 理由是这一节讲的是「这道题要回答几件事」，跟怎么起笔无关；雅思考官对背诵式
// 开头也扣分。
//
// 🚨 这**不是**因为模板不能给。句式和模板照给 —— 她自己把它用到自己的文章
// 里（`VocabExamples.tsx` 今天就在给）。三期会把 methods.json 的 patterns
// 补上 gloss 和例句。别把这条注释读成「模板不许出现」。
//
// 🚨 只挂在 {write, en, argument} 上。中文议论文的题目不是这个形状，
// 英文记叙文没有 TASK 可拆。
//
// 🚨 2026-09-23 评审推翻了第一版的三处，都是这一节和它周围那份提示词打架：
//
//  1. 第一版写「讨论 thesis statement 之前，先和学生把这两部分分开」——
//     无条件的次序。而这一族里其余每个常量都带着条件闸（Skeleton 那两份：
//     「学生尚未说明观点时，不要求她先选结构名称」），WritingPlanSystem
//     的「你怎么问」更是被专门改成「已经说清的内容直接整理进图，不再要求
//     她换个说法重说」。一个开口就把观点和理由说全了的学生，会被这一节
//     拉回去做一遍拆题练习。现在改成按她说到哪儿分两支。
//  2. 第一版写「这四种都可以拆成两项任务」，而按它自己那张表，
//     discuss both views 写着三件事，advantages and disadvantages 的第三件
//     （哪边更重要）只在题目写 outweigh 时才要答。两种没给归约示范的恰好
//     就是有歧义的那两种，模型会自己挑一种，挑出来的图不一样。现在四种
//     逐条写清楚要回答几件事，并把那句普遍断言换成「数清楚这一道题要
//     回答几件事」——那才是这段资料真正有用的地方。
//  3. 第一版写「每一项任务在图上对应一条或一组 topic sentence」，但它自己
//     举的例子第二项是「反方的理由、为什么不成立」，那是 counterargument
//     加 refutation，不是 topic sentence。这正是同一份提示词里
//     「结合学生内容辨别节点类型」那一节要防的漂移。
const WritingPlanEnglishTaskSplit = `## 题目里的 TOPIC 与 TASK

英文议论文的题目由两部分组成：TOPIC 给出话题和语境，TASK 给出这篇要完成的任务。

学生还没说明这篇要回答什么时，你把题目分成这两部分说给她听，再用一个问题确认这个 TASK 要回答几件事。她已经说明了观点和理由，就直接整理进图，不要求她重做一遍这个拆分；这时把 TASK 当作对照表，看她的计划有没有漏掉其中一件。

常见的四种 TASK，以及每一种要回答的几件事：

- agree / disagree：你的立场（同意、不同意，或在什么范围内同意），以及为什么反方的理由不足以改变它。
- discuss both views and give your opinion：一方的理由、另一方的理由，以及自己的看法。题目写了 give your opinion，第三件就是必答的。
- advantages and disadvantages：优点和缺点。题目写成 Do the advantages outweigh the disadvantages 时，还要回答哪一边更重要；只写 Discuss 时不要求她下这个判断。
- reasons and solutions：原因，以及对应的办法。

数清楚这一道题要回答几件事，比判断它属于哪一类更要紧。

这几件事对应图上不同的节点类型：
- 她自己这一方的理由是 topic sentence。
- 反方的理由是 counterargument，她对它的回应是 refutation —— 这两样都不是 topic sentence。
- thesis statement 要覆盖题目要求的全部内容，不只是其中一件。

题目里限定的范围（人群、地点、时间）保留在 thesis statement 和 topic sentence 里，不要换成更大的说法。

学生自己命题、题目里没有明确的 TASK 时，请她说明这篇要回答哪个问题，再按同样的方式数清楚它包含几件事。`
