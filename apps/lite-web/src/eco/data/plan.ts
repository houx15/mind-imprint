import type { Approach, PlanStep, ToolKind, TrackId } from "./types";
import { cardById, cardsForTrack } from "./cards";

/**
 * 项目计划 — what 印记 proposes before anything is built.
 *
 * ## Why this file exists
 * The workbench used to go straight from "hello" to the first 工具卡. That is
 * not how an agentic tool behaves and not how a competent collaborator works
 * either: a plan goes in front of you first, and it waits. The plan is the
 * artefact that lets a student disagree before three weeks are spent.
 *
 * ## What the plan screen shows, and what it deliberately does not
 * Per step: **a number, a name, one line.** That is all.
 *
 * 🚨 An earlier version printed 你带来 / 印记做 / 决策要点 on every node, plus a
 * field for scheduling each step. It turned a route into a wall of
 * specification — seven of those is not something a student reviews, it is
 * something she skims — and the scheduling in particular was ceremony: asking a
 * thirteen-year-old to date seven steps before she knows what any of them
 * involve produces seven guesses, not a plan she owns.
 *
 * The detail moved to **the step itself**, where it is actionable, as
 * `goal` + the split of work below.
 *
 * ## 分工方案 — the three lines every step must be able to fill
 *   - **印记做** (`iBring`) — what the AI does. In a project it may genuinely
 *     build: write the code, draw the layout, assemble the list.
 *   - **你来做** (`youBring`) — her half. Concrete, never 「参与讨论」.
 *   - **然后** (`then`) — what she hands back, which is what turns an
 *     assignment into a division of labour with a meeting point.
 *
 * And `decide`: the judgement that stays hers. **Every step has one.** If a
 * step cannot name a decision she owns, the honest thing is to let 印记 do it
 * silently rather than seat her in front of it and call it learning.
 */

/** 工具分类 — the label a student sees on every instrument. */
export const TOOL_KINDS: Record<ToolKind, { label: string; blurb: string; hue: string }> = {
  plan: {
    label: "计划工具",
    blurb: "确定做什么、为谁做、按什么顺序做。",
    hue: "var(--mk-accent-400)",
  },
  research: {
    label: "调研工具",
    blurb: "去现场、去问人、去看别人已经做过的。",
    hue: "var(--mk-lake)",
  },
  frame: {
    label: "框架工具",
    blurb: "把模糊的想法变成一份别人能执行的说明。",
    hue: "var(--mk-taro)",
  },
  decide: {
    label: "决策工具",
    blurb: "在几个都说得通的选项之间做出选择，并说明理由。",
    hue: "var(--mk-butter)",
  },
  question: {
    label: "质疑工具",
    blurb: "检查一个说法、一份数据、一道题目站不站得住。",
    hue: "var(--mk-berry)",
  },
  review: {
    label: "审查工具",
    blurb: "在交出去之前，找出自己作品里的问题。",
    hue: "var(--mk-matcha)",
  },
  reflect: {
    label: "反思工具",
    blurb: "回头看这一段做了什么、学到了什么、还差什么。",
    hue: "var(--mk-peach)",
  },
};

/* ── the personal page ────────────────────────────────────────────────────
 * The journey the product proposes first. Note the order: she names the style
 * she wants BEFORE she looks at other people's pages. That is deliberate —
 * coming back to three words you wrote an hour ago and finding two of them
 * gone is the cheapest lesson in this whole sequence.
 * ---------------------------------------------------------------------- */
const HOMEPAGE: PlanStep[] = [
  {
    id: "hp-why",
    title: "受众分析",
    blurb: "确定这个页面写给谁看。",
    goal: "找到一个具体的读者。页面上的每一个决定，最后都要回到这个人身上。",
    youBring: "想象一个真实的读者：他会在什么时候打开这一页，你希望他看完是什么感受",
    iBring: "追问到你说得出一个具体的人为止",
    then: "把这个人写下来给我，之后我做的每一版都会按他来判断",
    decide: "这一页为谁存在",
    opens: { kind: "card", cardId: "motive" },
  },
  {
    id: "hp-words",
    title: "关键词定义",
    blurb: "用三个词定义你想要的页面风格。",
    goal: "先说出你以为自己想要什么，再去看别人的。看完回来对一次，你会发现有几个词换掉了。",
    youBring: "三个描述风格的词，和一个你明确不想要的样子",
    iBring: "把这三个词记下来，之后每一版都拿它对照",
    then: "案例调研做完之后回来，把变化的那几个词改掉",
    decide: "你想要的到底是什么感觉",
    opens: { kind: "card", cardId: "keywords" },
  },
  {
    id: "hp-look",
    title: "案例调研",
    blurb: "找几个你最喜欢的个人网站，说说你为什么喜欢。",
    goal: "把「好看」拆成你说得出口、我做得出来的东西。",
    youBring: "至少四个你真的喜欢的例子，每个写清楚你喜欢它哪一处",
    iBring: "给你一份可以直接打开的名单",
    then: "把这四处具体的地方写给我，我按它们来做方案",
    decide: "哪些是你真正想要的，哪些只是当下觉得好看",
    opens: { kind: "card", cardId: "sweep" },
  },
  {
    id: "hp-style",
    title: "方案对比与选择",
    blurb: "印记提供三个方案，你选择一个并说明理由。",
    goal: "在三个都说得通的方案之间做一次有理由的选择。理由会成为后面所有工作的依据。",
    youBring: "一个选择，和你为什么不选另外两个",
    iBring: "按你的关键词和案例，做三版方向不同的方案",
    then: "把你的理由写下来，我后面每一步都按它执行",
    decide: "选哪一版，以及为什么不是另外两版",
    opens: { kind: "make", artifactId: "hp-style" },
  },
  {
    id: "hp-content",
    title: "内容规划",
    blurb: "确定这个页面上呈现哪些内容。",
    goal: "让这一页上的每一句话都是你自己的话。",
    youBring: "删掉不该在的，改掉不像你的，补上只有你知道的",
    iBring: "从你读过、写过、做过的东西里草拟每一块的内容",
    then: "把改过的版本交回来，改过的句子我原样使用",
    decide: "读者看完这一页，应该记住你哪一点",
    opens: { kind: "make", artifactId: "hp-content" },
  },
  {
    id: "hp-build",
    title: "页面构建与验收",
    blurb: "印记完成开发，你逐轮提出修改意见。",
    goal: "把方案变成一个真的能打开的页面，并由你决定什么程度算完成。",
    youBring: "具体的修改意见——「第二屏的字太小」这种，我能直接执行",
    iBring: "写代码、排版、出图，按你的意见一轮一轮改",
    then: "确认它可以了，并说明你的判断标准",
    decide: "什么程度算做完了",
    opens: { kind: "make", artifactId: "hp-build" },
  },
  {
    id: "hp-ship",
    title: "发布与复盘",
    blurb: "发布页面，并记录这一版的已知不足。",
    goal: "把作品交出去，同时诚实地记下它现在还差什么。",
    youBring: "一段说明：做出了什么、哪里还不够、下一版想改什么",
    iBring: "把它挂到你的主页地址上",
    then: "发布，并把这段说明一起发出去",
    decide: "现在发布，还是再改一轮",
    opens: { kind: "card", cardId: "ship" },
  },
];

/* ── the community garden, road A: a map with numbered points ──────────── */
const GARDEN_MAP: PlanStep[] = [
  {
    id: "gd-recon",
    title: "实地调研",
    blurb: "走一遍花园，记录路线和容易走错的位置。",
    goal: "拿到一份现场才有的资料：人具体在哪几个位置会犹豫。",
    youBring: "一张你自己画的图，和你标出来的迷路位置",
    iBring: "给你一份踏勘清单，回来帮你把手画的图整理成底图",
    then: "把图和迷路位置带回来，后面每一步我都用它",
    decide: "哪些岔路是真的会让人走错的",
    opens: { kind: "card", cardId: "recon" },
  },
  {
    id: "gd-ask",
    title: "问卷调研",
    blurb: "向住户收集真实回答，验证你的观察。",
    goal: "把「我觉得」变成「我问过」。一个人的观察是一个样本。",
    youBring: "改掉我写坏的题目，再写一句你自己的邀请",
    iBring: "按你标的位置拟一份问卷，并标出我自己写得有问题的题目",
    then: "问卷发出去，收回来的答案我们一起看",
    decide: "哪些题该问，哪些题问了也拿不到有用的答案",
    opens: { kind: "make", artifactId: "gd-ask" },
  },
  {
    id: "gd-points",
    title: "标记点规划",
    blurb: "确定标记点的数量和位置。",
    goal: "在「够用」和「你做得完」之间定一个数字。",
    youBring: "一个数字，和你选它的理由",
    iBring: "按你的图和收回来的答案，算三种密度并写清楚各自的代价",
    then: "定下数字之后，我按它来做页面上的编号",
    decide: "覆盖多少，维护多少",
    opens: { kind: "make", artifactId: "gd-points" },
  },
  {
    id: "gd-build",
    title: "页面开发",
    blurb: "印记开发扫码页面，你在手机上验收。",
    goal: "做出一个扫码就能用的页面，并由你确认它对老人和小孩也成立。",
    youBring: "在手机上真的扫一次，然后说清楚哪里不对",
    iBring: "写这个页面：地图、当前位置、每个点的编号",
    then: "确认它可以了，我出最终版和二维码",
    decide: "第一眼看不懂的人，能不能自己走出去",
    opens: { kind: "make", artifactId: "gd-build" },
  },
  {
    id: "gd-test",
    title: "原型测试",
    blurb: "先印三张二维码，找一个不认识花园的人实地走一次。",
    goal: "在做完整版之前，先知道它会不会失败。",
    youBring: "一个真的不认识花园的人，和一次真的测试",
    iBring: "帮你在测试之前先把「什么算失败」定下来",
    then: "把测试结果带回来，包括他卡住的地方",
    decide: "这次测试算成功还是失败",
    opens: { kind: "card", cardId: "proto" },
  },
  {
    id: "gd-talk",
    title: "沟通与协调",
    blurb: "和物业沟通，取得张贴许可。",
    goal: "拿到许可。东西做得再好，贴不上去等于没做。",
    youBring: "一次真的对话，和对方的原话",
    iBring: "陪你把对方的顾虑和你能给的条件列清楚",
    then: "把他提的条件带回来，我按条件改最终版",
    decide: "他提的条件，哪些你接受",
    opens: { kind: "card", cardId: "talk" },
  },
  {
    id: "gd-ship",
    title: "修订与交付",
    blurb: "按沟通结果修订，完成交付。",
    goal: "把东西真的装到园子里，并写清楚还有什么没做完。",
    youBring: "一段交付说明，包括已知不足",
    iBring: "出最终版的页面和二维码",
    then: "交付，并把说明一起交出去",
    decide: "现在交付，还是再改一轮",
    opens: { kind: "card", cardId: "ship" },
  },
];

/* ── the community garden, road B: physical signage ────────────────────── */
const GARDEN_SIGNS: PlanStep[] = [
  {
    id: "gs-recon",
    title: "实地调研",
    blurb: "走一遍花园，找出人们真正会犹豫的路口。",
    goal: "标牌立在哪里，取决于人在哪里犹豫。所以先去看人在哪里犹豫。",
    youBring: "一张你自己画的图，和你标出来的迷路位置",
    iBring: "给你一份踏勘清单，回来帮你整理",
    then: "把犹豫的位置带回来，它就是标牌的位置清单",
    decide: "哪几个路口是真的需要标牌的",
    opens: { kind: "card", cardId: "recon" },
  },
  {
    id: "gs-sign",
    title: "标识方案设计",
    blurb: "在三种标识写法之间做出选择。",
    goal: "决定这些牌子是帮人走完这一次，还是让人记住这个园子。",
    youBring: "一个选择，和你的理由",
    iBring: "把编号、地名、箭头加距离三种写法各做一版",
    then: "定下写法之后，我出可以直接打印的图纸",
    decide: "解决「这一次」，还是解决「以后」",
    opens: { kind: "make", artifactId: "gs-signs" },
  },
  {
    id: "gs-test",
    title: "原型测试",
    blurb: "先做三块纸质标牌贴上去试。",
    goal: "在做成实物之前先犯错。纸是可以扔的。",
    youBring: "一个不认识花园的人，和一次真的测试",
    iBring: "帮你在测试之前先把「什么算失败」定下来",
    then: "把测试结果带回来",
    decide: "这次测试算成功还是失败",
    opens: { kind: "card", cardId: "proto" },
  },
  {
    id: "gs-talk",
    title: "沟通与协调",
    blurb: "和物业沟通，取得安装许可。",
    goal: "在公共空间立东西，绕不开管这片地的人。",
    youBring: "一次真的对话，和对方的原话",
    iBring: "陪你把对方的顾虑和你能给的条件列清楚",
    then: "把条件带回来，我按条件改图纸",
    decide: "他提的条件，哪些你接受",
    opens: { kind: "card", cardId: "talk" },
  },
  {
    id: "gs-ship",
    title: "修订与交付",
    blurb: "按沟通结果修订，完成交付。",
    goal: "把标牌装上去，并写清楚还有什么没做完。",
    youBring: "一段交付说明，包括已知不足",
    iBring: "出最终版图纸",
    then: "交付，并把说明一起交出去",
    decide: "现在交付，还是再改一轮",
    opens: { kind: "card", cardId: "ship" },
  },
];

export const PLANS: Record<string, PlanStep[]> = {
  homepage: HOMEPAGE,
  "garden-map": GARDEN_MAP,
  "garden-signs": GARDEN_SIGNS,
};

/** A fresh copy — plans are edited in place per project. */
export function planById(id: string): PlanStep[] {
  return (PLANS[id] ?? []).map((s) => ({ ...s }));
}

/**
 * The fallback plan for a track with no hand-written journey.
 *
 * Derived from the track's card sequence rather than hand-written, so a track
 * can never show an empty planner — and derived from ONE source, so the plan
 * and the cards can never disagree about what happens next. The copy is
 * generic and says so; a student who wants a real plan gets one by telling
 * 印记 what she is actually making.
 */
export function planForTrack(track: TrackId): PlanStep[] {
  return cardsForTrack(track).map((cid) => {
    const spec = cardById(cid);
    return {
      id: `s-${cid}`,
      title: spec?.title ?? cid,
      blurb: spec?.teaches.split("。")[0] ?? "",
      goal: spec?.reason ?? "",
      youBring: "把这张卡填完",
      iBring: "在你动手之前，先说清楚为什么是这一张",
      then: "填完交回来，我按你写的继续",
      decide: spec?.teaches.split("。")[0] ?? "这一步该怎么走",
      opens: { kind: "card" as const, cardId: cid },
    };
  });
}

/** The steps she has actually kept. Everything that walks a plan uses this —
 *  three separate `filter(s => !s.off)` calls is how a progress bar and a
 *  step counter end up disagreeing on the same screen. */
export function activeSteps(plan: PlanStep[]): PlanStep[] {
  return plan.filter((s) => !s.off);
}

/* ── approaches ───────────────────────────────────────────────────────────
 *
 * 印记 proposes roads. It does not take one.
 *
 * These two are genuinely different KINDS of answer, which is the only way
 * the choice teaches anything: one is a thing on a screen that can be wrong
 * and fixed in a minute, the other is a thing screwed to a post that is right
 * or it is not. A student who picks between them has learned something about
 * the difference; a student picking between two colour schemes has not.
 * ---------------------------------------------------------------------- */

export const GARDEN_APPROACHES: Approach[] = [
  {
    id: "map",
    name: "做一张这个花园自己的地图",
    shape: "屏幕上的东西 · 可以一直改",
    how: [
      "在园子里定若干个编号点，每个点贴一张二维码。",
      "扫码打开一页，第一眼就是「你在 7 号点」，然后才是整张图。",
      "图是你自己走出来画的，包含电子地图上没有的小路。",
    ],
    costs: [
      "二维码会被雨泡烂、被人撕掉，得有人补。",
      "没带手机、手机没电、老人不扫码的人，一点都用不上。",
      "要在公共区域贴东西，必须先过物业那一关。",
    ],
    needs: "你得亲自把整个园子走一遍并画出来。这张图没人能替你画——电子地图上就是没有。",
    hue: "var(--mk-lake)",
    planId: "garden-map",
  },
  {
    id: "signs",
    name: "在园子里加实体的标识和指引",
    shape: "现实里的东西 · 装上去就很难改",
    how: [
      "在容易走错的路口立牌子：这是哪儿、往哪走、还有多远。",
      "不需要手机，不需要电，走过路过的人都能看见。",
      "可以先用纸做三块试，好用了再做正式的。",
    ],
    costs: [
      "做错了就得重做，改一次的成本比改一个网页高得多。",
      "牌子只能告诉人「这里是哪」，没法告诉人「你要去的地方在哪」。",
      "同样要过物业那一关，而且他们对实体的东西通常更谨慎。",
    ],
    needs: "你得先弄清楚人具体是在哪几个路口犹豫的，不然牌子会立在没人迷路的地方。",
    hue: "var(--mk-matcha)",
    planId: "garden-signs",
  },
];

/* ── branch conversations ────────────────────────────────────────────────
 *
 * An approach is a hook. She opens it, digs, and comes back with a sentence.
 *
 * The scripted answers below are the honest ones — several of them say *go
 * ask a real person, my guess is worth less than their answer*, because on
 * this kind of question that is true, and a product that teaches
 * source-checking cannot model an AI that pretends to know the local
 * property manager's mind.
 * ---------------------------------------------------------------------- */

export interface BranchAsk {
  q: string;
  a: string;
}

export interface BranchScript {
  opener: string;
  asks: BranchAsk[];
  /** Said once she has asked at least two things: bring something back. */
  nudge: string;
}

export const BRANCHES: Record<string, BranchScript> = {
  map: {
    opener:
      "好，我们只谈这一条路，不用现在就决定。\n\n先说清楚它是什么：**一张只属于这个园子的图**，加上若干个编号点。人扫一下就知道自己在哪。\n\n你想先问哪一头？",
    asks: [
      {
        q: "要贴多少个点才够？",
        a: "这个数字不该由我拍。它取决于两件事：园子里有多少个「会走错的路口」，以及你能维护多少张码。\n\n经验上，人在看不见上一个标记的时候就开始不安。所以真正的问题是：站在一个点上，还能不能看见下一个点。你去走的时候可以直接量这个。",
      },
      {
        q: "二维码坏了怎么办？",
        a: "会坏。雨、太阳、熊孩子，三个月内你一定会掉几张。\n\n这不是这条路的死穴，但它意味着这个东西需要一个**维护它的人**。如果那个人只能是你，那它的寿命就是你还在这个小区的时间。想清楚这一点比想清楚怎么防水重要。",
      },
      {
        q: "不用手机的人怎么办？",
        a: "用不上。这是这条路真实的边界，我不打算替它圆过去。\n\n值得你去查一件事：在你们园子里迷路的人，主要是哪一群？如果多数是来找朋友的年轻人，这条路覆盖得不错；如果多数是散步的老人，那你可能选错了工具。这个答案在园子里，不在我这儿。",
      },
      {
        q: "我不会写代码，这一页谁来做？",
        a: "我来。这一页的代码、地图的画法、扫码之后跳转到哪一个编号，都是我的活。\n\n你的活是另外三样：图得你去画，点得你去定，做完了得你去试。这三样我做不了——我没去过你们园子。",
      },
      {
        q: "物业会同意吗？",
        a: "我不知道，我也不该猜。我没见过你们物业。\n\n但我可以帮你想清楚他会担心什么：贴的东西好不好撕、会不会有人投诉不好看、出了事算谁的。你带着这三个答案去谈，比空手去强很多。这一步我们后面有一张专门的卡。",
      },
    ],
    nudge: "问得差不多了。现在把你想清楚的那一句写下来——不是「这条路不错」，是你到底看明白了它的什么。",
  },
  signs: {
    opener:
      "好，只谈这一条。\n\n它的形状和上一条很不一样：**东西装在现实里**。不需要手机，也几乎改不了。\n\n你想先问哪一头？",
    asks: [
      {
        q: "牌子上该写什么？",
        a: "三种写法，教给人的东西不一样：\n\n写**编号**——人知道自己在哪，但不知道该往哪走。\n写**地名**——人有了方向感，下次还记得。\n写**箭头 + 距离**——这一次一定找得到路，但下次还是不认识这个园子。\n\n选哪种，取决于你想解决「这一次」还是「以后」。这是你的判断。",
      },
      {
        q: "做错了怎么办？",
        a: "这是这条路最贵的地方：一块装好的牌子，改一次的成本大概是改一个网页的五十倍。\n\n所以顺序反过来——**先用纸做三块贴上去**，让人走一次，看他在哪儿停下来。纸是可以扔的。等纸的那版没问题了，再做正式的。",
      },
      {
        q: "要立几块？",
        a: "不该由我定，而且比上一条路更不该：牌子立错地方就是白花钱。\n\n定这个数只有一个靠谱的办法：你去看人在哪几个路口犹豫。带一个不认识花园的人走一次，记下他每次停住的位置——那就是你的清单。",
      },
      {
        q: "这个我自己能做完吗？",
        a: "比上一条路更能。它不依赖代码，也不依赖谁的手机。\n\n它依赖的是：你能不能拿到许可，以及你有没有办法把东西固定在户外还不歪。第二件事听起来小，实际上是这条路最容易卡住的地方。",
      },
      {
        q: "物业会同意吗？",
        a: "同样：我不知道，我不猜。\n\n不过按常理，实体的东西他们通常更谨慎——因为它更难拆、更容易被投诉、坏了责任更难说清。你可以把「先试三块纸的」当成谈判的筹码：可撤销的方案比不可撤销的好批。",
      },
    ],
    nudge: "问得差不多了。把你想清楚的那一句写下来——你到底看明白了这条路的什么。",
  },
};

/**
 * 印记's answer inside a branch.
 *
 * Keyword-matched against the approach's own question list, then a general
 * fallback. 🚨 The fallback must never invent a confident answer — see the
 * `ai-errors-must-surface-never-fake` rule. Here the honest answer is usually
 * *that one is not mine to know*, which is also the right lesson.
 */
export function branchReply(approachId: string, text: string): string {
  const script = BRANCHES[approachId];
  if (!script) return "这条路我还没想过。你先说说你担心的是什么？";
  const t = text.trim();
  for (const ask of script.asks) {
    if (t === ask.q) return ask.a;
  }
  const hit = script.asks.find((ask) =>
    keyOf(ask.q).some((k) => t.includes(k)),
  );
  if (hit) return hit.a;
  return "这个我答不了——它取决于你们那个园子的具体情况，而我没去过。\n\n与其听我猜，不如把它记下来，去问一个真的在那儿待过的人。他随口一句会比我这段有用得多。";
}

/** The nouns worth matching in a scripted question. Two characters minimum:
 *  single Chinese characters match almost anything and turn the matcher into
 *  a random answer generator. */
function keyOf(q: string): string[] {
  return q
    .replace(/[？?，,。、]/g, " ")
    .split(/\s+/)
    .filter((w) => w.length >= 2);
}
