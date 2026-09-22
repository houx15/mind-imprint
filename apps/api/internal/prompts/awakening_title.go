package prompts

// Purpose: awakening/title.go 的固定提示词与条件指令。
// Consumer: internal/awakening/title.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// AwakeningTitleSystemPrompt retains the production text of titleSystemPrompt.
const AwakeningTitleSystemPrompt = `学生正在进行兴趣测试。通过前面的对话，已经表达了一些感兴趣的事物或活动。请根据提供的对话记录，准确概括这次讨论的兴趣，给出三个名称供学生选择。

三个名称是同一兴趣的不同表达，供学生选择最准确的一项。用名词或动词短语，第一项直接写出兴趣对象，例如「潮汐发电」「桌游」；其余两项可以结合学生谈到的活动或具体内容，例如「了解潮汐发电原理」「利用潮水发电」，或「玩桌游」「修改桌游规则」。三个名称都应保持原话的范围：只谈过潮汐发电，就不概括为海洋能源或可再生能源；只谈过玩桌游，就不推断学生想设计桌游。

每个名称不超过 14 个字。使用学生能理解的词，不引入对话中没有依据的专业方向。不加引号、书名号或结尾标点，也不单独用「兴趣」「探索」「线索」「话题」作为名称。
只输出 JSON：{"titles":["……","……","……"]}`
