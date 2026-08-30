/**
 * 印记 — her AI, scripted.
 *
 * There is no model behind this prototype. That creates a specific danger the
 * codebase already has a rule about ([AI errors must surface, never fake]):
 * a canned sentence that PRETENDS to be a real answer makes the student talk
 * into a dead terminal. So this module does two things instead:
 *
 *  1. It answers only what it can actually answer — replies are keyed off the
 *     SURFACE she is on and off keywords it really recognises.
 *  2. When it has nothing scripted, it says so plainly (`FALLBACK`) rather
 *     than improvising a plausible-sounding non-answer.
 *
 * Voice rules ([印记 must talk like a teacher]): name the stakes, use the real
 * method name, give her a choice, offer to show an example. **一次只问一个。**
 */

export type CoachSurface = "world" | "tree" | "projects" | "homepage" | "reading" | "writing" | "step";

export interface CoachMessage {
  id: string;
  role: "coach" | "student";
  text: string;
  /** Rendered as tappable chips under the message. */
  choices?: string[];
}

/** What 印记 opens with, per surface. It always knows where she is — that is
 *  the difference between an assistant and a chatbot in a corner. */
export const OPENERS: Record<CoachSurface, { text: string; choices: string[] }> = {
  world: {
    text: "今天这五颗里，有一颗和你树上的「例外与代表性」是同一件事。要我指给你看，还是你自己先转一圈？",
    choices: ["指给我看", "我自己先看看", "为什么今天是这五条？"],
  },
  tree: {
    text: "你的树上现在有 16 个词。有一个我想跟你说一句：「留白与沉默」是唯二走完「读到→试了→写下来」三步的词。要听听我从这里看见了什么吗？",
    choices: ["说说看", "哪些词快掉下去了？", "我想让哪个词长大"],
  },
  projects: {
    text: "做项目最容易死在第三步，原因几乎都一样：一开始没说清楚为什么做。所以我们会在「为什么」上花掉比你预期更多的时间。你想直接挑一条赛道，还是先聊聊你最近在想什么？",
    choices: ["我直接挑赛道", "先聊聊", "为什么一定要问为什么？"],
  },
  homepage: {
    text: "先说清楚我在这里的位置：主页上的每一句都得是你写的，我不替你写。我能做的是问、给骨架、挑出啰嗦的地方。你想先看看别人的页面吗？",
    choices: ["看别人的页面", "什么叫「清楚的指令」？", "我不知道该写什么"],
  },
  reading: {
    text: "读到哪儿了？如果有一句你读了两遍的，把它贴给我，我们从那句开始。",
    choices: ["有一句我没看懂", "这篇能写成什么"],
  },
  writing: {
    text: "你想我在哪一层帮你——结构（这篇的骨架对不对），还是论证（这一段站不站得住）？一次只看一层比较有用。",
    choices: ["看结构", "看论证", "帮我挑毛病"],
  },
  step: {
    text: "这一步的目的写在标题下面那行。开始之前，先确认一件事：你打算做完它，还是先做一半看看？两个都行，但先说清楚，你后面才不会觉得自己在拖。",
    choices: ["我要做完", "先做一半", "这一步能跳过吗？"],
  },
};

interface Rule {
  match: RegExp;
  reply: string;
  choices?: string[];
}

/** Recognised topics. Ordered — first match wins. */
const RULES: Rule[] = [
  {
    match: /为什么.*(五条|5条|这五)|怎么挑|选出来/,
    reply:
      "五条的标准写在星图右上角那枚说明片里，可以点开看。简单说：它得改变了一个事实（不是「谁说了什么」），你读得懂它为什么重要，而且它能牵出一个问题。政治和冲突类不放在这里——不是因为不重要，是因为半分钟不足以让人对复杂的事下判断。",
    choices: ["那我怎么判断一条新闻重不重要？"],
  },
  {
    match: /政治|不谈|过滤/,
    reply:
      "我们不在这里讨论政治和冲突。理由很实在：讨论它们需要背景、立场和分寸，而一颗行星只有三十秒。我们不想请你在三十秒里对一件复杂的事下判断。这不是说它不重要——是说这里不是那个地方。",
  },
  {
    match: /例外|代表|以偏概全|样本/,
    reply:
      "这正是你树上最强的那个词。它有一个正式名字叫**代表性（representativeness）**：一个例子能不能代表整体，要看三件事——它有多大、它是怎么被挑出来的、它和其他情况有什么不同。你在珊瑚那篇里已经用对了第一件（4 平方公里 vs 34.4 万）。要不要试试第二件？",
    choices: ["第二件是什么意思", "给我看一个例子"],
  },
  {
    match: /让步|反驳|对方|承认/,
    reply:
      "**让步段（concession）**：在反驳之前，先把对方最有力的理由说全，而且承认它是真的。它的作用不是礼貌，是让你后面的「可是」有落点——读者知道你不是没看见那一面。你在珊瑚那篇里写过一次，写得很好。想在新的一篇里再用一次，还是想学下一招（**归谬**）？",
    choices: ["再用一次让步", "归谬是什么"],
  },
  {
    match: /指令|prompt|怎么跟.*(AI|你).*说|说不清/,
    reply:
      "「清楚的指令」不是玄学，是三样东西凑齐：① 一个**直接的例子**（你要它像哪一个）；② **内容的结构**（哪几块、什么顺序）；③ **你想怎么合作**（我提问你答 / 我提骨架你填 / 你写我挑毛病）。缺哪一样，它就会自己瞎猜哪一样。你想先定哪一个？",
    choices: ["先定例子", "先定结构", "先定合作方式"],
  },
  {
    match: /不知道.*写|没什么可写|我很普通|不知道该说/,
    reply:
      "这句我听过很多次，而且几乎每次都不成立。你上个月做了一件事：把家里那盏修不好的台灯拆了、量了、重画了外壳、打印出来装回去。那不是「普通」，那是一整个项目。问题不是你没东西写，是你还没把它当回事。要从那盏灯开始吗？",
    choices: ["从台灯开始", "我还有别的事想说"],
  },
  {
    match: /为什么.*(要问|问为什么)|动机|motivation/,
    reply:
      "因为项目会在第三步左右变难，那时候唯一能让你继续的东西就是「我为什么做这个」。如果那时候你的答案是「老师让的」，你就停了。所以我们现在花二十分钟，换后面不塌。三个问题，一次一个：谁会用它？没人做会怎样？为什么是你？",
    choices: ["开始第一个"],
  },
  {
    match: /树|关键词|词是怎么来的/,
    reply:
      "树上每一个词都必须能指出来源——哪几篇阅读、哪一篇你写的、哪个项目。点开任何一个词，抽屉里第二段就是来源清单，里面还会有你自己写过的原话。如果一个词说不出来源，它就不该在树上。",
    choices: ["哪个词来源最多？", "怎么让一个词长大"],
  },
  {
    match: /长大|变强|怎么让/,
    reply:
      "一个词变强只有一条路：多一个**不同类型**的来源。已经有三篇阅读的词，再读第四篇几乎不动；写一篇，或者做一个项目，它会明显长一截。你树上「手艺与时间」现在只有一个来源——那是最容易动的一个。",
    choices: ["就选它", "我想动别的词"],
  },
  {
    match: /跳过|不想做|太难|算了/,
    reply:
      "可以跳过，跳过也会被记下来——不是为了扣你分，是因为「你在哪里停下」本身是有用的信息。但先告诉我一件事：是这一步太难，还是这个项目本身你已经不想做了？这两个的解决办法完全不一样。",
    choices: ["这一步太难", "这个项目我不想做了"],
  },
  {
    match: /帮我写|你写|直接给我|生成一段/,
    reply:
      "这个我不做。不是规矩问题——是如果这一页是我写的，它就不是你的页面了，别人一眼能看出来。我能做的是三件：问你问题、给你留空格的骨架、挑出你写完之后啰嗦的地方。你想要哪一种？",
    choices: ["问我问题", "给我骨架", "我写完你挑毛病"],
  },
  {
    match: /结构|骨架|怎么排/,
    reply:
      "先定**脊柱**（整篇怎么走）：立场式 / 起承转合 / 钩子式 / 记叙。你上一篇用的是立场式，收在让步段。这一篇如果还是同一个脊柱，你会写得快但学不到新东西。要换一个试试吗？",
    choices: ["换一个", "还是用立场式"],
  },
];

const FALLBACK =
  "这句我没有把握答好，我不想编一个听起来对的答案给你。换个方式说说看——或者告诉我你现在卡在哪一步，我从那里接。";

/** The scripted reply. `surface` is carried so a future real implementation
 *  has the same signature; today it only selects the opener. */
export function coachReply(text: string): { text: string; choices?: string[] } {
  const rule = RULES.find((r) => r.match.test(text));
  if (rule) return { text: rule.reply, choices: rule.choices };
  return { text: FALLBACK };
}

/** 印记's read of a keyword, for the tree drawer's "印记 看见的" line. */
export function coachOnKeyword(note: string): string {
  return note;
}
