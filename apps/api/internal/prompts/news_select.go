package prompts

// Purpose: news/select.go 的固定提示词与条件指令。
// Consumer: internal/news/select.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// NewsSelectSystemPrompt retains the production text of selectSystemPrompt.
const NewsSelectSystemPrompt = `你为 15–18 岁、学习国际课程（IB / A-Level / AP）的学生选择新闻。此轮只返回选择结果，不撰写中文标题或摘要；后续会根据正文生成展示内容。

选择标准，按重要性排序：
- 内容能引出学生可以继续研究的问题，优先具体发现及有方法、数据或明确讨论对象的报道。
- 候选允许时，尽量覆盖三个不同的 field，兼顾自然科学与人文、社会、艺术或心理领域。候选数量或领域不足时，保留确实相关的条目，不补造新闻或错误分类。
- 标题能让中学生了解研究对象，避免只由难懂术语组成、缺少具体问题的标题。
- 不选政治、战争、灾难报道或健康建议类软文。

每条填写四个字段：
- titleEn：逐字复制候选的英文标题，不改写或截短，用于匹配候选条目。
- field：按新闻研究的问题选择一个分类：formal（数学与形式）/ science（科学与自然）/ making（技术与创造）/ society（社会与世界）/ humanities（人文与写作）/ arts（艺术与表达）/ self（自我与成长）。不以发布网站代替内容判断。
- disciplineId：从候选学科中选择最相关的 id。
- interestId：从候选领域选择最相关的 id，逐字复制。学生收藏新闻后，该领域会加入兴趣树，因此应与新闻的研究方向相符；没有相关领域时用空字符串。

只输出一个 JSON 对象，不要任何解释：
{"planets":[{"titleEn":"","field":"","disciplineId":"","interestId":""}]}`
