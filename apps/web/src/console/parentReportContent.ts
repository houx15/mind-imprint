// Fixed 教育 front-matter for the parent report. Transcribed VERBATIM from the
// binding design's SHARED block:
// docs/design/teacher end/project/家长报告.dc.html (lines 442-499).
// These never change per student/session — do not paraphrase or shorten.

export const PRINCIPLES: { title: string; text: string }[] = [
  { title: "只看证据，不猜分数", text: "每一个判断都附一句孩子自己写的或说的原话；写不出证据，我们就不下判断，而是记「暂无可计入的证据」。" },
  { title: "两条轴分开看，不合成总分", text: "认知深度和智识自主是两件不同的事，我们不把它们加成一个分数，也从不预测考试或升学成绩。" },
  { title: "先看机会，再看表现", text: "如果平台还没给到某个锻炼机会，那记成我们的欠账，不算孩子的短板；只有「机会已经给了、孩子没接住」，才算孩子的信号。" },
  { title: "不贴标签、不排名", text: "报告只描述这一次作品里的行为，不评价孩子这个人，也不和其他同学做比较。" },
];

export const D_LEVELS: { label: string; color: string; desc: string }[] = [
  { label: "起步", color: "#C4574D", desc: "主要在复述资料，还没形成自己的问题和判断。" },
  { label: "发展", color: "#C68A3A", desc: "能提出问题、找到一些证据，但论证和方法还比较薄。" },
  { label: "熟练", color: "#3E7CA8", desc: "能把问题、方法、证据和结论连成一条线，会注意研究的局限。" },
  { label: "优秀", color: "#3E8A6E", desc: "论证扎实、证据充分，能主动说明「这份研究不能说明什么」。" },
];

export const D_DIMS: { name: string; meaning: string }[] = [
  { name: "D1 任务理解与问题表述", meaning: "能不能把一个宽泛的话题，收窄成真正能研究、能回答的问题。" },
  { name: "D2 证据与信源", meaning: "会不会找资料、判断资料可不可信、说清每份资料能支持什么。" },
  { name: "D3 论证结构", meaning: "能不能用证据一步步推出结论，而不是「结论大于证据」。" },
  { name: "D4 视角与偏见", meaning: "有没有考虑反方观点、承认研究的局限和偏差。" },
  { name: "D5 反馈处理与修订", meaning: "收到质疑后，是真的改进论证，还是只改了措辞。" },
  { name: "D6 反思与元认知", meaning: "能不能回头审视自己的思路，说清自己的判断是怎么来的。" },
];

export const A_STATES: { label: string; color: string; desc: string }[] = [
  { label: "观察到主动信号", color: "#3E8A6E", desc: "本周期多次、跨多个阶段地自己发起，是很好的自主表现。" },
  { label: "偶有·多在引导后", color: "#C68A3A", desc: "出现过，但常需要老师或工具卡提醒才发生。" },
  { label: "暂未观察到", color: "#C4574D", desc: "这一周期还没看到——可能是机会已提供但没取用，也可能是我们还没提供机会。" },
];

export const A_SIGNALS: { name: string; meaning: string }[] = [
  { name: "A1 方向自主", meaning: "是不是自己决定研究方向，而不是让老师或 AI 替他定。" },
  { name: "A2 发起自主", meaning: "会不会主动去查证、补证据，而不是只做被要求的事。" },
  { name: "A3 边界主权", meaning: "用 AI 时会不会设边界（「不要代写、不要编来源」）。" },
  { name: "A4 对抗与检验", meaning: "会不会主动请人挑刺、找自己论证里的问题。" },
  { name: "A5 判断署名", meaning: "愿不愿意为自己的结论负责，说清「这是我的判断」。" },
  { name: "A6 求真优先", meaning: "证据不支持时，愿不愿意把结论改小，而不是硬撑。" },
];

export const HOW_LIST: { title: string; text: string }[] = [
  { title: "D 轴按四个台阶判。", text: "我们逐条看孩子的表现落在哪个台阶，再看他「最稳定停留」在哪里——同一个台阶出现两次以上，才算稳定。" },
  { title: "A 轴只记录信号、不打分。", text: "我们记的是「主动信号」出现了几次、是自己发起还是被提醒后发生，用三种状态呈现，全程不出现任何数字或等级。" },
  { title: "反思维度只认孩子自己写的。", text: "「反思」这一项，只有孩子亲手写的反思才会被计入；如果没有，我们记「暂无可计入的证据」，而不是判低分。" },
  { title: "真实性只记录、不处罚。", text: "如果关键部分由 AI 代写，我们如实记录并建议当面核对，用作教学起点，不作任何处罚。" },
];

export const SUPPLY: string[] = [
  "在下一个任务里主动给出「请 AI 当反方」和「提交前先自评」的机会，把这次没提供到的机会补上。",
  "把「自己写一段反思」列入任务要求，让反思维度有据可依。",
];

export const GLOSSARY: { term: string; def: string }[] = [
  { term: "认知深度（D 轴）", def: "思考和论证走了多深，用起步 / 发展 / 熟练 / 优秀四个台阶来看。" },
  { term: "智识自主（A 轴）", def: "孩子有多愿意、多能够自己做研究判断，只记录信号、不打分。" },
  { term: "主动信号", def: "孩子自己发起的行为，比如主动质疑资料、主动请人挑刺、主动设边界。" },
  { term: "机会供给先于判定", def: "先看平台有没有给到锻炼机会。没给到，算平台欠账；给了没接住，才算孩子的信号。" },
  { term: "研究问题（RQ）", def: "一项研究想回答的核心问题，好的研究问题既不太大也不太空。" },
  { term: "研究空白（gap）", def: "已有研究还没回答清楚、值得孩子去补上的地方。" },
  { term: "求真优先", def: "当证据不支持时，愿意把结论改小、承认局限，而不是硬撑一个漂亮结论。" },
  { term: "AI 边界", def: "使用 AI 时明确「不要代写、不要编造来源、不要替我下结论」。" },
  { term: "自写反思", def: "孩子亲手写的、复盘自己思路的文字，是「反思」维度唯一的计入来源。" },
  { term: "训练用判定", def: "把这一次作品对照「低 / 中 / 高」的参考样例，只描述作品，不代表孩子的能力上限，也不预测考分。" },
];

export const FOOTER =
  "本报告由思维印记基于孩子在平台上的过程记录生成，供家校沟通参考，不作为学业评价或升学依据。报告不合成总分、不预测考试成绩、不对孩子作类型化标签或与其他同学横向比较。如有疑问，欢迎与任课老师联系。";

// Badge/state → color (client presentation; server sends only the label —
// never recompute a level from these maps' keys).
export const D_BADGE_COLOR: Record<string, { color: string; bg: string }> = {
  起步: { color: "#C4574D", bg: "#F7E6E4" },
  发展: { color: "#C68A3A", bg: "#F6EED9" },
  熟练: { color: "#3E7CA8", bg: "#E1EDF5" },
  优秀: { color: "#3E8A6E", bg: "#E4F0EA" },
  暂无: { color: "#8A92A3", bg: "#F1F2F6" },
};

export const A_STATE_COLOR: Record<string, { color: string; bg: string }> = {
  观察到主动信号: { color: "#3E8A6E", bg: "#E4F0EA" },
  "偶有·多在引导后": { color: "#C68A3A", bg: "#F6EED9" },
  暂未观察到: { color: "#C4574D", bg: "#F7E6E4" },
};
