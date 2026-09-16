// Rubric dimensions and report sections from the existing marketing site.
export const depth: [string, string, string, string, string][] = [
  ["D1", "任务理解与问题表述", "Framing the question", "能否把研究问题说清、限定范围、拆出可答的子问题", "Stating it clearly, bounding scope, splitting out answerable sub-questions"],
  ["D2", "证据与信源", "Evidence & sources", "追溯一手来源、区分转述与原文、核查数据", "Tracing to primary sources, separating paraphrase from original, checking numbers"],
  ["D3", "论证结构", "Argument structure", "主张—证据—推理的完整性、识别缺失的 warrant、构造让步段", "Claim–evidence–warrant integrity, spotting the missing warrant, building the concession"],
  ["D4", "视角与偏见", "Perspective & bias", "多视角比较、识别来源的立场与利益、处理反方证据", "Comparing perspectives, reading a source's stake, handling counter-evidence"],
  ["D5", "反馈处理与修订", "Feedback & revision", "理解批评、修订有理由、改的是论证而非只是措辞", "Understanding critique, revising with reasons, changing the argument not just the wording"],
  ["D6", "反思与元认知", "Reflection & metacognition", "从复盘事实，到审视自己的方法与框架", "From recounting what happened to interrogating her own method and frame"],
];

export const autonomy: [string, string, string, string, string][] = [
  ["A1", "方向自主", "Direction", "谁在定目标与路线：自拟或修改计划、主动往深处钻", "Who sets the target and route — writing or rewriting the plan, digging deeper unprompted"],
  ["A2", "发起自主", "Initiation", "无人要求时的提问、核查、修订", "Questions, checks and revisions nobody asked for"],
  ["A3", "边界主权", "Boundary", "给 AI 设界、拒绝建议并给出理由、守住自己的路径", "Setting limits on the AI, refusing a suggestion with a reason, holding her own route"],
  ["A4", "对抗与检验", "Challenge", "主动召唤反方、带证据质疑 AI、抵抗迎合陷阱", "Summoning the counter-case, challenging the AI with evidence, resisting its urge to agree"],
  ["A5", "判断署名", "Signature", "评估判断自己下、反思自写、为结论给理由", "Making the call herself, writing her own reflection, giving reasons for the conclusion"],
  ["A6", "求真优先", "Truth first", "愿意跟着证据走到对自己不利的地方：接受反证、修正立场", "Willing to follow the evidence somewhere inconvenient — accepting it, moving her position"],
];

/* ── the report's nine parts ────────────────────────────────────────── */
export const reportParts: [string, string, string, string][] = [
  ["基础数据", "Basics", "标题、起止、里程碑，以及数出来的计数：AI 轮次、材料数、正文字数、批注、修订。", "Title, dates, milestones, and counted numbers: AI turns, materials, words, comments, revisions."],
  ["摘要", "Abstract", "一段回顾她这次真正走过的路，加上三句分别说材料、写作与 AI 使用。", "A paragraph on the route she actually took, plus one line each on materials, writing and AI use."],
  ["事件时间线", "Timeline", "对话、阅读室、图谱、写作、复盘，逐条按时间摊开。", "Chat, reading room, map, writing, retrospective — laid out in order."],
  ["材料清单", "Materials", "每条来源的加入时间、出处、链接、它在论证里承担了什么。", "Every source: when it arrived, where from, and what job it does in the argument."],
  ["认知深度 D1–D6", "Depth D1–D6", "每一维给分层、一句判断、若干条可点开的证据、一句建议。", "Per dimension: a level, a verdict, clickable evidence, one suggestion."],
  ["智识自主 A1–A6", "Autonomy A1–A6", "同样结构；并标记这一次是自发还是被引导。", "Same structure — plus whether each move was self-initiated or prompted."],
  ["提示词观测", "Prompt lens", "挑出 3–10 条她真实写过的提示词，说明好在哪、可以怎么改。", "3–10 of her actual prompts, with what worked and what to change."],
  ["工具卡与子代理", "Cards & sub-agents", "哪张卡、在哪一站、为什么被递出来、产出改变了后续什么。", "Which card, at which station, why it was offered, and what changed downstream."],
  ["风险行为", "Risk signals", "AI 代劳、缺少信源、论证断裂、数据口径、兔子洞跑题——各自附建议。", "Ghost-writing, missing sources, broken warrants, shifting denominators, tangents — each with advice."],
];