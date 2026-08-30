import type { HomepageSection, StyleId, WorkMode } from "./types";

/**
 * PBL#0 · 建一个属于你的主页 — the config behind the first project.
 *
 * ## The teaching claim
 * 「AI 在明确的指令下才好用。」明确 = 三样东西：一个直接的例子、内容的结构、
 * 以及**你想怎么和它合作**。选完三样，`compileInstruction` 把它们编译成一段
 * 真的指令给她看 —— 这是整个模块的核心时刻：指令不再是玄学，是三个可选项拼出来的。
 *
 * ## 🚨 三种合作模式都不代写正文
 * 铁律①：AI 绝不代写学生的正文。这一条在这里最容易被违反 —— 「让 AI 帮我写自我
 * 介绍」正是最诱人的捷径。所以三种模式各自绕开它：
 *  - `ask` 提问式：AI 只提问，一次一个；句子全部是她写的。
 *  - `propose` 提案式：AI 提的是**角度和骨架**（带空格），不是成品句子；空格由她填。
 *  - `tidy` 整理式：她先写，AI 只标出可以删 / 可以合并 / 缺例子的地方，逐条由她决定。
 * 任何一处出现「AI 直接生成一段可以照抄的自我介绍」，这个模块就自毁了。
 */

export const WORK_MODES: {
  id: WorkMode;
  label: string;
  short: string;
  blurb: string;
  /** How the instruction says it, in the compiled prompt. */
  instruction: string;
  /** What it feels like, one line, so she can choose knowingly. */
  feels: string;
  glyph: string;
}[] = [
  {
    id: "ask",
    label: "AI 提问，我来答",
    short: "提问式",
    blurb: "它一次问你一个问题，你答。所有句子都是你写的。",
    instruction: "不要替我写。一次只问我一个问题，等我答完再问下一个。问完之后把我说过的话原样还给我，让我自己组织。",
    feels: "最慢，但写出来最像你。适合你还不知道自己想说什么的时候。",
    glyph: "?",
  },
  {
    id: "propose",
    label: "AI 提方案，我来定",
    short: "提案式",
    blurb: "它给你两三个**角度和骨架**（留着空格），你挑一个，空格自己填。",
    instruction: "给我 2–3 个不同的角度，每个角度给一个留空的骨架（例如「我一直在想 ___，因为 ___」）。不要把空格填上——那是我的部分。",
    feels: "最快找到方向。适合你有话说，但不知道从哪句开始。",
    glyph: "⋮",
  },
  {
    id: "tidy",
    label: "我来写，AI 只整理",
    short: "整理式",
    blurb: "你随便写，它只标出「这里重复了」「这两句可以合并」「这里缺一个具体例子」，你逐条决定。",
    instruction: "我写完之后，不要改我的句子。只标出：哪里重复、哪两句可以合并、哪里缺一个具体的例子。每条给我一个理由，改不改我自己决定。",
    feels: "最保留你的语气。适合你已经写得出来，只是有点乱。",
    glyph: "✎",
  },
];

export function workModeById(id: WorkMode) {
  return WORK_MODES.find((m) => m.id === id) ?? WORK_MODES[0]!;
}

export const PAGE_STYLES: {
  id: StyleId;
  label: string;
  en: string;
  blurb: string;
  paper: string;
  ink: string;
  accent: string;
  /** CSS font stack for the published page. */
  font: string;
  /** Heading treatment on the published page. */
  headline: "serif-xl" | "mono-caps" | "sans-tight" | "serif-italic";
}[] = [
  {
    id: "morning",
    label: "清晨纸感",
    en: "Morning Paper",
    blurb: "暖白纸、衬线标题、一条细线。像一封写好的信。",
    paper: "#FBF8F4",
    ink: "#33302E",
    accent: "#EA5140",
    font: '"Noto Serif SC","Songti SC",Georgia,serif',
    headline: "serif-xl",
  },
  {
    id: "terminal",
    label: "深夜终端",
    en: "Night Terminal",
    blurb: "近黑底、等宽字、绿色光标。安静、精确、有点酷。",
    paper: "#14120F",
    ink: "#E8E2D8",
    accent: "#6FBFB0",
    font: 'ui-monospace,"SF Mono","PingFang SC",monospace',
    headline: "mono-caps",
  },
  {
    id: "magazine",
    label: "杂志切页",
    en: "Magazine",
    blurb: "大标题、粗色块、强对比。适合作品多、想被一眼看到的人。",
    paper: "#FFFFFF",
    ink: "#1A1A1A",
    accent: "#E0A63A",
    font: '-apple-system,"PingFang SC","Helvetica Neue",sans-serif',
    headline: "sans-tight",
  },
  {
    id: "garden",
    label: "植物园",
    en: "Garden",
    blurb: "淡绿底、圆角、手写感的小标注。适合还在生长中的东西。",
    paper: "#F4F7F0",
    ink: "#2F3B2C",
    accent: "#5FA97E",
    font: '"Noto Serif SC","Songti SC",Georgia,serif',
    headline: "serif-italic",
  },
];

export function styleById(id: StyleId) {
  return PAGE_STYLES.find((s) => s.id === id) ?? PAGE_STYLES[0]!;
}

/** The section menu. `enabled` is her structural choice in step 2 — the
 *  ORDER and the SELECTION are the "structure of content" half of a clear
 *  instruction, so they are chosen before any writing happens. */
export const DEFAULT_SECTIONS: HomepageSection[] = [
  {
    id: "intro",
    label: "一句话介绍",
    hint: "你是谁 + 你着迷于什么。不是头衔。",
    value: "",
    enabled: true,
  },
  {
    id: "question",
    label: "我在乎的问题",
    hint: "一个你反复回来的问题。写下来它就有了地址。",
    value: "",
    enabled: true,
  },
  {
    id: "projects",
    label: "我做过的",
    hint: "从你的项目里挑。做完的和在做的都可以，标清楚就行。",
    value: "",
    picker: "projects",
    picked: [],
    enabled: true,
  },
  {
    id: "writings",
    label: "我写的",
    hint: "挑你自己最喜欢的，不一定是分最高的那篇。",
    value: "",
    picker: "writings",
    picked: [],
    enabled: true,
  },
  {
    id: "readings",
    label: "我读过的",
    hint: "读过什么，比说自己喜欢什么更可信。",
    value: "",
    picker: "readings",
    picked: [],
    enabled: true,
  },
  {
    id: "detail",
    label: "一个关于我的怪细节",
    hint: "让页面变成「你的页面」的那一件小事。",
    value: "",
    enabled: true,
  },
  {
    id: "contact",
    label: "怎么找到我",
    hint: "一个真的能收到消息的地方就够了。",
    value: "",
    enabled: false,
  },
];

/**
 * The compiled instruction — shown to her verbatim as 「这就是你给 AI 的指令」.
 *
 * This is the module's payoff, so it is built to be READ, not to be clever:
 * four labelled parts, her actual choices interpolated, in the second person
 * she would really type. If she copies it into any other AI tool, it works
 * there too — which is the point. She is learning to instruct, not learning
 * our buttons.
 */
export function compileInstruction(input: {
  exampleName: string;
  exampleSteal: string;
  sections: HomepageSection[];
  mode: WorkMode;
  styleLabel: string;
  who: string;
}): string {
  const mode = workModeById(input.mode);
  const list = input.sections
    .filter((s) => s.enabled)
    .map((s, i) => `${i + 1}. ${s.label}（${s.hint}）`)
    .join("\n");
  return [
    `我要做一个属于我自己的个人主页。我是${input.who}。`,
    ``,
    `【我要它像谁】`,
    `像 ${input.exampleName} 的页面。我想学的那一招是：${input.exampleSteal}`,
    ``,
    `【内容的结构】按这个顺序，不要多加也不要少：`,
    list,
    ``,
    `【风格】${input.styleLabel}。一种字体、一个主色，不要多余的装饰。`,
    ``,
    `【我们怎么合作】${mode.label}。${mode.instruction}`,
    ``,
    `最后一条，最重要：正文是我写的。你可以问我、可以给我骨架、可以帮我挑出啰嗦的地方，但不要替我写出可以直接抄的句子。`,
  ].join("\n");
}

/**
 * Scripted AI behaviour per section per mode. The prototype has no model, so
 * these carry the WEIGHT of proving the three modes really differ — if these
 * three arrays read the same, the whole teaching collapses.
 */
export const MODE_SCRIPTS: Record<
  WorkMode,
  Record<string, { lead: string; items: string[]; cta: string }>
> = {
  ask: {
    intro: {
      lead: "先问一个。别想着答得漂亮，答得具体就行。",
      items: ["你最近一次因为一件事查到半夜，那件事是什么？"],
      cta: "答完我再问下一个",
    },
    question: {
      lead: "一个问题。",
      items: ["有没有一个问题，你在不同的场合已经想过三次以上？"],
      cta: "答完我再问下一个",
    },
    detail: {
      lead: "一个。",
      items: ["有什么事你会做，但从来没跟人说过，因为觉得说了别人会觉得怪？"],
      cta: "答完我再问下一个",
    },
    contact: { lead: "一个。", items: ["你希望什么样的人来找你？把这句写在联系方式前面。"], cta: "答完我再问下一个" },
  },
  propose: {
    intro: {
      lead: "三个角度，骨架里的空格留给你。挑一个，或者告诉我都不对。",
      items: [
        "【从一个物件切入】「我家有一___，它___。因为它，我开始想___。」",
        "【从一个反复的问题切入】「我一直在想___，尤其是当___的时候。」",
        "【从一个矛盾切入】「大家都说___，但我发现___。」",
      ],
      cta: "选一个骨架，空格我自己填",
    },
    question: {
      lead: "三个角度。",
      items: [
        "【一个你不同意的说法】「很多人觉得___，我不确定，因为___。」",
        "【一个你想量出来的东西】「我想知道到底有多少___。」",
        "【一个你想替谁问的问题】「___没有人替他们问：___？」",
      ],
      cta: "选一个骨架，空格我自己填",
    },
    detail: {
      lead: "三个角度。",
      items: [
        "【一个你收集的东西】「我留着所有的___。」",
        "【一个你的怪习惯】「我做___之前一定要___。」",
        "【一个你数过的数字】「我数过___，一共___。」",
      ],
      cta: "选一个骨架，空格我自己填",
    },
    contact: {
      lead: "两个角度。",
      items: ["【开放式】「如果你也在想___，写信给我。」", "【限定式】「只有一件事欢迎来找我：___。」"],
      cta: "选一个骨架，空格我自己填",
    },
  },
  tidy: {
    intro: {
      lead: "读完了。我不改你的句子，只标三处，改不改你定。",
      items: [
        "第 1 句和第 3 句说的是同一件事 —— 留一句更具体的那句。",
        "「我很喜欢科学」这里缺一个具体的例子。你上一次查到半夜的是什么？把它换进来。",
        "最后一句可以删。它在总结前面已经说清楚的事，读者不需要。",
      ],
      cta: "逐条决定",
    },
    question: {
      lead: "两处。",
      items: [
        "「我关心环保」太大了，大到没有信息。你其实关心的是一个更小的东西 —— 是哪一个？",
        "问句和陈述句混在一起了。这一段如果只留一个问句，它会更有力。",
      ],
      cta: "逐条决定",
    },
    detail: {
      lead: "一处。",
      items: ["这段很好，唯一的问题是你在解释它为什么有趣。删掉解释，只留那件事本身，会更有趣。"],
      cta: "逐条决定",
    },
    contact: { lead: "一处。", items: ["「欢迎大家交流」等于没写。换成一个具体的邀请：你希望谁来找你？"], cta: "逐条决定" },
  },
};

/** The six steps of PBL#0. */
export const HOMEPAGE_STEPS = [
  { id: "examples", label: "看看别人的家", sub: "六个真实的页面，和你能偷走的那一招" },
  { id: "instruction", label: "学会下清楚的指令", sub: "例子 + 结构 + 合作方式，编译成一段真的指令" },
  { id: "style", label: "选风格", sub: "一种字体，一个主色" },
  { id: "compose", label: "写内容", sub: "按你选的合作方式，一块一块写" },
  { id: "publish", label: "发布", sub: "预览，拿到你的网址" },
  { id: "share", label: "分享", sub: "把链接给一个人" },
] as const;

export type HomepageStepId = (typeof HOMEPAGE_STEPS)[number]["id"];
