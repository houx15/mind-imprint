import type { Project, ThreadItem, TrackId } from "./types";

/**
 * PBL — tracks, entry doors, and the seed projects.
 *
 * ## The rewrite (2026-08-31)
 * v1 was a **plan with checkboxes**: pick a track, the system emitted six
 * steps, you ticked them off. It looked like project management and taught
 * project management. It was not agentic in any sense that matters — nothing
 * ever happened between the student and the AI; the AI's entire contribution
 * arrived in one burst at the start and then sat there.
 *
 * v2 is a **workbench**. A project is a running conversation with 印记 in
 * which it summons 工具卡 (see `data/cards.ts`) — real working surfaces with
 * real fields — and what she puts in them comes back into the conversation.
 * The unit of progress is a card that came back, not a box that got ticked.
 *
 * ## What replaced the plan
 *  - `TRACK_CARDS` in `data/cards.ts` — the sequence 印记 works through.
 *  - `OPENERS` here — what 印记 says when a project opens, per track.
 *  - `VARIANTS` here — the three options the 三个版本 card puts in front of
 *    her. They have to be genuinely different or the card teaches nothing.
 *
 * ## 排计划 is still a system action, not a consent ritual
 * 印记 deciding which card comes next is orchestration, and per AGENTS.md
 * 铁律 govern the student's own prose, not orchestration. What SHE confirms is
 * opening a card — the card never opens itself (铁律②).
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
    examples: ["我自己的个人主页", "一个整理本地老照片的站", "我们班的读书地图"],
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

/**
 * What 印记 says when a project opens.
 *
 * Short, and it ends by summoning the first card with a reason. Note what it
 * does NOT do: no plan, no timeline, no "here are your six steps". The first
 * thing that happens in a project is a question about why.
 */
export const OPENERS: Record<TrackId, string> = {
  website:
    "好，开工。做网站这件事，最容易犯的错是先想它长什么样——那样做出来的一定是你见过的东西的平均值。\n\n我们从别的地方开始。",
  design:
    "好。设计这条路上，「为谁做」不是客套话，它决定了后面每一个决定。\n\n先把这个说清楚，我们再谈样子。",
  game:
    "游戏这条路有个特点：你想讲的道理，最后是靠规则讲出来的，不是靠文字。\n\n所以先说说，你想让玩的人明白什么。",
  survey:
    "调查这条路，最贵的错误发生在最前面：问卷发出去就收不回来了。\n\n所以我们会花不少时间在「怎么问」上。先从头开始。",
  other:
    "不在那四条里，很好——那说明是你自己的题目。\n\n那我们就得先把它说清楚，我才知道能给你什么。",
};

/**
 * The three options the 三个版本 card puts in front of her.
 *
 * 🚨 They must be genuinely, structurally different. Three variations on one
 * idea teach a student that choosing is cosmetic, which is the opposite of
 * this card's whole point.
 */
export const VARIANTS: Record<TrackId, { key: string; name: string; shape: string; body: string[] }[]> = {
  website: [
    {
      key: "A",
      name: "一句话开场",
      shape: "长页 · 无导航",
      body: [
        "整个第一屏只有一句话：你是谁、你在想什么。",
        "往下滚是三件你做过的事，每件配一句「我为什么做它」。",
        "结尾放联系方式。没有导航栏，因为只有一页。",
      ],
    },
    {
      key: "B",
      name: "索引式",
      shape: "密 · 像一份目录",
      body: [
        "首页是一张列表：日期 + 标题 + 一句话，一屏能看到十几条。",
        "顶部一行小字说明这里是什么。",
        "点进去才是正文。密度优先，像一个人的档案柜。",
      ],
    },
    {
      key: "C",
      name: "一个作品打头",
      shape: "图先行",
      body: [
        "开头直接是你最好的一件作品，占满一屏，不解释。",
        "往下才是「这是谁做的」。",
        "其余作品做成小图排在最后，点开看。",
      ],
    },
  ],
  design: [
    {
      key: "A",
      name: "改造现有的",
      shape: "低成本 · 快",
      body: ["不做新东西，在现有的东西上加一层。", "好处：明天就能试。", "坏处：受限于原来的形状。"],
    },
    {
      key: "B",
      name: "从零做一个",
      shape: "彻底 · 慢",
      body: ["完全重新设计。", "好处：能解决根子上的问题。", "坏处：你可能做不完。"],
    },
    {
      key: "C",
      name: "只改流程不改东西",
      shape: "不做实物",
      body: ["东西不动，改的是人怎么用它——顺序、提示、谁先谁后。", "好处：几乎零成本。", "坏处：别人不觉得你做了东西。"],
    },
  ],
  game: [
    {
      key: "A",
      name: "卡牌",
      shape: "纸 · 3–6 人",
      body: ["一副牌，一条规则。", "好处：改规则只要改一句话。", "坏处：要有人陪你玩。"],
    },
    {
      key: "B",
      name: "一个人的小程序",
      shape: "屏幕 · 单人",
      body: ["点击就能玩，一局两分钟。", "好处：能发给任何人。", "坏处：做起来比纸慢十倍。"],
    },
    {
      key: "C",
      name: "现实里的一次活动",
      shape: "真人 · 一次性",
      body: ["在教室里真的玩一次，你当主持。", "好处：反应最真实。", "坏处：只能玩一次，要录下来。"],
    },
  ],
  survey: [
    {
      key: "A",
      name: "全年级问卷",
      shape: "广 · 浅",
      body: ["10 道题，尽量多人填。", "好处：能说「多少比例」。", "坏处：说不出原因。"],
    },
    {
      key: "B",
      name: "十个人的深访",
      shape: "窄 · 深",
      body: ["找 10 个人，每人聊 15 分钟。", "好处：能说出原因和故事。", "坏处：不能说「大家都」。"],
    },
    {
      key: "C",
      name: "去数，不去问",
      shape: "观察 · 不问人",
      body: ["直接去数真实发生的事，不依赖别人的记忆。", "好处：绕过了所有问卷偏差。", "坏处：只能测到看得见的东西。"],
    },
  ],
  other: [
    {
      key: "A",
      name: "做小做完",
      shape: "一周",
      body: ["把范围压到最小，一周内彻底做完一版。", "好处：你会真的做完。", "坏处：野心要收起来。"],
    },
    {
      key: "B",
      name: "做一个片段",
      shape: "样片",
      body: ["不做整个，只做最关键的一小段，做到最好。", "好处：能展示水平。", "坏处：不是完整的东西。"],
    },
    {
      key: "C",
      name: "先做记录",
      shape: "过程即作品",
      body: ["把做的过程本身做成作品。", "好处：过程一定有。", "坏处：需要你写得住。"],
    },
  ],
};

export function variantsFor(track: TrackId) {
  return VARIANTS[track];
}

/** Read off her tree — the hub proposes projects that have evidence behind
 *  them, so a proposal is never a generic prompt. */
export const TRACK_PROPOSALS: {
  track: TrackId;
  title: string;
  from: string;
  why: string;
}[] = [
  {
    track: "survey",
    title: "我们年级有多少东西是「修不好」被扔的",
    from: "修理权 · 谁替我们做了决定",
    why: "你在这条线上有三个来源，还写过一篇。缺的正好是数据。",
  },
  {
    track: "design",
    title: "重新设计学校里一个真的很难用的东西",
    from: "设计的伦理",
    why: "你已经看到「为最少数人设计」这一招。它需要一个真实的对象来验证。",
  },
  {
    track: "website",
    title: "我自己的个人主页",
    from: "把想法做出来",
    why: "你读过写过的东西已经够摆一页了。做这一页会顺带教会你怎么给 AI 下指令。",
  },
  {
    track: "game",
    title: "一个关于「让步」的辩论卡牌",
    from: "让步段",
    why: "你会写让步段了。把它变成规则，玩过的人也会。",
  },
];

/**
 * Seed projects. One finished, one mid-flight — the hub has to show both
 * states or a new student cannot tell what "done" looks like here.
 */
export const SEED_PROJECTS: Project[] = [
  {
    id: "p-lamp",
    track: "design",
    title: "让爷爷看得清的药盒",
    intent: "我想做一个我爷爷真的能用的药盒，不是那种好看但他看不清的。",
    status: "published",
    cover: "var(--mk-taro)",
    startedAt: "2026-06-12",
    summary:
      "做了三版。第一版好看但他看不清；第二版能看清但装不下他的药；第三版用红色胶带分格，丑，但他现在自己会拿。我学到的是：给别人做的东西，好不好看由我说了不算。",
    // This one models a student who SKIPPED cards and went straight to
    // building. The rail will show six cards she never opened, and that is the
    // truthful picture — the sequence is 印记's suggestion, not a gate.
    thread: [
      { id: "t1", kind: "say", role: "coach", text: OPENERS.design },
      { id: "t2", kind: "card", cardId: "motive" },
      {
        id: "t3",
        kind: "say",
        role: "coach",
        text: "记下了。这个项目是为我爷爷做的，不做的代价是他上周吃错了一次。\n\n下一张我想请你先把要问清楚的事拆开。",
      },
      { id: "t4", kind: "say", role: "student", text: "我不想拆了，我想先做一个给他看。" },
      {
        id: "t5",
        kind: "say",
        role: "coach",
        text: "行。那就跳过前面几张，直接做——不过做之前先定一件事：什么结果算失败。做完再定标准的人，永远都会成功。",
      },
      { id: "t6", kind: "card", cardId: "proto" },
      {
        id: "t7",
        kind: "say",
        role: "coach",
        text: "他没问，但他把周三的撕下来贴到了周一上。这一条比「成功了」有用得多——你按他的顺序改了，说明你把他当用户，不是当评委。",
      },
      { id: "t8", kind: "card", cardId: "ship" },
    ],
    cards: [
      {
        cardId: "motive",
        status: "done",
        doneAt: "2026-06-12",
        values: {
          who: "我爷爷。78 岁，看不清小字，每天四种药。",
          cost: "他上周吃错了一次。我妈现在每天要打两个电话确认。",
          mine: "只有我知道他其实看得清红色，看不清蓝色。",
        },
      },
      {
        cardId: "proto",
        status: "done",
        doneAt: "2026-06-24",
        values: {
          smallest: "一张纸做的标签，只做周一到周三。",
          who: "爷爷",
          fail: "他还是要问我「今天吃哪个」——那就是失败。",
          result: "他没问。但他把周三的撕下来贴到了周一上，因为他觉得顺序反了。我按他的顺序改了。",
        },
      },
      {
        cardId: "ship",
        status: "done",
        doneAt: "2026-07-02",
        values: {
          made: "做了三版，最后一版最丑但他在用。",
          unfinished: "只做了一周的量，一个月的还没做。",
          learned: "第一版我花了两小时排版。应该先给他看一张纸的。",
        },
      },
    ],
  },
  {
    id: "p-wait",
    track: "survey",
    title: "同学在讨论里平均等几秒",
    intent: "我想知道我们班的人到底给别人留多少时间。",
    status: "running",
    cover: "var(--mk-matcha)",
    startedAt: "2026-08-20",
    // 🚨 A seed project's thread must contain an invitation for every card the
    // rail shows as 待开. The first version stopped at 动机三问 while the rail
    // already listed 问题清单 as available — so the conversation looked stalled
    // next to a rail that had moved on.
    thread: [
      { id: "t1", kind: "say", role: "coach", text: OPENERS.survey },
      { id: "t2", kind: "card", cardId: "motive" },
      {
        id: "t3",
        kind: "say",
        role: "coach",
        text: "记下了。这个项目是为我们班每次讨论都不说话的那四个人做的，不做的代价是他们的想法一直没被听到。\n\n我把这三句钉在上面了。到第三周你想放弃的时候，回来读一遍——多数时候管用。",
      },
      { id: "t4", kind: "card", cardId: "questions" },
    ],
    cards: [
      {
        cardId: "motive",
        status: "done",
        doneAt: "2026-08-20",
        values: {
          who: "我们班每次讨论都不说话的那四个人。",
          cost: "他们的想法一直没被听到，久了大家默认他们没想法。",
          mine: "我自己就是被抢过话的那个，我知道那三秒有多长。",
        },
      },
      { cardId: "questions", status: "invited", values: {} },
    ],
  },
];

/** Opening thread for a new project. */
export function openingThread(track: TrackId, firstCard: string): ThreadItem[] {
  return [
    { id: "t-open", kind: "say", role: "coach", text: OPENERS[track] },
    { id: "t-first", kind: "card", cardId: firstCard },
  ];
}
