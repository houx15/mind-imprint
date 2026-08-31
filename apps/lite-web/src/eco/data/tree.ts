import type { Field, FieldId, Keyword } from "./types";

/**
 * 我的树 — the keyword model, as a tree.
 *
 * The claim the tree makes is strong and has to be earned: **every keyword on
 * it came from something she actually did.** So every `Keyword` below carries
 * its `sources` — reading ids, writing ids, project ids, news ids that really
 * exist in the other data modules — and, wherever possible, HER OWN SENTENCE
 * as the evidence. A keyword with no sources is a decoration, and the drawer
 * would have nothing to show when she asks "为什么这个词在我的树上？".
 *
 * ## Geometry
 * Branches are cubic Béziers in a 1000×780 viewBox with the trunk at x=500.
 * A keyword's `at.t` is its position along its branch (0 = near the trunk,
 * 1 = the tip) and `at.spread` pushes it off the curve along the normal, so
 * leaves on the same branch don't stack. `pointOnBranch` below is the single
 * place that maps (t, spread) → (x, y); the tree component draws the same
 * curve, so nodes are always ON their branch by construction.
 */

export const FIELDS: Field[] = [
  { id: "science", label: "科学与自然", en: "Science & Nature", hue: "var(--mk-lake)", angle: -62 },
  { id: "humanities", label: "人文与写作", en: "Humanities & Writing", hue: "var(--mk-peach)", angle: -118 },
  { id: "making", label: "技术与创造", en: "Making & Tech", hue: "var(--mk-mist)", angle: -28 },
  { id: "society", label: "社会与世界", en: "Society & World", hue: "var(--mk-matcha)", angle: -152 },
  { id: "arts", label: "艺术与表达", en: "Arts & Expression", hue: "var(--mk-taro)", angle: -8 },
  { id: "self", label: "自我与成长", en: "Self & Growth", hue: "var(--mk-berry)", angle: -172 },
];

/** The six fields are a fixed literal list, so index 0 always exists; the
 *  assertion documents that rather than inviting an impossible undefined
 *  branch at every call site. */
const DEFAULT_FIELD = FIELDS[0]!;

export function fieldById(id: FieldId): Field {
  return FIELDS.find((f) => f.id === id) ?? DEFAULT_FIELD;
}

type Pt = [number, number];

/** The four cubic control points of each field's branch, trunk → tip. Typed as
 *  a fixed 4-tuple (not `Pt[]`) so destructuring gives four DEFINED points —
 *  the geometry helpers below would otherwise be littered with impossible
 *  undefined checks. */
export const BRANCH_CURVES: Record<FieldId, [Pt, Pt, Pt, Pt]> = {
  humanities: [
    [500, 352],
    [400, 300],
    [268, 268],
    [176, 168],
  ],
  science: [
    [500, 330],
    [606, 288],
    [738, 250],
    [836, 148],
  ],
  society: [
    [500, 470],
    [382, 452],
    [246, 424],
    [136, 352],
  ],
  making: [
    [500, 448],
    [626, 434],
    [760, 402],
    [880, 330],
  ],
  self: [
    [500, 588],
    [400, 592],
    [286, 578],
    [196, 528],
  ],
  arts: [
    [500, 574],
    [610, 580],
    [726, 568],
    [828, 516],
  ],
};

/** Point on a field's branch at parameter `t`, pushed `spread` px along the
 *  curve normal. One implementation, used by both the node layout and the
 *  connector stubs, so a node can never drift off its own branch. */
export function pointOnBranch(field: FieldId, t: number, spread = 0): { x: number; y: number } {
  const [p0, p1, p2, p3] = BRANCH_CURVES[field];
  const u = 1 - t;
  const bx = u * u * u * p0[0] + 3 * u * u * t * p1[0] + 3 * u * t * t * p2[0] + t * t * t * p3[0];
  const by = u * u * u * p0[1] + 3 * u * u * t * p1[1] + 3 * u * t * t * p2[1] + t * t * t * p3[1];
  // derivative → tangent → normal
  const dx =
    3 * u * u * (p1[0] - p0[0]) + 6 * u * t * (p2[0] - p1[0]) + 3 * t * t * (p3[0] - p2[0]);
  const dy =
    3 * u * u * (p1[1] - p0[1]) + 6 * u * t * (p2[1] - p1[1]) + 3 * t * t * (p3[1] - p2[1]);
  const len = Math.hypot(dx, dy) || 1;
  return { x: bx + (-dy / len) * spread, y: by + (dx / len) * spread };
}

export function branchPath(field: FieldId): string {
  const [p0, p1, p2, p3] = BRANCH_CURVES[field];
  return `M ${p0[0]} ${p0[1]} C ${p1[0]} ${p1[1]}, ${p2[0]} ${p2[1]}, ${p3[0]} ${p3[1]}`;
}

/**
 * The growth replay.
 *
 * 🚨 **The stops are RELATIVE to when she started, never absolute months.**
 * The first version read 三月 / 五月 / 七月 / 现在, which is fine for a student
 * who happens to have started in March and meaningless — worse, quietly false
 * — for everyone else. A student on her second day would be shown a five-month
 * growth story that is not hers.
 *
 * Relative buckets survive both ends of the range: someone three weeks in has
 * 起点 and 现在 and nothing between (see `stopsFor`, which drops the buckets
 * that hold nothing), and someone two years in still has five labels rather
 * than twenty-four months of dots.
 *
 * Index 3 is 现在; a keyword with `bornAt: 2` appears at the third stop and
 * stays.
 */
export const GROWTH_STOPS = [
  { label: "起点", sub: "刚开始的时候" },
  { label: "半年前", sub: "读得多起来" },
  { label: "近两个月", sub: "开始写" },
  { label: "现在", sub: "今天" },
];

/**
 * The stops worth showing, given what actually exists.
 *
 * A stop that holds no keywords is a dot she can press to see the same tree
 * she is already looking at. 现在 is always kept — there is always a present —
 * so a brand-new student gets a single label rather than a scrubber with
 * nothing to scrub.
 */
export function stopsFor(keywords: Keyword[]): number[] {
  const last = GROWTH_STOPS.length - 1;
  const live = GROWTH_STOPS.map((_, i) => i).filter(
    (i) => i === last || keywords.some((k) => k.bornAt <= i),
  );
  return live;
}

export const KEYWORDS: Keyword[] = [
  // ── 科学与自然 ───────────────────────────────────────────────────────────
  {
    id: "k-representative",
    text: "例外与代表性",
    en: "Exception vs. representative",
    field: "science",
    strength: 5,
    bornAt: 0,
    note: "你现在会先问「这个例子能代表多少」。这是统计思维的入口，也是你写议论文最锋利的一手。",
    at: { t: 0.46, spread: -56 },
    sources: [
      {
        kind: "reading",
        id: "r-coral",
        label: "红海北端那片不白化的珊瑚",
        date: "2026-08-29",
        evidence: "我一开始觉得这是好消息，读到面积那一段才反应过来它有多小。",
      },
      {
        kind: "writing",
        id: "w-coral",
        label: "一片珊瑚不能代表一片海",
        date: "2026-08-30",
        evidence: "一个避难所不是一个计划。我们不能拿百分之零点零二的运气，去替百分之九十九点九八做决定。",
      },
      { kind: "news", id: "n-0829-2", label: "有一小片珊瑚，学会了在热浪里活下来", date: "2026-08-29" },
    ],
    shining: {
      title: "你自己去查了那个面积",
      body: "没有人让你查。你读到「几乎没有白化」时停了下来，去找这片珊瑚有多大——4 平方公里。那一下，你把一条新闻变成了一个论证。",
      date: "2026-08-29",
    },
  },
  {
    id: "k-climate",
    text: "气候与海洋",
    en: "Climate & ocean",
    field: "science",
    strength: 3,
    bornAt: 0,
    note: "三次阅读都落在这里。它还没长成你的主问题，但它一直在。",
    at: { t: 0.42, spread: 30 },
    sources: [
      { kind: "reading", id: "r-coral", label: "红海北端那片不白化的珊瑚", date: "2026-08-29" },
      { kind: "news", id: "n-0830-1", label: "海洋吸走的碳，比我们以为的少 7%", date: "2026-08-30" },
      { kind: "writing", id: "w-coral", label: "一片珊瑚不能代表一片海", date: "2026-08-30" },
    ],
  },
  {
    id: "k-memory",
    text: "记忆怎么形成",
    en: "How memory forms",
    field: "science",
    strength: 2,
    bornAt: 0,
    note: "你读完就改了自己的作息。知识变成行动的次数，比知识本身值钱。",
    at: { t: 0.88, spread: 28 },
    sources: [
      {
        kind: "reading",
        id: "r-sleep",
        label: "睡觉时大脑在重放什么",
        date: "2026-08-25",
        evidence: "重放次数越多记得越牢——那我熬夜等于把重放的时间砍掉了。",
      },
    ],
  },

  // ── 人文与写作 ───────────────────────────────────────────────────────────
  {
    id: "k-ordinary-history",
    text: "普通人的历史",
    en: "History of ordinary people",
    field: "humanities",
    strength: 4,
    bornAt: 1,
    note: "你反复回到同一个念头：大事件之外，谁在记录日常。这可能是你的长期问题。",
    at: { t: 0.48, spread: 48 },
    sources: [
      {
        kind: "reading",
        id: "r-letters",
        label: "十万封普通人的家书",
        date: "2026-08-12",
        evidence: "出现最多的词是钱、天气和你吃了吗——这三个词现在也一样。",
      },
      {
        kind: "writing",
        id: "w-letters",
        label: "如果一百年后只剩我的聊天记录",
        date: "2026-08-14",
        evidence: "也许问题不是我的记录够不够体面，而是我有没有在真的过日子。",
      },
      { kind: "news", id: "n-0829-5", label: "十万封普通人的家书被扫描上线", date: "2026-08-29" },
    ],
  },
  {
    id: "k-hook",
    text: "钩子式开头",
    en: "The hook opening",
    field: "humanities",
    strength: 3,
    bornAt: 1,
    note: "你用过一次就用顺了：先甩一个具体的数字，再问一句。这是你自己的招式了。",
    at: { t: 0.38, spread: -28 },
    sources: [
      {
        kind: "writing",
        id: "w-letters",
        label: "如果一百年后只剩我的聊天记录",
        date: "2026-08-14",
        evidence: "十万封家书里出现最多的三个词：钱、天气、你吃了吗。",
      },
    ],
  },
  {
    id: "k-concession",
    text: "让步段",
    en: "Concession",
    field: "humanities",
    strength: 4,
    bornAt: 1,
    note: "「我承认……可是……」——你会主动写对方的道理了。会让步的人才有说服力。",
    at: { t: 0.90, spread: -32 },
    sources: [
      {
        kind: "writing",
        id: "w-coral",
        label: "一片珊瑚不能代表一片海",
        date: "2026-08-30",
        evidence: "我承认这片珊瑚很重要。……可是「有一个例外」和「问题解决了」之间，隔着一整个规模的问题。",
      },
    ],
    shining: {
      title: "你替对手把话说全了",
      body: "在反驳之前，你先把对方最有力的三个理由列出来，还承认它们是真的。初二能做到这一步的不多——这一段是你整篇文章最有力的地方。",
      date: "2026-08-30",
    },
  },

  // ── 社会与世界 ───────────────────────────────────────────────────────────
  {
    id: "k-scale",
    text: "规模感",
    en: "A sense of scale",
    field: "society",
    strength: 3,
    bornAt: 1,
    note: "4 平方公里 vs 34.4 万平方公里。你开始用「多大」去衡量一件事值不值得高兴。",
    at: { t: 0.6, spread: -24 },
    sources: [
      { kind: "writing", id: "w-coral", label: "一片珊瑚不能代表一片海", date: "2026-08-30" },
      { kind: "news", id: "n-0826-2", label: "全球新增电力里，太阳能第一次超过一半", date: "2026-08-26" },
    ],
  },
  {
    id: "k-who-decides",
    text: "谁替我们做了决定",
    en: "Who decided for us",
    field: "society",
    strength: 2,
    bornAt: 2,
    note: "从一盏修不好的台灯开始的问题。它可以长很大。",
    at: { t: 0.78, spread: 28 },
    sources: [
      { kind: "reading", id: "r-repair", label: "把螺丝钉重新设计一遍", date: "2026-08-18" },
      {
        kind: "writing",
        id: "w-repair",
        label: "被设计成修不好的东西",
        date: "2026-08-19",
        evidence: "那是一个选择——有人在图纸上决定了它修不好。",
      },
    ],
  },

  // ── 技术与创造 ───────────────────────────────────────────────────────────
  {
    id: "k-repair",
    text: "修理权",
    en: "Right to repair",
    field: "making",
    strength: 4,
    bornAt: 2,
    note: "你从一件小事（家里的台灯）走到了一个真实的公共议题。这条路很值钱。",
    at: { t: 0.63, spread: 36 },
    sources: [
      { kind: "reading", id: "r-repair", label: "把螺丝钉重新设计一遍", date: "2026-08-18" },
      { kind: "writing", id: "w-repair", label: "被设计成修不好的东西", date: "2026-08-19" },
      { kind: "news", id: "n-0829-4", label: "一家公司把螺丝钉重新设计了一遍", date: "2026-08-29" },
    ],
  },
  {
    id: "k-design-ethics",
    text: "设计的伦理",
    en: "Design ethics",
    field: "making",
    strength: 3,
    bornAt: 2,
    note: "「为最少数人设计」和「故意让它坏」是同一枚硬币的两面，你两面都看到了。",
    at: { t: 0.76, spread: -30 },
    sources: [
      {
        kind: "reading",
        id: "r-gesture",
        label: "为盲人设计的手势",
        date: "2026-07-20",
        evidence: "为最少数人做的设计，常常对所有人都更好。",
      },
      { kind: "reading", id: "r-repair", label: "把螺丝钉重新设计一遍", date: "2026-08-18" },
    ],
  },
  {
    id: "k-make-it",
    text: "把想法做出来",
    en: "Shipping it",
    field: "making",
    strength: 1,
    bornAt: 2,
    note: "刚冒头。等你的第一个项目发布，它会长起来。",
    at: { t: 0.26, spread: -28 },
    sources: [{ kind: "project", id: "p-homepage", label: "建一个属于我的主页", date: "2026-08-30" }],
  },

  // ── 艺术与表达 ───────────────────────────────────────────────────────────
  {
    id: "k-restraint",
    text: "克制",
    en: "Restraint",
    field: "arts",
    strength: 2,
    bornAt: 2,
    note: "修画只补 3%。你把它记下来了，还用在了自己的文章上——你删掉了两段。",
    at: { t: 0.72, spread: 28 },
    sources: [
      {
        kind: "reading",
        id: "r-restore",
        label: "修了十一年，只补了 3%",
        date: "2026-08-05",
        evidence: "克制也是技术。知道什么时候停手，比会画更难。",
      },
    ],
  },
  {
    id: "k-craft",
    text: "手艺与时间",
    en: "Craft & time",
    field: "arts",
    strength: 1,
    bornAt: 3,
    note: "只有一次来源。想让它长大，再读一篇同方向的就够了。",
    at: { t: 0.30, spread: -26 },
    sources: [{ kind: "reading", id: "r-restore", label: "修了十一年，只补了 3%", date: "2026-08-05" }],
  },

  // ── 自我与成长 ───────────────────────────────────────────────────────────
  {
    id: "k-silence",
    text: "留白与沉默",
    en: "Silence as space",
    field: "self",
    strength: 4,
    bornAt: 3,
    note: "读到 → 试了 → 写下来。三步都走完的词，在你的树上只有两个，这是其中一个。",
    at: { t: 0.60, spread: -40 },
    sources: [
      {
        kind: "reading",
        id: "r-wait",
        label: "老师多等三秒会发生什么",
        date: "2026-08-20",
        evidence: "0.9 秒到 3.5 秒——差的这 2.6 秒里，本来会有一个更完整的回答。",
      },
      {
        kind: "writing",
        id: "w-three-seconds",
        label: "我决定在小组讨论里闭嘴三秒",
        date: "2026-08-22",
        evidence: "我以前一直以为讨论是抢时间……现在我觉得，会留白的人才带节奏。",
      },
    ],
    shining: {
      title: "你把读到的东西，当天就试了",
      body: "大多数人读完就过去了。你在第二天的小组讨论里真的数了三秒，还记下了发生什么——「那句话如果我抢在两秒的时候说话，就永远不会出现」。",
      date: "2026-08-22",
    },
  },
  {
    id: "k-honesty",
    text: "能说真话的人",
    en: "Someone to be honest with",
    field: "self",
    strength: 2,
    bornAt: 3,
    note: "你读完写了一句只给自己看的话。它还没变成文章，但它是真的。",
    at: { t: 0.79, spread: 26 },
    sources: [
      {
        kind: "reading",
        id: "r-lonely",
        label: "青少年说孤独的时候",
        date: "2026-07-28",
        evidence: "我身边有很多人，但能说真话的可能只有一个半。",
      },
    ],
  },
  {
    id: "k-self-watch",
    text: "观察我自己",
    en: "Watching myself",
    field: "self",
    strength: 3,
    bornAt: 3,
    note: "你写过三篇「我做了什么、结果怎样」。这个习惯比任何一篇文章都重要。",
    at: { t: 0.30, spread: 40 },
    sources: [
      { kind: "writing", id: "w-three-seconds", label: "我决定在小组讨论里闭嘴三秒", date: "2026-08-22" },
      { kind: "writing", id: "w-letters", label: "如果一百年后只剩我的聊天记录", date: "2026-08-14" },
    ],
  },
];

export function keywordById(id: string): Keyword | undefined {
  return KEYWORDS.find((k) => k.id === id);
}

export function keywordsForField(field: FieldId): Keyword[] {
  return KEYWORDS.filter((k) => k.field === field);
}

/** Keywords visible at a growth stop (index into GROWTH_STOPS). */
export function keywordsAt(stop: number): Keyword[] {
  return KEYWORDS.filter((k) => k.bornAt <= stop);
}
