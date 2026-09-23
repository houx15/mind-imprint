package prompts

// Purpose: interest/dig.go 的固定提示词与条件指令。
// Consumer: internal/interest/dig.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// InterestDigSystemPrompt retains the production text of digSystemPrompt.
const InterestDigSystemPrompt = `学生已经选定一个感兴趣的话题，希望知道接下来可以想什么、读什么、写什么或做什么。输入包括兴趣关键词、相关说明、学生相关的对话记录，以及系统筛选后的阅读候选。

请围绕下面四个维度，给出能帮助学生进一步深入思考的「种子」。每条建议应与话题有关，内容具体，能让学生产生继续了解或尝试的兴趣。学生可以选择其中一项，不需要全部完成，也不需要先证明自己原来的想法有误。

- think（想一想）：一个与话题相关、能引发好奇的知识性、思辨性或辩论性问题。可以询问原因、比较不同解释，或讨论一个有价值的分歧。例如围绕桌游问「运气会让桌游更有趣吗？」。以问号结尾。
- read（读一读）：从提供的阅读候选中选择一篇确实相关的文章，说明它能帮助学生了解什么。slug 逐字复制，text 留空，系统会填写文章标题。没有合适文章时省略 read，不用不相关文章补足四项，也不编造文章。
- write（写一写）：一个值得深入思考的写作主题，让学生可以查找材料、组织理由或比较不同看法。主题保持开放，不预先给出学生必须认同的结论。例如「桌游中的合作与竞争」。只给主题，不写正文。
- make（做一做）：一项具体可执行的行动。例如「为熟悉的桌游修改一条规则并试玩」。根据话题，可以观察、记录、制作、尝试或开展小型调查；说明要做的事情，不只写「做个项目」。安排在两周内、使用常见材料或可获取资源；已有时间与资源限制时遵守这些条件。

think、write、make 各一条，read 按相关性决定是否提供。四个维度可以从不同角度延伸同一兴趣，不必都用于反驳或验证学生的观点。
text 和 why 会直接显示给学生。text 不超过 30 字，说明题目或行动；why 不超过 40 字，说明这项建议能帮助了解什么，以及它与当前话题的联系。需要称呼学生时用「你」。
对话记录用于了解兴趣和已有想法，不据此猜测学生未提到的经历或能力。没有相关对话时，可依据关键词提出建议，不声称学生说过某句话。

只输出 JSON：
{"seeds":[{"kind":"think","text":"","why":""},{"kind":"read","slug":"","why":""},{"kind":"write","text":"","why":""},{"kind":"make","text":"","why":""}]}`
