/**
 * 继续深挖 — what a keyword is FOR.
 *
 * ## Why this file exists
 * The keyword drawer used to end with four generic buttons: 再读一篇 / 写一篇 /
 * 做个项目 / 问印记. They were the same four on every keyword, so they said
 * nothing — a student reading them learns that the model has an opinion about
 * her ("你关心修理权") and no idea what to do about it. Four empty verbs after
 * a real observation is worse than no ending at all.
 *
 * So every keyword now carries FOUR CONCRETE SEEDS, written for that keyword:
 * a question she cannot answer yet, a next thing to read, a piece she could
 * write, and a project that could actually exist. They render as bubbles;
 * clicking one carries its text into 印记, the reading room, or a new project
 * with the seed already in the box.
 *
 * 🚨 INVARIANT: every keyword in `data/tree.ts` must have an entry here. A
 * keyword that falls back to `GENERIC` is a keyword whose drawer has nothing
 * to say — see `digFor`. When you add a keyword, add its four seeds in the
 * same commit.
 */

export type DigKind = "question" | "reading" | "writing" | "project";

export interface DigSeed {
  kind: DigKind;
  /** The bubble's text. A question, a title, a proposal — never a verb. */
  text: string;
  /** For `reading`: an id in `READINGS`, when we have the real article. */
  ref?: string;
  /** Shown under the bubble once it is opened — why this one, for her. */
  why?: string;
}

export const DIG_META: Record<DigKind, { label: string; hue: string; glyph: string }> = {
  question: { label: "想一想", hue: "var(--mk-accent-400)", glyph: "?" },
  reading: { label: "去读", hue: "var(--mk-lake)", glyph: "▤" },
  writing: { label: "去写", hue: "var(--mk-peach)", glyph: "✎" },
  project: { label: "去做", hue: "var(--mk-taro)", glyph: "◇" },
};

export const DIG: Record<string, DigSeed[]> = {
  "k-representative": [
    {
      kind: "question",
      text: "要多少个例子，一个说法才算「有代表性」？这个数字是谁定的？",
      why: "你已经会问「能代表多少」了。下一步是问「多少才够」——这一步没有标准答案，正好适合你练。",
    },
    {
      kind: "reading",
      text: "《红海北端那片不白化的珊瑚》再读一次，这次只找它的样本量",
      ref: "r-coral",
      why: "同一篇读第二遍、只盯一个东西，是你还没试过的读法。",
    },
    {
      kind: "writing",
      text: "写一篇《我在朋友圈里看到的世界有多大》——统计你一周刷到的内容来自几个人",
      why: "把代表性这件事用在你自己身上，比用在珊瑚上更难，也更有意思。",
    },
    {
      kind: "project",
      text: "做一份班级问卷，故意设计一个「样本有偏」的版本和一个没偏的，比较结果",
      why: "这是能真实做出来的：两份问卷、一个班、一张对比图。",
    },
  ],
  "k-climate": [
    {
      kind: "question",
      text: "海洋少吸 7% 的碳，谁最先要改计划？改什么？",
      why: "你读了三篇气候，但还没为任何一篇找出「所以谁要动」。",
    },
    {
      kind: "reading",
      text: "找一篇写「碳汇是怎么测出来的」的文章，补上方法这一环",
      why: "你的三个来源都在讲结论，没有一个讲测量。",
    },
    {
      kind: "writing",
      text: "写《一个数字被改了 7%，然后呢》——追一个修正数字的后果",
      why: "你擅长追后果，这次的对象是一个数字。",
    },
    {
      kind: "project",
      text: "做一张「我家一周的碳」记录表，自己定测量方法，并写清楚它哪里不准",
      why: "重点不是数字准，是你要写出它为什么不准——那才是这条线上的真本事。",
    },
  ],
  "k-memory": [
    {
      kind: "question",
      text: "小鼠的海马体和你的一样吗？从小鼠到人，这一步跨得有多大？",
      why: "你读完就改了作息，但没有先问这一句。",
    },
    {
      kind: "reading",
      text: "《睡觉时大脑在重放什么》",
      ref: "r-sleep",
      why: "回去看看你当时划的那句。",
    },
    {
      kind: "writing",
      text: "写《我改了作息的第十四天》——记录你自己的实验，包括失败的几天",
      why: "你已经在做这个实验了，只差把它写下来。",
    },
    {
      kind: "project",
      text: "做一个两周的「睡眠 × 记忆」自我实验：定指标、记数据、写一份诚实的结论",
      why: "样本量 1。你要在报告里说清楚这意味着什么。",
    },
  ],
  "k-ordinary-history": [
    {
      kind: "question",
      text: "谁决定什么值得存档？如果没人存，一件事算发生过吗？",
      why: "这是你回来过三次的地方。它够大，可以当你的长期问题。",
    },
    {
      kind: "reading",
      text: "《十万封普通人的家书》",
      ref: "r-letters",
      why: "你写过它，但只用了一半——最常出现的词那一段你还没用。",
    },
    {
      kind: "writing",
      text: "写《我奶奶的一天》——用一次真实的采访，不用回忆",
      why: "你的普通人历史一直在读别人的档案。这次自己造一份。",
    },
    {
      kind: "project",
      text: "做一个小型口述史：采访三位长辈，各问同一个问题，做成一页对照",
      why: "三个人、同一个问题、一页纸。做得完，也做得好。",
    },
  ],
  "k-hook": [
    {
      kind: "question",
      text: "钩子用第二次还灵吗？读者什么时候会发现你在用招式？",
      why: "你已经用顺了。用顺之后的下一个坎就是这个。",
    },
    {
      kind: "reading",
      text: "找三篇你喜欢的文章，只抄它们的第一句，看看有几种开头",
      why: "抄开头是最快的文体训练，你还没做过。",
    },
    {
      kind: "writing",
      text: "给同一篇旧文写三个不同的开头：数字式、场景式、反问式",
      why: "同一篇写三次开头，比写三篇新文章学得多。",
    },
    {
      kind: "project",
      text: "做一本《开头图鉴》：收集 20 个开头，标注每个用的是什么招",
      why: "这是能发布出去、别人真的会用的东西。",
    },
  ],
  "k-concession": [
    {
      kind: "question",
      text: "让步和退让差在哪？什么时候让步会让你的论点更弱？",
      why: "你会写让步段了，但还没遇到过「让过头」的那次。",
    },
    {
      kind: "reading",
      text: "找一篇你不同意的评论文章，把它的道理写成一段你自己的让步",
      why: "为对手写最强版本，是最硬的一种阅读。",
    },
    {
      kind: "writing",
      text: "写一篇《我改变主意的一次》——把自己当成被说服的那一方",
      why: "你写过别人的道理，还没写过自己认输。",
    },
    {
      kind: "project",
      text: "组织一次班级小辩论，规则是每个人必须先说对方最强的一条",
      why: "把让步段变成一个规则，看它在真人身上会怎样。",
    },
  ],
  "k-scale": [
    {
      kind: "question",
      text: "多大才算大？你怎么给一个数字找参照物？",
      why: "你已经会比了，还没总结出自己的比法。",
    },
    {
      kind: "reading",
      text: "找一篇用了很多数字的报道，把每个数字都换算成你熟悉的东西",
      why: "换算是规模感的练习方式，做一次就会。",
    },
    {
      kind: "writing",
      text: "写《一个让人误会的数字》——挑一条你见过的、技术上没错但会骗人的统计",
      why: "你的例外与代表性和规模感在这里会撞上。",
    },
    {
      kind: "project",
      text: "做一组「尺度对照图」：把五个新闻里的数字都画在同一张纸上",
      why: "一张图，五个数字，同一个比例尺。它会比五段文字更有说服力。",
    },
  ],
  "k-who-decides": [
    {
      kind: "question",
      text: "一件东西被做成现在这样，是谁在什么时候决定的？你能追到吗？",
      why: "从一盏台灯开始的问题。追一次到底，它就变成方法了。",
    },
    {
      kind: "reading",
      text: "《把螺丝钉重新设计一遍》",
      ref: "r-repair",
      why: "这篇给了你一个反例：有人反过来决定了。",
    },
    {
      kind: "writing",
      text: "写《我没被问过的三件事》——找三个你生活里没得选的设计",
      why: "你的问题一直在别人的物件上。这次找自己的。",
    },
    {
      kind: "project",
      text: "拆解一件家里的东西，画出它的决策链：谁选了材料、谁选了接口、谁选了寿命",
      why: "一件真东西、一张图。这是你能拿给别人看的证据。",
    },
  ],
  "k-repair": [
    {
      kind: "question",
      text: "「能修」应该是权利，还是卖点？把它写进法律会有什么副作用？",
      why: "你已经走到公共议题了，下一步是走到取舍。",
    },
    {
      kind: "reading",
      text: "找一篇反对修理权立法的文章，读它最强的一条理由",
      why: "你目前的三个来源都站在同一边。",
    },
    {
      kind: "writing",
      text: "给你那篇《被设计成修不好的东西》补一个让步段",
      why: "你会写让步段，那篇却没有——补上它，文章会硬一倍。",
    },
    {
      kind: "project",
      text: "在班里办一次「修理日」：收集坏掉的小东西，记录哪些修得好、哪些修不了、为什么",
      why: "这是你最有可能真的做成的项目：材料现成，结论只能从现场来。",
    },
  ],
  "k-design-ethics": [
    {
      kind: "question",
      text: "为最少数人设计，为什么常常对所有人更好？这条规律有反例吗？",
      why: "你看到了硬币的两面。找反例是第三步。",
    },
    {
      kind: "reading",
      text: "《为盲人设计的手势》",
      ref: "r-gesture",
      why: "回去看它对「溢出效应」的解释到底解释了什么。",
    },
    {
      kind: "writing",
      text: "写《学校里最该重新设计的一件事》，并说清你为谁设计",
      why: "「为谁」这一问，是你这条线上最锋利的地方。",
    },
    {
      kind: "project",
      text: "重新设计学校里一个真实的界面（选课表、公告栏、饮水机标识），做出来给人用",
      why: "做出来、被人用、被吐槽、再改——这一圈走完才算学会。",
    },
  ],
  "k-make-it": [
    {
      kind: "question",
      text: "你手上哪个想法是「明天就能做出最小一版」的？最小一版长什么样？",
      why: "这个词刚冒头，它需要的是一个能在一周内做完的东西。",
    },
    {
      kind: "reading",
      text: "找一篇「某人做了个小东西」的自述，只看他第一版有多丑",
      why: "看别人的第一版，是治「不敢开始」最有效的药。",
    },
    {
      kind: "writing",
      text: "写一页《我要做什么、为什么是我、做不出来会怎样》",
      why: "一页纸。写完你就知道这个想法是不是真的。",
    },
    {
      kind: "project",
      text: "开一个项目，要求只有一条：一周内有一个能给别人看的版本",
      why: "你的树上这个词只有一个来源。它需要的是一次真正的发布。",
    },
  ],
  "k-restraint": [
    {
      kind: "question",
      text: "你怎么知道该停手了？停手和放弃，从外面看是一样的吗？",
      why: "你删过两段。那次你是怎么决定的？",
    },
    {
      kind: "reading",
      text: "《修了十一年，只补了 3%》",
      ref: "r-restore",
      why: "「全部可逆」那句你还没用过。",
    },
    {
      kind: "writing",
      text: "写《我删掉的那两段》——把删掉的贴出来，说你为什么删",
      why: "删稿本身就是内容。你已经有素材了。",
    },
    {
      kind: "project",
      text: "做一个《改稿现场》：把一篇文章的三个版本并排展示，标出每次改了什么",
      why: "别人只看得到成品。你可以做那个把过程摊开的人。",
    },
  ],
  "k-craft": [
    {
      kind: "question",
      text: "花十一年做一件事，值不值？这个「值」是谁在算？",
      why: "只有一个来源，所以先别急着做项目——先把问题问清楚。",
    },
    {
      kind: "reading",
      text: "再读一篇同方向的：任何一个「慢手艺」的报道都行",
      why: "你的树上说得很直白：再读一篇，这个词就长起来了。",
    },
    {
      kind: "writing",
      text: "写《我做过最慢的一件事》",
      why: "先给这个词一个属于你自己的来源。",
    },
    {
      kind: "project",
      text: "记录你做某件事的完整用时一个月，做成一张用时分布图",
      why: "手艺与时间这条线，需要你自己的时间数据才站得住。",
    },
  ],
  "k-silence": [
    {
      kind: "question",
      text: "你留给别人的沉默有多长？为什么这么难留？",
      why: "你读到→试了→写下来，三步走完了。第四步是问为什么难。",
    },
    {
      kind: "reading",
      text: "《老师多等三秒会发生什么》",
      ref: "r-wait",
      why: "回去看它是一次复现研究——这一点你当时没提。",
    },
    {
      kind: "writing",
      text: "给《我决定在小组讨论里闭嘴三秒》写一个两周后的续篇",
      why: "第一篇写的是决定。续篇写的是它到底成不成立。",
    },
    {
      kind: "project",
      text: "在一次真实的小组讨论里当计时员，记录每个人的等待时间，做成一份匿名报告",
      why: "这是你能在一节课里做完、而且结果一定出人意料的项目。",
    },
  ],
  "k-honesty": [
    {
      kind: "question",
      text: "「能说真话的人」是找来的还是养出来的？你为别人当过这个人吗？",
      why: "你写了一句只给自己看的话。这个问题是它的下一句。",
    },
    {
      kind: "reading",
      text: "《青少年说孤独的时候》",
      ref: "r-lonely",
      why: "它把孤独重新定义了一次，你还没写出那个定义。",
    },
    {
      kind: "writing",
      text: "写《一个我可以说真话的人》——可以不发表",
      why: "有些文章的读者只有一个。这不影响它是不是好文章。",
    },
    {
      kind: "project",
      text: "做一份匿名的班级问卷：不问「你孤独吗」，问「你上一次说真话是什么时候」",
      why: "怎么问，比问什么更难。这个项目真正的难点在问卷设计。",
    },
  ],
  "k-self-watch": [
    {
      kind: "question",
      text: "你写的三篇复盘里，有没有一次结论是「我错了」？",
      why: "复盘最容易变成自我表扬。检查一下你的三篇。",
    },
    {
      kind: "reading",
      text: "找一篇公开的失败复盘（产品、实验、比赛都行），看它敢写到什么程度",
      why: "看别人写失败写到哪一层，你才知道自己停在了哪一层。",
    },
    {
      kind: "writing",
      text: "写一篇《这次没成》，规定自己不许在结尾写「但是我学到了」",
      why: "去掉那个转折，是这篇文章唯一的难点。",
    },
    {
      kind: "project",
      text: "把你的三篇复盘做成一页「我的模式」：找出重复出现的那一个毛病",
      why: "三篇复盘里一定有同一个毛病出现了两次。找到它。",
    },
  ],
};

/** Four honest fallbacks. Reached only when a keyword has no entry above —
 *  which is a content bug, not a state the student should be in. They still
 *  have to be真话: nothing here pretends to know something about her. */
const GENERIC: DigSeed[] = [
  { kind: "question", text: "这个词对你来说，是一个兴趣，还是一个还没解决的问题？" },
  { kind: "reading", text: "去阅读室找一篇和它相关的，让它多一个来源" },
  { kind: "writing", text: "写三百字说说你为什么会反复回到这里" },
  { kind: "project", text: "开一个项目，先只写清楚「为谁做」" },
];

export function digFor(keywordId: string): DigSeed[] {
  return DIG[keywordId] ?? GENERIC;
}
