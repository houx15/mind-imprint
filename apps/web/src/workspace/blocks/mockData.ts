// Self-contained mock data for the studio redesign prototype (design-only).
// No API, no backend — everything here is fixture content anchored to the
// canonical demo: Phoebe / "中国的发展让地球更可持续了吗？".

import type { CardTurnRef } from "@mind-imprint/contracts";

export type BlockKey = "forming" | "plan" | "reading" | "writing" | "reflection";

export type PlanTag = "read" | "write" | "review";
export type PlanColumn = "todo" | "doing" | "done";

export type PlanItem = {
  id: string;
  title: string;
  tag: PlanTag;
  column: PlanColumn;
  stage: string; // WBS phase grouping (阶段一 / 阶段二)
  refId?: string; // links a 读 item to a reference
  start: number; // day index on the project timeline (0-based)
  days: number; // duration in days — drives the Gantt bar width
};

export const STAGE_1 = "阶段一 · 研究与写作";
export const STAGE_2 = "阶段二 · 展示与答辩";
export const STAGES: string[] = [STAGE_1, STAGE_2];

export type ReadingNote = { quote: string; finding: string };

export type Collection = { id: string; name: string; parentId?: string };

// "Should I use this resource?" — the annotated-bibliography verdict column.
export type UseDecision = "use" | "maybe" | "drop" | null;

export type Reference = {
  id: string;
  title: string;
  kind: string; // 期刊 / 报告 / 新闻 / 数据集… (= Classification)
  author: string;
  credentials: string; // author credentials — annotated-bib column
  year: string;
  url: string;
  read: boolean;
  tags: string[];
  collectionId: string;
  credibility?: "strong" | "mixed" | "weak";
  takeaway: string; // 这篇能回答什么 / 不能回答什么 (= relevance + reliability)
  notes: ReadingNote[]; // reading-room outcomes (= cited parts / relevance)
  decision: UseDecision; // should I use this resource?
  pending?: boolean; // 还没找到这篇 —— 显示搜索建议而非档案
  searchHints?: string[];
};

export const DECISION_LABEL: Record<"use" | "maybe" | "drop", string> = {
  use: "该用",
  maybe: "待定",
  drop: "不用",
};

// Collections (folders) are the primary categorization — nestable for a
// multi-layer structure. Tags cross-cut them.
export const collections: Collection[] = [
  { id: "c-for", name: "正方证据" },
  { id: "c-for-eco", name: "生态与绿化", parentId: "c-for" },
  { id: "c-for-energy", name: "清洁能源", parentId: "c-for" },
  { id: "c-against", name: "反方证据" },
  { id: "c-bg", name: "背景与方法" },
];

export const CRED_LABEL: Record<NonNullable<Reference["credibility"]>, string> = {
  strong: "可信度高",
  mixed: "需交叉核实",
  weak: "存疑",
};

// `card`, when set, marks a card-turn: the bubble renders as a content-first
// clickable chip that opens a read-only view of the student's answers, instead
// of raw text. `text` stays the plain compiled fallback.
export type ChatMsg = { role: "ai" | "student"; text: string; card?: CardTurnRef | null };

// The four proposal dimensions (EPQ 开题报告 §1–§4). The forming chat coaches
// across all four; a formal proposal doc is an OPTIONAL export — never forced.
// These dimensions get valued in assessment whether or not a doc is produced.
export type Proposal = {
  objective: string; // §1 目标：想回答的问题 / 想学会的
  reason: string; // §2 缘由：为什么做这个项目
  activities: string; // §3 活动与时间：打算怎么做（→ 计划）
  resources: string; // §4 资源：需要哪些书/期刊/工具（→ 阅读清单）
  counterpoints: string; // §5（可选）可能的反例/张力：论点可能撞上的反例（→ 写作/让步段）
};

export type ProtoProject = {
  title: string;
  qualLabel: string;
  proposal: Proposal;
};

export const emptyProposal: Proposal = { objective: "", reason: "", activities: "", resources: "", counterpoints: "" };

export const project: ProtoProject = {
  title: "中国的发展让地球更可持续了吗？",
  qualLabel: "TOK · 拓展论文风格",
  proposal: {
    objective:
      "以中国近二十年的发展为例，回答：经济增长与环境可持续之间的张力，该用什么尺度来判断？我也想练会在对立证据之间做判断。",
    reason:
      "新闻里中国既大规模植树造林、光伏领先，又是全球碳排放第一。这个矛盾让我想弄清楚——发展到底让地球更可持续，还是更不可持续。",
    activities: "溯源关键数据 → 读正反两方文献 → 搭论证、撞反例 → 处理让步段 → 成稿与反思。",
    resources: "NASA / Nature 卫星数据、IEA 能源报告、Our World in Data 排放数据；学校数据库与 Google Scholar。",
    counterpoints: "中国碳排放全球第一——这条最硬的反例，我的论点必须正面回应（让步段）。",
  },
};

// The four REQUIRED dims plus the optional 5th (counterpoints). `required`
// distinguishes the gate: only the four required dims must be filled before
// 生成项目计划; 反例/张力 is invited, never required.
export const PROPOSAL_DIMS: { key: keyof Proposal; label: string; hint: string; required: boolean }[] = [
  { key: "objective", label: "目标", hint: "想回答什么问题？想学会做什么？", required: true },
  { key: "reason", label: "缘由", hint: "为什么是这个题？关联的学科、兴趣、未来", required: true },
  { key: "activities", label: "活动与时间", hint: "打算怎么做——之后会长成你的计划", required: true },
  { key: "resources", label: "资源", hint: "需要哪些书 / 期刊 / 数据 / 工具", required: true },
  { key: "counterpoints", label: "可能的反例 / 张力", hint: "你的论点最可能撞上什么反例？（可选，写作时会用到）", required: false },
];

// The forming-phase conversation, pre-seeded mid-flow so the page reads real.
export const formingChat: ChatMsg[] = [
  {
    role: "ai",
    text: "先不急着列提纲。你想做的这个题目，最初是什么让你想问它的？说说你脑子里的那个画面就行。",
  },
  {
    role: "student",
    text: "我看到中国又植树造林又搞光伏，好像很环保，但又听说它碳排放世界第一。到底算不算让地球更可持续？",
  },
  {
    role: "ai",
    text: "这个「既…又…」的矛盾，正是一篇好文章的种子。那你现在心里，更偏向哪一边？还是真的没定？",
  },
  {
    role: "student",
    text: "没定。我觉得可能要看用什么标准衡量吧。",
  },
  {
    role: "ai",
    text: "很好——「取决于尺度」本身就可以是你的论点雏形。我先把你说的整理进右边的卡片，你随时改。觉得差不多了，就生成一份计划。",
  },
];

export const planItems: PlanItem[] = [
  { id: "p1", title: "读：NASA 卫星植被覆盖数据", tag: "read", column: "done", stage: STAGE_1, refId: "r1", start: 0, days: 2 },
  { id: "p2", title: "读：Nature Sustainability 综述", tag: "read", column: "doing", stage: STAGE_1, refId: "r2", start: 1, days: 3 },
  { id: "p3", title: "读：找一篇质疑视角（中国碳排放总量）", tag: "read", column: "todo", stage: STAGE_1, refId: "r3", start: 3, days: 2 },
  { id: "p4", title: "写：确定论点与整体结构", tag: "write", column: "todo", stage: STAGE_1, start: 4, days: 2 },
  { id: "p5", title: "写：起草「正方」证据段", tag: "write", column: "todo", stage: STAGE_1, start: 6, days: 3 },
  { id: "p6", title: "写：处理反例（碳排放全球第一）", tag: "write", column: "todo", stage: STAGE_1, start: 9, days: 3 },
  { id: "p7", title: "省：论点是否真的回答了题目？", tag: "review", column: "todo", stage: STAGE_1, start: 12, days: 2 },
  { id: "p8", title: "写：整理注释书目（Annotated Bibliography）", tag: "write", column: "todo", stage: STAGE_2, start: 14, days: 2 },
  { id: "p9", title: "省：模拟答辩与复盘", tag: "review", column: "todo", stage: STAGE_2, start: 16, days: 2 },
];

// Total span of the timeline (days), used to lay out the Gantt grid.
export const TIMELINE_DAYS = 18;

// The brand-new-project state: no thesis yet, no chat but the opening line, an
// empty statement the conversation will fill in.
export const freshChat: ChatMsg[] = formingChat.slice(0, 1);

export const references: Reference[] = [
  {
    id: "r1",
    decision: "maybe",
    title: "《卫星图看中国变绿》",
    kind: "公众号文章",
    author: "某科普公众号",
    credentials: "匿名科普账号，无署名作者、无机构背书",
    year: "2021",
    url: "https://mp.weixin.qq.com/s/china-greening-satellite",
    read: true,
    tags: ["正方", "入口"],
    collectionId: "c-for-eco",
    credibility: "mixed",
    takeaway:
      "能回答：给了「中国让地球变绿」这个通俗印象的入口。不能回答：把「变绿」直接等于「环保见效」是作者放大的结论，数据要回溯到一手源。",
    notes: [
      { quote: "根据 NASA 卫星数据……", finding: "「NASA 数据」是转述——溯源到 Chen et al. (2019) 才是一手。" },
      { quote: "很难不把这读成一个信号：环保政策正在起效。", finding: "作者把「变绿」滑向「更可持续」，绕开了碳排放这个反例。" },
    ],
  },
  {
    id: "r2",
    decision: "use",
    title: "Chen et al. (2019), Nature Sustainability",
    kind: "期刊论文",
    author: "Chen, C. et al.",
    credentials: "同行评议期刊 Nature Sustainability；作者为可持续与遥感研究者",
    year: "2019",
    url: "https://doi.org/10.1038/s41893-019-0220-7",
    read: true,
    tags: ["正方", "一手源"],
    collectionId: "c-for-eco",
    credibility: "strong",
    takeaway:
      "能回答：卫星确证地球在变绿、中国是最大贡献者之一，机制是农业集约化与植树造林。不能回答：论文未涉及碳排放，不支撑「中国让地球更可持续」这个更大的结论。",
    notes: [{ quote: "增量主要来自农业集约化与大规模植树", finding: "变绿≠生态系统整体改善——这是我论证里要小心的跳步。" }],
  },
  {
    id: "r4",
    decision: "use",
    title: "An Energy Sector Roadmap to Carbon Neutrality in China",
    kind: "机构报告 · IEA",
    author: "International Energy Agency",
    credentials: "国际能源署（IEA），政府间权威机构",
    year: "2021",
    url: "https://www.iea.org/reports/an-energy-sector-roadmap-to-carbon-neutrality-in-china",
    read: false,
    tags: ["正方", "清洁能源"],
    collectionId: "c-for-energy",
    credibility: "strong",
    takeaway: "能回答：中国光伏/风电装机与投资的领先地位。不能回答：装机领先≠总排放下降，仍需与存量排放对照看。",
    notes: [],
  },
  {
    id: "r5",
    decision: "use",
    title: "CO₂ emissions — China (country profile)",
    kind: "数据集 · Our World in Data",
    author: "Ritchie & Roser",
    credentials: "牛津大学 Our World in Data，数据透明、可溯源",
    year: "2023",
    url: "https://ourworldindata.org/co2/country/china",
    read: false,
    tags: ["反方", "一手源"],
    collectionId: "c-against",
    credibility: "strong",
    takeaway: "能回答：中国碳排放总量全球第一、人均已超部分发达国家。不能回答：不含治理趋势与承诺，需与正方证据并置。",
    notes: [],
  },
  {
    id: "r6",
    decision: "maybe",
    title: "中国碳达峰、碳中和「双碳」目标解读",
    kind: "政策评论",
    author: "某智库",
    credentials: "立场性智库，观点需交叉核实",
    year: "2022",
    url: "https://example.org/dual-carbon",
    read: false,
    tags: ["背景"],
    collectionId: "c-bg",
    credibility: "mixed",
    takeaway: "能回答：官方承诺的时间线与口径。不能回答：承诺≠已实现，属立场性材料，需交叉核实。",
    notes: [],
  },
  {
    id: "r7",
    decision: "use",
    title: "IPCC AR6 WG3 · Ch.2 Emissions Trends & Drivers",
    kind: "报告 · IPCC",
    author: "IPCC",
    credentials: "联合国政府间气候变化专门委员会（IPCC）",
    year: "2022",
    url: "https://www.ipcc.ch/report/ar6/wg3/",
    read: false,
    tags: ["背景", "一手源"],
    collectionId: "c-bg",
    credibility: "strong",
    takeaway: "能回答：全球排放趋势与归因的权威框架，给我的尺度之争一个共同基准。",
    notes: [],
  },
  {
    id: "r3",
    decision: null,
    title: "China remains the world's largest CO₂ emitter",
    kind: "待补充",
    author: "—",
    credentials: "",
    year: "—",
    url: "",
    read: false,
    tags: ["反方"],
    collectionId: "c-against",
    credibility: undefined,
    takeaway: "",
    notes: [],
    pending: true,
    searchHints: [
      "Google Scholar 搜「China CO2 emissions global share」",
      "找 IEA 或 Our World in Data 的国别排放数据（一手、可交叉核实）",
      "注意区分「总量第一」与「人均」——两个尺度会导向不同结论",
    ],
  },
];

// The Library's AI assistant — coaches the hunt (what's missing, where to look,
// what's reliable). It never fetches sources; that stays the student's move.
export const readingCoachChat: ChatMsg[] = [
  {
    role: "ai",
    text: "你现在有两篇偏「正方」的来源（变绿、造林），但还没有任何质疑的声音。一篇好文章需要一个能打的反例。",
  },
  { role: "student", text: "从哪找质疑的？" },
  {
    role: "ai",
    text: "别搜「中国环保成绩」——那只会强化你已有的。试着搜「China largest CO₂ emitter」，优先找 IEA、Our World in Data 这类能交叉核实的一手数据。找到了拖进来，我们一起在阅读室里判断它可不可信。",
  },
];

// The activity log — a real EPQ deliverable. Some entries are auto-seeded from
// platform activity ("auto"), some are the student's own notes ("me"). Lives in
// Project Management and exports from there.
export type LogEntry = { id: string; date: string; text: string; source: "auto" | "me" };

export const activityLog: LogEntry[] = [
  { id: "l1", date: "07-02", text: "确定题目方向：中国是否让地球更可持续；和印记聊清了目标与缘由。", source: "me" },
  { id: "l2", date: "07-03", text: "在阅读室打开《卫星图看中国变绿》，溯源发现「NASA 数据」是转述。", source: "auto" },
  { id: "l3", date: "07-05", text: "读 Chen et al. (2019)，记下 1 条笔记：变绿≠生态整体改善。", source: "auto" },
  { id: "l4", date: "07-08", text: "在写作区起草引言与「正方」证据段（首稿 320 字）。", source: "auto" },
  { id: "l5", date: "07-09", text: "撞上反例：中国碳排放全球第一。意识到论证有跳步。", source: "me" },
  { id: "l6", date: "07-09", text: "计划里新增任务「找一篇质疑视角」，移入「待办」。", source: "auto" },
];

// Bilingual: 印记 can guide across the four proposal parts in Chinese OR English.
export const formingChatEN: ChatMsg[] = [
  { role: "ai", text: "No need to outline yet. What first made you want to ask this question? Just describe the picture in your head." },
  { role: "student", text: "China plants huge forests and leads in solar, yet it's the world's largest CO₂ emitter. Is it really making the planet more sustainable?" },
  { role: "ai", text: "That \"both… and…\" tension is the seed of a good essay. Which way do you lean right now — or genuinely undecided?" },
  { role: "student", text: "Undecided. I think it depends on the yardstick you use." },
  { role: "ai", text: "\"It depends on the scale\" is already a thesis in embryo. Let's make sure we've thought through four things — your objective, your reasons, your activities, and the resources you'll need. Shall we start with the objective?" },
];

// The Write block: an outline (nested, flat-with-depth for easy editing) and a
// single draft panel. Student writes; AI talks.
export type OutlineNode = { id: string; text: string; depth: number };

export const outline: OutlineNode[] = [
  { id: "o1", text: "引言：中国是否让地球更可持续——取决于用什么尺度", depth: 0 },
  { id: "o1a", text: "背景：卫星显示地球在变绿，中国贡献最大", depth: 1 },
  { id: "o1b", text: "论点：局部生态与清洁能源确有正向改变，但以总量计的碳排放使「更可持续」难以成立", depth: 1 },
  { id: "o2", text: "正方：中国带来的正向改变", depth: 0 },
  { id: "o2a", text: "生态：植树造林与农业集约化（Chen et al. 2019）", depth: 1 },
  { id: "o2b", text: "清洁能源：光伏 / 风电装机领先（IEA）", depth: 1 },
  { id: "o3", text: "反方与让步：绕不开的碳排放", depth: 0 },
  { id: "o3a", text: "中国碳排放总量全球第一（Our World in Data）", depth: 1 },
  { id: "o3b", text: "让步段：趋势变好 ≠ 问题已解决", depth: 1 },
  { id: "o4", text: "结论：判断取决于尺度——总量 / 人均 / 趋势", depth: 0 },
];

export const draftMarkdown = `## 引言

这篇文章想讨论一个常被简化的说法：中国是否让地球更可持续。

卫星数据显示，2000 年以来地球明显变绿，其中中国的贡献最大。照这个趋势，似乎可以说中国正在让整个地球更可持续。

（未完）当然，有人会说中国的碳排放总量是全球第一……
`;

export const writingCoachChat: ChatMsg[] = [
  { role: "ai", text: "你的提纲里，「正方」和「反方」都立住了。但引言最后那句「似乎可以说中国正在让地球更可持续」——这就是你要论证的，还是你要反驳的？" },
  { role: "student", text: "是我要复杂化的。我不想直接下这个结论。" },
  { role: "ai", text: "那就别让引言替你把话说满。要不要在这里先埋一个「取决于尺度」的伏笔，把判断留到结论？" },
];

// Reflection prompts follow the real EPQ/TOK reflection arc: goal → did I meet
// it → data/methods → problems (& nearly giving up) → limitations → what I
// gained + future. The 目标 prompt anchors back to what the student wrote at
// 开题, closing the loop.
export const reflectionPrompts: { label: string; q: string; anchor?: "goal" }[] = [
  { label: "目标与达成", q: "你当初的目标是什么？现在回头看，达成了多少？", anchor: "goal" },
  { label: "方法与数据", q: "你用了哪些资料、数据或方法？它们怎么支撑（或没能支撑）你的判断？" },
  { label: "遇到的问题", q: "过程中最卡的地方是什么？你是怎么熬过来的——有没有想过放弃？" },
  { label: "局限", q: "这份研究还有哪些不足或局限？如果有人质疑，最先会打到哪里？" },
  { label: "收获与未来", q: "你获得了什么（知识、技能、态度）？接下来你会怎么继续或改进？" },
];

// The "你的思维印记" mirror — AI-assembled from the whole process, shown ONLY
// as support for the student's own reflection (not a grade, not a verdict).
export const mirrorSections: { title: string; body: string }[] = [
  { title: "你的论点是怎么长出来的", body: "你从「中国让地球更可持续」这个通俗判断出发，一路把它复杂化成「取决于用什么尺度」。真正的转折点，是你在写作里撞上「碳排放总量第一」这个反例。" },
  { title: "阅读怎样喂养了写作", body: "读 NASA / Chen (2019) 时你做了溯源，把「NASA 数据」从转述追到一手；这条判断后来直接进了你的正方段，也让你对「变绿≠更可持续」一直保持警惕。" },
  { title: "哪里你自己想通，哪里靠印记", body: "溯源和让步段是你自己发现的；引言那句「似乎可以说更可持续」，是印记提醒你别把话说满、把判断留到结论。" },
  { title: "你召唤过的思维卡", body: "这一程你召唤了 SIFT 溯源、让步段两张卡；CRAAP 五维那次你跳过了——跳过也被记下了。" },
];

export const carryForwards: string[] = [
  "你倾向到很后面才处理反例——下次试试在提纲阶段就先把反方埋进去。",
  "遇到通俗结论，你已经会追问「用什么尺度」——把这个习惯带到下一个题目。",
];

export const TAG_LABEL: Record<PlanTag, string> = {
  read: "读",
  write: "写",
  review: "省",
};

export const COLUMN_LABEL: Record<PlanColumn, string> = {
  todo: "待办",
  doing: "进行中",
  done: "完成",
};
