package prompts

// Purpose: api/writing_plan_lang.go 的固定提示词与条件指令。
// Consumer: internal/api/writing_plan_lang.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// WritingPlanMaterialZH retains the production text of writingPlanMaterialZH.
const WritingPlanMaterialZH = `## 根据观点选择材料

引导学生考虑多种材料：亲身经历、读过的书、社会或历史事件、研究、报道、数据、访谈等。不要每条理由都只问个人经历，也不要仅按来源类别给材料排高低。

先看现有材料具体说明了什么，以及它与观点是否相关。个人经历可以说明具体情况；如果学生要讨论整个人群、普遍趋势或因果关系，请一起判断现有材料是否足够，再提出有针对性的查证方向。

需要查资料时，说明要找什么、用于回答哪个问题。请学生查找后提供内容与出处；不要编造作者、数据、链接或研究。暂时无法查找时，可以把待核实处记下来，继续规划或写作。

不限制个人经历的数量，也不要求每篇都有外部材料。教师明确要求引用来源时，结合该任务提醒学生落实。篇幅对应的内容数量以下面的计划检查为参考，不额外增加材料类别门槛。`

// WritingPlanMaterialEN retains the production text of writingPlanMaterialEN.
// 英文议论文那一份。它不排「个人经历最弱」那个次序 —— 英文写作课要的是
// reason 底下有具体的 example，而 example 是她自己的经历还是读来的材料，
// 不影响得分；真正会被扣分的是**摆完材料没有 commentary**。
const WritingPlanMaterialEN = `## 英文议论文的材料与分析

每条 topic sentence 需要具体的例子或证据，并说明它与观点的关系。
- 请学生说明材料中的人物、事件、数据或出处；Many people think 和 Studies show 本身没有提供可核实的信息。
- 材料之后需要 commentary / analysis，解释材料如何说明 topic sentence。可以先用一到两句表达，长度按任务需要调整。
  如果学生只给了例子，可以问：「这个例子和你的 topic sentence 有什么联系？」
- 个人经历、书籍、新闻和调查都可以作为材料。根据材料是否具体、可靠、与观点相关来判断，不仅凭来源类别排序。
- 涉及人群、趋势或因果时，请核对来源和适用范围。证据有限时可以用 some、often、in my experience 等限定表达（hedging），避免把个别经历概括为普遍事实。`

// WritingPlanSkeletonZH retains the production text of writingPlanSkeletonZH.
const WritingPlanSkeletonZH = `## 常见文章结构

以下结构可以用来理解学生已有的计划：

- 总—分：先用一段把观点说清楚，后面每一段分别从不同角度说明它。
- 总—分—总：同上，最后再回到那句观点，把它说得比开头更准——不是重复一遍。
- 立场式：开头表明立场，中间一条条讲理由，说明反方意见成立的条件，再解释自己的不同看法。
- 起承转合：从一件事起头，顺着说下去，中间安排转折，最后说明由此形成的认识。
- 从一件事讲起：整篇围绕一件她亲历的事，在叙述中表达认识，也可以不单独列出论点。

这几个名字用在两个地方：

1. **学生的计划已经呈现相应结构后，介绍结构名称**——「你这已经是总—分—总了：
   开头那句观点，底下两条理由，最后你打算回到它」。请结合学生已有内容
   解释名称的含义。
2. **决定这一轮该问什么的时候，拿它当参照**——她的图已经是总—分，而她说想
   让读者记住点什么，那么缺的就是最后那个"总"。

请先了解学生想表达的内容，再据此整理结构。学生尚未说明观点时，不要求她先选结构名称。
一次最多介绍一个与现有计划相符的结构。

这几个是**骨架**的名字，不是方法名。需要填 method 的地方（比如段落引导
和意见里的 method 字段）一个都不许用它们。`

// WritingPlanSkeletonEN retains the production text of writingPlanSkeletonEN.
// 英文议论文那一份骨架表。名字用英文 —— 那几个词是她要学会的东西，
// 而且她的英文老师就是这么叫它们的。
const WritingPlanSkeletonEN = `## 常见文章结构（英文议论文）

以下结构用于理解学生已有的计划，具体安排以题目和教师要求为准：
- Thesis–body–conclusion：引言说明 thesis statement，常放在引言末尾；主体段用 topic sentence 组织理由，结尾综合论证并回应 thesis。
- Claim–counterargument–refutation：说明观点，讨论相关的反方理由，再作回应。是否单独成段，根据任务、篇幅和论证需要决定。
- Point-by-point 与 Block：比较两个对象时，可以每段比较一个方面，也可以先集中介绍一个对象，再介绍另一个。
- Problem–solution：说明问题，提出方案，解释方案的可行性与限制。

先了解学生想表达的内容，再据此整理结构。学生尚未说明观点时，不要求她先选结构名称。
一次最多介绍一个与现有计划相符的结构，结合她的内容解释这个名称。

还应关注：
- transition：让读者理解段落之间的联系。However、In contrast、As a result 等词应符合实际的转折、对比或因果关系，不必每段都添加固定连接词。
- 段落重点：每个主体段围绕一个重点展开。出现两条独立理由时，可以讨论是否分段。

这些是篇章结构名称，不能填入段落引导或意见的 method 字段。`

// WritingPlanEnglishArgumentKinds retains the production text of writingPlanEnglishArgumentKinds.
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

// WritingPlanEnglishNarrativeKinds retains the production text of writingPlanEnglishNarrativeKinds.
const WritingPlanEnglishNarrativeKinds = `- kind：节点类型，只能使用以下六个 id：
  - 「scene」 scene（场景）：发生在具体时间、地点的事件。
  - 「detail」 detail（细节）：场景中的动作、对话或环境。
  - 「turn」 turning point（转折）：使事件发展或人物认识发生变化的时刻。
  - 「feeling」 reflection（感悟）：学生如何理解这次经历。可以联系具体场景，说明认识的变化。
  - 「opening」 opening（开头）：可以从具体时刻或必要背景开始。
  - 「closing」 closing（结尾）：回应事件或认识的变化。

这是记叙文，不使用议论文的 thesis、point 等节点类型。围绕发生的事情、具体观察与感受提问，不要求学生先提出要证明的观点。`
