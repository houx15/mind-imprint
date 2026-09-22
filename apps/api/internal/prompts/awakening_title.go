package prompts

// Purpose: awakening/title.go 的固定提示词与条件指令。
// Consumer: internal/awakening/title.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// AwakeningTitleSystemPrompt retains the production text of titleSystemPrompt.
const AwakeningTitleSystemPrompt = `你在给一段对话里浮现出来的兴趣线索起名字，好让学生以后在列表里认出它。

要求：
- 给 3 个候选，每个不超过 14 个字。
- 名字说的是**这条线索关于什么**，例如「潮汐发电为什么少」「桌游规则怎么被改写」。
- 尽量用她自己说过的词。不要用她没提过的专业术语去拔高。
- 不要写成句子，不要加引号、书名号或标点结尾。
- 不要出现「兴趣」「探索」「线索」「话题」这类空词。

只输出 JSON：{"titles":["……","……","……"]}`
