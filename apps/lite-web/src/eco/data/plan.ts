import type { Approach, PlanStep, TrackId } from "./types";
import { cardById, cardsForTrack } from "./cards";

/**
 * 计划 — what 印记 proposes before anything is built.
 *
 * ## Why this file exists (2026-08-31)
 * The workbench went straight from "hello" to the first 工具卡. That is not
 * how an agentic tool behaves and it is not how a person with judgement
 * works either: Codex, Cowork and every competent collaborator put a PLAN in
 * front of you first, and wait. The plan is the artefact that lets a student
 * disagree before three weeks are spent, and it is the only place where the
 * division of labour can be argued about instead of assumed.
 *
 * ## The three things a step must name
 *   - **你带来** — what she has to bring. Concrete. Not 「参与讨论」.
 *   - **印记做** — what the AI actually does. In a project it may build:
 *     write the code, draw the layout, generate the options.
 *   - **你判断** — the judgement that stays hers. **Every step has one.** If a
 *     step cannot name a decision she owns, the honest thing is to let 印记 do
 *     it silently rather than seat her in front of it and call it learning.
 *
 * `when` is left EMPTY on purpose. 印记 does not pre-fill the timeline — she
 * writes it during the review, because a schedule you did not write is a
 * schedule you will not keep, and because writing it is the moment a student
 * finds out her plan has nine steps and two free afternoons in it.
 */

/* ── the personal page ────────────────────────────────────────────────────
 * The journey the product proposes first. Note the order: she names the look
 * she wants BEFORE she goes looking at other people's pages. That is
 * deliberate — coming back to three words you wrote an hour ago and finding
 * two of them gone is the cheapest lesson in this whole sequence.
 * ---------------------------------------------------------------------- */
const HOMEPAGE: PlanStep[] = [
  {
    id: "hp-why",
    title: "说清楚这一页是给谁看的",
    blurb: "一个具体的人会打开它。是谁、什么时候、他想找到什么。",
    youBring: "一个具体的人，不是「大家」",
    iBring: "追问到你说得出名字为止",
    decide: "这一页是为谁存在的",
    when: "",
    opens: { kind: "card", cardId: "motive" },
  },
  {
    id: "hp-words",
    title: "先用三个词说出你想要的样子",
    blurb: "在看任何别人的东西之前写下来。看完再回来对一次。",
    youBring: "三个词，和你不想要的那一种",
    iBring: "把词记下来，之后每一步都拿它对照",
    decide: "你到底想要什么感觉",
    when: "",
    opens: { kind: "card", cardId: "keywords" },
  },
  {
    id: "hp-look",
    title: "去看十个真人的主页",
    blurb: "每个只挑一处你真的喜欢的地方，写到「一眼能验证」。",
    youBring: "至少四个参考，和一个反例",
    iBring: "给你一份可以直接打开的名单",
    decide: "哪些是你真的想要的，哪些只是好看",
    when: "",
    opens: { kind: "card", cardId: "sweep" },
  },
  {
    id: "hp-style",
    title: "印记给三个方案，你选一个并说清为什么",
    blurb: "三个方案是三种不同的做法，不是三种配色。挑一个，写下理由。",
    youBring: "一个选择 + 你选它的理由",
    iBring: "按你的三个词和四个参考，做三版真能看的方案",
    decide: "哪一版，以及为什么不是另外两版",
    when: "",
    opens: { kind: "make", artifactId: "hp-style" },
  },
  {
    id: "hp-content",
    title: "定这一页上放什么",
    blurb: "印记从你读过、写过、做过的东西里拟一版。你删、你改、你加。",
    youBring: "删掉不该在的，补上只有你知道的",
    iBring: "从你自己的库里草拟每一块的内容",
    decide: "别人看完这一页，该记住你哪一点",
    when: "",
    opens: { kind: "make", artifactId: "hp-content" },
  },
  {
    id: "hp-build",
    title: "印记开工，你验收",
    blurb: "这一步印记要做很久。做完你看，然后给具体的反馈，改到你认可为止。",
    youBring: "具体的反馈——「第二屏的字太小」，不是「感觉怪怪的」",
    iBring: "写代码、排版、出图，一轮一轮改",
    decide: "什么程度算做完了",
    when: "",
    opens: { kind: "make", artifactId: "hp-build" },
  },
  {
    id: "hp-ship",
    title: "发布，并说清楚你放弃了什么",
    blurb: "把网址发出去之前，先写下这一版里你知道还不够好的地方。",
    youBring: "一段诚实的说明",
    iBring: "把它挂到你的主页地址上",
    decide: "现在发，还是再改一轮",
    when: "",
    opens: { kind: "card", cardId: "ship" },
  },
];

/* ── the community garden, road A: a map with numbered points ──────────── */
const GARDEN_MAP: PlanStep[] = [
  {
    id: "gd-recon",
    title: "走一遍花园，把它画下来",
    blurb: "带纸和手机去。走完所有的岔路，记下你自己在哪里犹豫了。",
    youBring: "一张你自己画的图，和你迷路的那几个位置",
    iBring: "给你一张踏勘清单，回来帮你把手画的图整理成能用的底图",
    decide: "哪些岔路是真的会让人走错的",
    when: "",
    opens: { kind: "card", cardId: "recon" },
  },
  {
    id: "gd-points",
    title: "定几个点，定在哪儿",
    blurb: "点太少没用，点太多没人贴。印记给三种密度，你挑一种。",
    youBring: "一个数字，和你为什么选它",
    iBring: "按你的图算三种方案，每种都写清楚代价",
    decide: "在「够用」和「做得完」之间，你站哪儿",
    when: "",
    opens: { kind: "make", artifactId: "gd-points" },
  },
  {
    id: "gd-build",
    title: "印记做这个网页",
    blurb: "扫码就打开，一眼看到「你在这里」。这一步印记做，你验收。",
    youBring: "在手机上真的试一次，然后说哪里不对",
    iBring: "写这个页面：地图、定位、每个点的编号",
    decide: "老人和小孩能不能一眼看懂",
    when: "",
    opens: { kind: "make", artifactId: "gd-build" },
  },
  {
    id: "gd-test",
    title: "印几张二维码，去现场试",
    blurb: "先只做三个点。找一个不认识花园的人，让他从 A 走到 B。",
    youBring: "一个真的会迷路的人，和一次真的测试",
    iBring: "帮你把「什么算失败」在测试前定下来",
    decide: "这次测试算成功还是失败",
    when: "",
    opens: { kind: "card", cardId: "proto" },
  },
  {
    id: "gd-talk",
    title: "去找物业谈一次",
    blurb: "东西再好，贴不上去就等于没做。这一步只有你能做。",
    youBring: "一次真的对话，和对方真的说了什么",
    iBring: "陪你想清楚他会担心什么、你能给他什么",
    decide: "他提的条件，哪些你接受",
    when: "",
    opens: { kind: "card", cardId: "talk" },
  },
  {
    id: "gd-ship",
    title: "改完，交付",
    blurb: "按物业的条件改一版，贴上去，写清楚还有什么没做完。",
    youBring: "一段诚实的说明",
    iBring: "出最终版的页面和二维码",
    decide: "现在交，还是再改一轮",
    when: "",
    opens: { kind: "card", cardId: "ship" },
  },
];

/* ── the community garden, road B: physical signage ────────────────────── */
const GARDEN_SIGNS: PlanStep[] = [
  {
    id: "gs-recon",
    title: "走一遍花园，把它画下来",
    blurb: "标牌立在哪里，取决于人在哪里犹豫。所以先去看人在哪里犹豫。",
    youBring: "一张你自己画的图，和你迷路的那几个位置",
    iBring: "给你一张踏勘清单，回来帮你整理",
    decide: "哪几个路口是真的需要标牌的",
    when: "",
    opens: { kind: "card", cardId: "recon" },
  },
  {
    id: "gs-sign",
    title: "定标牌怎么标",
    blurb: "编号？地名？还是箭头加距离？三种标法教会人的东西不一样。",
    youBring: "一个选择 + 理由",
    iBring: "把三种标法各做一版给你看",
    decide: "你要人记住位置，还是只要他这一次找到路",
    when: "",
    opens: { kind: "make", artifactId: "gs-signs" },
  },
  {
    id: "gs-test",
    title: "做三块，贴上去试",
    blurb: "先做三块纸的。找一个不认识花园的人走一次。",
    youBring: "一个真的会迷路的人，和一次真的测试",
    iBring: "帮你在测试前定下「什么算失败」",
    decide: "这次测试算成功还是失败",
    when: "",
    opens: { kind: "card", cardId: "proto" },
  },
  {
    id: "gs-talk",
    title: "去找物业谈一次",
    blurb: "在公共空间立东西，绕不开管这片地的人。",
    youBring: "一次真的对话",
    iBring: "陪你想清楚他会担心什么",
    decide: "他提的条件，哪些你接受",
    when: "",
    opens: { kind: "card", cardId: "talk" },
  },
  {
    id: "gs-ship",
    title: "做完，交付",
    blurb: "按条件改一版，装上去，写清楚还有什么没做完。",
    youBring: "一段诚实的说明",
    iBring: "出最终版的图纸",
    decide: "现在交，还是再改一轮",
    when: "",
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
      blurb: spec?.reason ?? "",
      youBring: "把这张卡填完",
      iBring: "在你填之前先说清楚为什么是这一张",
      decide: spec?.teaches.split("。")[0] ?? "这一步该怎么走",
      when: "",
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
