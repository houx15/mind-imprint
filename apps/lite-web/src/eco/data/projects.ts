import type { Project, ProjectStep, StepKind, TrackId } from "./types";

/**
 * PBL — tracks, the why-ladder, and the plan the AI lays out.
 *
 * ## Two rulings this file encodes
 *
 * 1. **动机先于计划。** The why-ladder (`WHY_LADDER`) is three questions deep
 *    and the UI spends real time on it, because a project a student cannot say
 *    why she is doing dies at step three. The answers become a 动机卡 pinned
 *    to the project header and resurfaced later ("你当初说…").
 *
 * 2. **排计划是系统的确定性动作。** Generating the step list from a track is
 *    something the system just DOES — per AGENTS.md, 铁律①/② govern the
 *    student's own writing (AI never authors her body text), not orchestration.
 *    So there is no "confirm the AI may plan for you" gate. She can reorder,
 *    edit, delete and add steps afterwards, which is a usability choice, not a
 *    consent ritual.
 */

export const TRACKS: {
  id: TrackId;
  label: string;
  en: string;
  blurb: string;
  hue: string;
  glyph: string;
  /** What she'd hand someone at the end. Concrete, not "an artifact". */
  ends: string;
  examples: string[];
}[] = [
  {
    id: "design",
    label: "一个设计",
    en: "A design",
    blurb: "为一个真实的人、真实的场景，做出一个能用的东西。",
    hue: "var(--mk-taro)",
    glyph: "◇",
    ends: "一份可以交给别人去做的设计稿 + 一次真人试用的记录",
    examples: ["给爷爷设计一个他看得清的药盒", "重新设计学校的失物招领流程", "一套让人愿意修的台灯"],
  },
  {
    id: "website",
    label: "一个网站",
    en: "A website",
    blurb: "把一件你在乎的事，做成任何人都能打开的一页。",
    hue: "var(--mk-mist)",
    glyph: "⌘",
    ends: "一个能分享出去的网址",
    examples: ["一个整理本地老照片的站", "「你家的东西修得好吗」自查工具", "我们班的读书地图"],
  },
  {
    id: "game",
    label: "一个游戏",
    en: "A game",
    blurb: "用规则讲一个道理。玩过的人会真的懂。",
    hue: "var(--mk-berry)",
    glyph: "◈",
    ends: "一个能被别人玩到结束的小游戏（纸上的也算）",
    examples: ["一个关于「让步」的辩论卡牌", "模拟珊瑚白化的桌游", "三秒沉默练习游戏"],
  },
  {
    id: "survey",
    label: "一份调查报告",
    en: "A survey & report",
    blurb: "去问真实的人，把答案变成别人无法忽视的东西。",
    hue: "var(--mk-matcha)",
    glyph: "◎",
    ends: "一份有数据、有方法说明、有结论的报告",
    examples: ["我们年级有多少东西是「修不好」被扔的", "同学在讨论里平均等几秒", "家里最久没被打开的抽屉里有什么"],
  },
  {
    id: "other",
    label: "其他",
    en: "Something else",
    blurb: "你想做的事不在上面四条里。那更好——先说说它。",
    hue: "var(--mk-peach)",
    glyph: "✳",
    ends: "由你和印记一起定义",
    examples: ["办一次展览", "拍一支两分钟的片子", "写一封会被真的寄出去的信"],
  },
];

export function trackById(id: TrackId) {
  return TRACKS.find((t) => t.id === id) ?? TRACKS[0]!;
}

export const STEP_META: Record<StepKind, { label: string; glyph: string; hue: string }> = {
  learn: { label: "学", glyph: "◐", hue: "var(--mk-mist)" },
  research: { label: "查", glyph: "◎", hue: "var(--mk-matcha)" },
  design: { label: "设计", glyph: "◇", hue: "var(--mk-taro)" },
  make: { label: "做", glyph: "▣", hue: "var(--mk-peach)" },
  document: { label: "记录", glyph: "✎", hue: "var(--mk-butter)" },
  test: { label: "试", glyph: "⟳", hue: "var(--mk-berry)" },
  publish: { label: "发布", glyph: "▲", hue: "var(--mk-accent-400)" },
};

/** The three-rung why-ladder. Order matters: outward (谁), then stakes (代价),
 *  then inward (你). Asking "你为什么在乎" first gets a shrug; asking it third,
 *  after she has already named a person and a cost, gets an answer. */
export const WHY_LADDER: { id: "who" | "cost" | "mine"; q: string; hint: string; probe: string }[] = [
  {
    id: "who",
    q: "这个东西做出来，谁会用它？",
    hint: "说一个具体的人，不是「大家」。一个名字最好。",
    probe: "「所有人」等于没有人。你脑子里其实有一个具体的人——他叫什么？他多大？他在什么时候会需要这个？",
  },
  {
    id: "cost",
    q: "如果没人做这件事，会怎样？",
    hint: "写下真实的代价。如果想不出代价，这个项目可能还没找到。",
    probe: "「会有点可惜」不是代价。谁会多花时间？谁会被误解？什么东西会被扔掉？说一件具体会发生的事。",
  },
  {
    id: "mine",
    q: "别人也能做这件事。为什么是你？",
    hint: "你身上的什么，让你比别人更适合做这个。可以很小。",
    probe: "不用说你有多厉害。说一件你经历过、别人没经历过的事——那就是理由。",
  },
];

/** Plan templates per track. `why` is what makes this a plan and not a
 *  checklist: every step says what it is for. */
const PLANS: Record<TrackId, Omit<ProjectStep, "id" | "done">[]> = {
  survey: [
    {
      kind: "learn",
      title: "先学：一份问卷怎么才不会骗自己",
      why: "问题的问法会决定答案。学会三件事：别用引导性提问、别让人回忆太久、留一个开放题。",
      minutes: 25,
    },
    {
      kind: "design",
      tool: "survey",
      title: "设计问卷：8 题以内",
      why: "题目越多，认真填的人越少。八题是一个初中生愿意填完的上限。",
      minutes: 40,
    },
    {
      kind: "research",
      title: "去发出去，收回至少 30 份",
      why: "30 是一个能开始看出趋势的最小数字。不到 30，你的结论只是巧合。",
      minutes: 90,
    },
    {
      kind: "document",
      title: "把数据画成三张图",
      why: "别人不会读你的表格。三张图 = 三个结论，多了就没人记得住。",
      minutes: 50,
    },
    {
      kind: "make",
      title: "写报告：结论放最前面",
      why: "报告不是侦探小说。先说你发现了什么，再说你怎么发现的。",
      minutes: 70,
    },
    {
      kind: "test",
      title: "找一个没参与的人读一遍",
      why: "你已经知道太多了，看不出哪里读不懂。找一个局外人，只问一句：你觉得我在说什么？",
      minutes: 20,
    },
    { kind: "publish", title: "发布到我的主页", why: "做完了要有人看得见。它会出现在你的主页上。", minutes: 15 },
  ],
  website: [
    { kind: "learn", title: "先学：一页网站需要哪几块", why: "结构先定，内容才有地方放。看三个真实的例子，抄结构不抄内容。", minutes: 25 },
    { kind: "research", title: "查：已经有人做过吗？做成什么样？", why: "找到三个做过的人，你才知道自己的那一版为什么值得存在。", minutes: 35 },
    { kind: "design", title: "画出这一页的样子（纸上就行）", why: "先在纸上排，能省掉一半改代码的时间。", minutes: 40 },
    { kind: "make", title: "把它做出来", why: "做的过程会推翻一部分设计，这是正常的。", minutes: 120 },
    { kind: "test", title: "让三个人在你面前打开它", why: "不要问「好看吗」。看他们第一眼点了哪里，那才是真话。", minutes: 30 },
    { kind: "document", title: "写一段：我为什么做成这样", why: "别人会问。写下来，你自己也会更清楚。", minutes: 25 },
    { kind: "publish", title: "发布 + 放到我的主页", why: "拿到网址，贴到主页上。", minutes: 15 },
  ],
  design: [
    { kind: "research", title: "去看那个人真的怎么用", why: "设计的第一步不是画，是看。你会发现你以为的问题不是问题。", minutes: 60 },
    { kind: "learn", title: "先学：一个「好用」的东西满足什么", why: "可见性、反馈、容错——三个词，够你判断你自己的设计了。", minutes: 30 },
    { kind: "design", title: "出三个方向，不是一个", why: "只有一个方案时，你会爱上它，看不见问题。三个才有得比。", minutes: 60 },
    { kind: "make", title: "做一个能拿在手里的粗模型", why: "纸板、泡沫、乐高都行。能拿在手里，问题会自己跳出来。", minutes: 80 },
    { kind: "test", title: "给那个真实的人试", why: "别解释怎么用。他不会用的地方，就是你要改的地方。", minutes: 40 },
    { kind: "document", title: "记录：改了什么，为什么改", why: "改动的理由比成品更值钱，那是你的思考过程。", minutes: 30 },
    { kind: "publish", title: "发布到我的主页", why: "把粗模型的照片一起放上去。过程也是作品。", minutes: 15 },
  ],
  game: [
    { kind: "learn", title: "先学：规则怎么教会一个道理", why: "好的教育游戏里，你不是被告知道理，你是被规则逼着体验到它。", minutes: 30 },
    { kind: "design", title: "写下核心规则（三条以内）", why: "三条以内的规则，别人五分钟就能上手。多了就只有你一个人会玩。", minutes: 45 },
    { kind: "make", title: "做纸原型", why: "纸和笔就能测规则。先别碰电脑。", minutes: 60 },
    { kind: "test", title: "找两个人玩一局，你不许解释", why: "你一开口解释，测试就失效了。忍住。", minutes: 40 },
    { kind: "design", title: "改规则，再玩一局", why: "第一版规则一定有漏洞。改一次比想十次有用。", minutes: 45 },
    { kind: "document", title: "写规则说明书", why: "能写清楚规则，才算真的做完了。", minutes: 30 },
    { kind: "publish", title: "发布到我的主页", why: "把规则和一局的照片放上去，别人可以照着玩。", minutes: 15 },
  ],
  other: [
    { kind: "research", title: "先说清楚你到底想做什么", why: "「其他」意味着还没有现成的路。第一步是把它说成一句话。", minutes: 30 },
    { kind: "learn", title: "找到做过类似事的人", why: "无论多特别，总有人做过一半。找到他们能省掉几周。", minutes: 40 },
    { kind: "design", title: "定义「做完了」是什么样", why: "没有终点的项目会一直拖。写下一个可以被检查的完成标准。", minutes: 30 },
    { kind: "make", title: "做出第一个版本", why: "先做出来，再变好。", minutes: 120 },
    { kind: "test", title: "给真实的人看", why: "在你自己房间里成立的东西，不一定在外面成立。", minutes: 40 },
    { kind: "document", title: "记录过程", why: "「其他」类的项目，过程往往比成果更值得说。", minutes: 30 },
    { kind: "publish", title: "发布到我的主页", why: "做完了要有人看得见。", minutes: 15 },
  ],
};

/** Build a fresh plan for a track. Ids are stable within a plan so reordering
 *  and editing in the store can address steps. */
export function planFor(track: TrackId): ProjectStep[] {
  return PLANS[track].map((s, i) => ({ ...s, id: `${track}-s${i + 1}`, done: false }));
}

/** Total planned time, humanised — the plan tells her what she is signing up
 *  for before she starts, which is the honest thing to do. */
export function planHours(steps: ProjectStep[]): string {
  const total = steps.reduce((sum, s) => sum + s.minutes, 0);
  return `${Math.round((total / 60) * 10) / 10} 小时`;
}

/** 印记's proposal when she chooses 「先和印记聊聊」 instead of picking a track.
 *  It reads HER TREE — that is the whole point of having a tree. */
export const TRACK_PROPOSALS: {
  fromKeyword: string;
  track: TrackId;
  title: string;
  pitch: string;
}[] = [
  {
    fromKeyword: "修理权",
    track: "survey",
    title: "我们年级到底扔掉了多少还能修的东西",
    pitch:
      "你的树上「修理权」这个词有三个来源，还写过一篇《被设计成修不好的东西》。你已经有观点了，缺的是证据——去数一数你们年级一学期扔了多少还能修的东西，这个数字会让你那篇文章变得不可反驳。",
  },
  {
    fromKeyword: "留白与沉默",
    track: "game",
    title: "三秒沉默：一个逼人闭嘴的讨论游戏",
    pitch:
      "你读了「多等三秒」，还真的在小组里试了一次，写下了发生什么。这是完整的一圈。把它做成规则，别人也能试——你已经知道它有用，现在让二十个人知道。",
  },
  {
    fromKeyword: "普通人的历史",
    track: "website",
    title: "一个收集家里旧物件故事的站",
    pitch:
      "「普通人的历史」是你树上来源最多的词之一。你说过历史由琐碎的句子铺成——那就去收一百个句子。一个网站可以让别人替你收。",
  },
];

/** The already-published project, so the personal page and the tree have
 *  something real to point at from the first screen. */
export const SEED_PROJECTS: Project[] = [
  {
    id: "p-lamp",
    track: "design",
    title: "让那盏台灯能被修好",
    status: "published",
    cover: "var(--mk-peach)",
    startedAt: "2026-07-02",
    summary:
      "家里的台灯被胶封死，修灯师傅说拆开就废了。我把它拆了（废了），量了尺寸，重新画了一版用三颗同规格螺丝的外壳，3D 打印出来装回去。现在它能亮，也能再拆。",
    motivation: {
      who: "我爸。那盏灯是他用了十年的，他不肯扔，一直放在阳台。",
      cost: "它会一直放在阳台上，直到某天被当成垃圾扔掉。而且我会一直以为「修不好」是技术问题。",
      mine: "我拆过它，我知道里面是什么样。而且我是唯一一个在乎那盏灯的人。",
    },
    steps: planFor("design").map((s, i) => ({ ...s, done: i < 7 })),
  },
];
