import type { CardRow, CardSpec, CardValue, Project, TrackId } from "./types";

/**
 * 工具卡库 — the instruments 印记 can summon inside a project.
 *
 * ## The one architectural rule
 * **A new card is a new spec object here. It is never new renderer code.**
 * `projects/CardSurface.tsx` renders any spec built from the five field
 * primitives (`text` / `textarea` / `choice` / `multi` / `rows`). If a card
 * cannot be expressed with those, that is the moment to consider a sixth
 * primitive — not the moment to special-case a card. This is the same
 * schema-driven contract the real product runs on (AGENTS.md), tried out at
 * prototype scale so we find out early whether the primitives are enough.
 *
 * ## Why these cards and not others
 * They are the moves a person with real judgement actually makes, in the
 * order they make them:
 *
 *   1. Before wanting anything — **look at a lot of things and name what you
 *      like.** (`sweep`) You cannot ask for what you cannot describe, and
 *      "make it nice" is how you get the average of the internet.
 *   2. Turn what you liked into a **spec the AI has to obey** (`style`), which
 *      is the difference between directing and hoping.
 *   3. Find out **what you actually have** before deciding structure
 *      (`inventory`).
 *   4. **Instruct precisely** (`command`) — a real example, the structure, and
 *      how you want to work together.
 *   5. Make the AI produce **options, not an answer** (`variants`), so the
 *      judgement stays with you.
 *   6. Build the **smallest real version** and put it in front of someone
 *      (`proto`).
 *   7. **Critique and revise** (`critique`), then say what you gave up
 *      (`tradeoff`), then ship honestly (`ship`).
 *
 * The research-side cards (`questions`, `sources`, `clinic`, `sample`) exist
 * because "teach students to search and investigate" is a stated goal, and
 * searching is a skill with named failure modes, not a vibe.
 */

export const CARDS: Record<string, CardSpec> = {
  /* ── 开局 ─────────────────────────────────────────────────────────── */
  frame: {
    id: "frame",
    title: "问题界定",
    kind: "plan",
    surface: "chat",
    reason: "你带来的是一个真问题。在我提任何办法之前，先把它问准——否则我们会花三周解决一个不存在的问题。",
    teaches:
      "一个问题被描述得越具体，能走的路就越多。「大家常迷路」只能得到一个笼统的答案；「周末下午来找人的客人，总在东三门那个岔路口停下来」能直接告诉你该把东西放在哪儿。",
    payoff:
      "你写的这四栏我马上就要用：下一屏我给的那几条路，是照着「谁碰到、在哪儿、多久一次」推出来的。这四栏含糊，我给的路就会含糊。",
    glyph: "◉",
    hue: "var(--mk-accent-400)",
    minutes: 15,
    fields: [
      {
        id: "who",
        kind: "textarea",
        label: "谁碰到了这个问题？",
        hint: "一类具体的人，不是「大家」。你见过的那几个最好。",
        placeholder: "例如：来我们小区找人的客人，和刚搬进来不到一个月的住户。",
      },
      {
        id: "when",
        kind: "textarea",
        label: "它具体在什么时候、什么地方发生？",
        hint: "时间、地点、频率。具体到你能画出来。",
        placeholder: "例如：周末下午最多。东三门进来第二个岔路口，树很高，两边看上去一模一样。",
      },
      {
        id: "now",
        kind: "textarea",
        label: "现在人们怎么应付？",
        hint: "已经存在的土办法。它们为什么不够用，就是你的入口。",
        placeholder: "例如：打电话，然后在电话里描述自己旁边的树。对方也听不懂。",
      },
      {
        id: "seen",
        kind: "textarea",
        label: "你自己亲眼看到过几次？",
        hint: "这一栏是在问证据。一次也行，但得是真的。",
        placeholder: "例如：上个月有两次。一次是快递员，一次是我同学的妈妈。",
      },
    ],
  },

  keywords: {
    id: "keywords",
    title: "关键词定义",
    kind: "frame",
    surface: "chat",
    reason: "在你去看别人的东西之前，先把你现在想要的样子写下来。看完十个再回来对一次。",
    teaches:
      "这三个词不是拿来用的，是拿来对照的。看完一圈回来，发现它们剩下几个——那才是你真的想要的。只看不写的人，看完一圈只会拿到最后一个的模仿版。",
    payoff:
      "这三个词会被钉在这个项目上。我做的每一版你都可以拿它们对照，说「这不是我要的」——没有这三个词，你只能说「感觉不对」，而那句话我改不了。",
    glyph: "✲",
    hue: "var(--mk-butter)",
    minutes: 8,
    fields: [
      {
        id: "words",
        kind: "text",
        label: "三个词",
        hint: "用逗号分开。可以是感觉（安静、锐利），也可以是东西（旧书、实验室）。",
        placeholder: "安静，密，像一份手写的笔记",
      },
      {
        id: "not",
        kind: "textarea",
        label: "一个你明确不想要的样子",
        hint: "写完它，你就有了一条底线。",
        placeholder: "例如：那种一进去就有个大头像和一句英文格言的。",
      },
      {
        id: "after",
        kind: "textarea",
        label: "（看完十个再回来填）这三个词还剩几个？",
        hint: "没变也行，写「都在」。变了就写新的。",
        placeholder: "例如：「安静」还在，「密」换成了「留白很多」。",
        optional: true,
      },
    ],
  },

  recon: {
    id: "recon",
    title: "实地调研",
    kind: "research",
    surface: "panel",
    reason: "这一步只能你去。我没去过你们那个地方，网上也没有它的图。",
    teaches:
      "现场会告诉你两件坐在屋里想不出来的事：人具体在哪几个位置犹豫，以及站在一个点上能不能看见下一个点。这两个事实会直接决定后面所有的设计。",
    payoff:
      "你标的迷路点会直接变成图上的位置。这一步只有你能做：我没去过你们那个地方，网上也没有它的图。你少标一个路口，那儿以后就没有东西。",
    glyph: "⌖",
    hue: "var(--mk-lake)",
    minutes: 45,
    fields: [
      {
        id: "paths",
        kind: "rows",
        label: "你走了哪几条路",
        rowsLabel: "路段",
        min: 3,
        hint: "每条路写它的两头，以及走的时候看见什么。",
        columns: [
          { id: "from", label: "从哪里到哪里", placeholder: "东三门 → 中心亭" },
          { id: "mark", label: "路上能认出来的东西", placeholder: "具体到能写在图上", wide: true },
        ],
      },
      {
        id: "lost",
        kind: "rows",
        label: "你（或别人）在哪里犹豫了",
        rowsLabel: "迷路点",
        min: 2,
        hint: "这一栏是整张卡最值钱的。你停下来的位置，就是东西该放的位置。",
        columns: [
          { id: "where", label: "在哪儿", placeholder: "第二个岔路口" },
          { id: "why", label: "为什么在这儿犹豫", placeholder: "两边看上去一模一样", wide: true },
        ],
      },
      {
        id: "sight",
        kind: "choice",
        label: "站在一个岔路口，能看见下一个吗？",
        hint: "这一题决定你需要多少个点。",
        options: [
          { id: "yes", label: "基本能", blurb: "那点可以少一些" },
          { id: "some", label: "有几段不能", blurb: "那几段就是重点" },
          { id: "no", label: "基本看不见", blurb: "那你需要的不只是标记" },
        ],
      },
      {
        id: "drawn",
        kind: "textarea",
        label: "你画的那张图，描述一下",
        hint: "手画的就行。把它拍下来，在这里写清楚它长什么样。",
        placeholder: "例如：一个横着的长方形，三个门在上边，中间一个圆形的中心亭，六条小路从亭子散出去。",
      },
    ],
  },

  talk: {
    id: "talk",
    title: "沟通提纲",
    kind: "plan",
    surface: "chat",
    reason: "东西做得再好，放不上去就等于没做。这一步我替不了你，但我能陪你想清楚再去。",
    teaches:
      "谈判不是把你的方案说一遍。先想对方在担心什么，再想你能给他什么——这两步想完再开口的人，拿到的同意率完全不一样。可撤销的方案（先试三个）比不可撤销的好批。",
    payoff:
      "他答应的条件会直接改掉我做的最后一版——贴在哪、贴多大、写不写「住户制作」。你把原话带回来，我才知道该改什么。",
    glyph: "◑",
    hue: "var(--mk-peach)",
    minutes: 30,
    fields: [
      {
        id: "who",
        kind: "text",
        label: "你要找谁",
        hint: "一个具体的人或一个具体的职位。",
        placeholder: "例如：物业服务中心前台的张阿姨，或者直接找物业经理",
      },
      {
        id: "fear",
        kind: "textarea",
        label: "他最可能担心什么？",
        hint: "至少写两条。想不出来，说明你还没把他当人看。",
        placeholder: "例如：贴上去撕不下来；有人投诉不好看；出了事算谁的。",
      },
      {
        id: "offer",
        kind: "textarea",
        label: "你能给他什么？",
        hint: "一个可撤销的小方案，通常比一个完美的大方案好批。",
        placeholder: "例如：先只贴三个点，用可撕的胶，一个月后我自己来揭。",
      },
      {
        id: "said",
        kind: "textarea",
        label: "（谈完再填）他真的说了什么？",
        hint: "原话，不是你的总结。拒绝也写下来——拒绝的理由里有下一版的答案。",
        placeholder: "例如：「贴可以，但不能贴在墙上，只能放在树牌上。而且得让我先看一眼。」",
        optional: true,
      },
    ],
  },

  /* ── 开局 ─────────────────────────────────────────────────────────────── */
  /**
   * 人物卡 — the tool that justifies the panel.
   *
   * Everything on it could have been asked as five questions in chat, and it
   * would have been a worse tool. Assembling ONE PERSON — a face, a name, an
   * age, what they do, what they are looking for — produces something a
   * student can point at and argue with later ("would 王阿姨 actually read
   * this?"). Five answers scattered up a conversation produce nothing to point
   * at.
   *
   * That is the test for `surface: "panel"`: does the finished thing become an
   * OBJECT she refers back to, or just answers she gave once?
   */
  persona: {
    id: "persona",
    title: "受众画像",
    kind: "plan",
    surface: "panel",
    reason: "在决定页面上放什么之前，我们先把「读者」变成一个具体的人。",
    teaches:
      "为「所有人」做的东西，最后谁都不合用。给一个具体的人做，你会发现很多决定突然有了答案——字要多大、话要多正式、哪一段可以删。",
    payoff:
      "这张卡做完会一直挂在项目上。后面每一版我做出来，你都可以拿他来判断：他会不会看懂、他会不会往下滚、他会不会记住。",
    glyph: "☺",
    hue: "var(--mk-accent-400)",
    minutes: 10,
    fields: [
      {
        id: "face",
        kind: "choice",
        label: "他长什么样",
        hint: "随便挑一个。有张脸之后，后面几栏会好写很多。",
        options: [
          { id: "a", label: "🧑‍🏫" },
          { id: "b", label: "👩‍💼" },
          { id: "c", label: "🧓" },
          { id: "d", label: "🧑‍🎓" },
          { id: "e", label: "👩‍🔬" },
          { id: "f", label: "🧑‍🍳" },
        ],
      },
      {
        id: "name",
        kind: "text",
        label: "他叫什么",
        hint: "真名、化名都行。有名字的人比「用户」好想象。",
        placeholder: "例如：陈老师",
      },
      {
        id: "who",
        kind: "text",
        label: "他是谁，多大年纪",
        placeholder: "例如：45 岁，我妈同事，在一所国际学校做招生",
      },
      {
        id: "when",
        kind: "textarea",
        label: "他会在什么时候打开这一页",
        hint: "时间、地点、他当时在干什么。越具体越好。",
        placeholder: "例如：晚上十点，在手机上，一边看一边还在回别的消息。",
      },
      {
        id: "want",
        kind: "textarea",
        label: "他打开是想找到什么",
        placeholder: "例如：想知道这个学生除了成绩还有什么。他只会看三十秒。",
      },
      {
        id: "feel",
        kind: "textarea",
        label: "你希望他看完是什么感受",
        hint: "一句话。这句会变成我判断每一版做得对不对的标准。",
        placeholder: "例如：这个人是真的在自己想事情，不是在交作业。",
      },
    ],
  },

  motive: {
    id: "motive",
    title: "立项分析",
    kind: "plan",
    surface: "chat",
    reason: "在排任何计划之前，我想先弄清楚这件事为什么值得你花时间。",
    teaches: "一个你说不出「为谁、为什么」的项目，通常死在第三周。先把它说出来，后面每次想放弃的时候可以回来看。",
    payoff:
      "这三句会一直钉在这个项目的最上面。到第三周你想放弃的时候，你会回来读它；我提每一个建议之前也会先看它一眼。",
    glyph: "◉",
    hue: "var(--mk-accent-400)",
    minutes: 12,
    fields: [
      {
        id: "who",
        kind: "textarea",
        label: "这个东西做出来，谁会用它？",
        hint: "说一个具体的人，不是「大家」。一个名字最好。",
        placeholder: "例如：我爷爷。他 78 岁，看不清小字，每天要吃四种药。",
      },
      {
        id: "cost",
        kind: "textarea",
        label: "如果没人做这件事，会怎样？",
        hint: "写下真实的代价。想不出代价，说明这个项目还没找到。",
        placeholder: "例如：他上周吃错了一次。我妈现在每天要打两个电话确认。",
      },
      {
        id: "mine",
        kind: "textarea",
        label: "别人也能做这件事。为什么是你？",
        hint: "你身上的什么，让你比别人更适合做这个。可以很小。",
        placeholder: "例如：只有我知道他其实看得清红色，看不清蓝色。",
      },
    ],
  },

  /* ── 看世界 ───────────────────────────────────────────────────────────── */
  sweep: {
    id: "sweep",
    title: "案例调研",
    kind: "research",
    surface: "panel",
    reason: "你说要做得「好看一点」。我们先把「好看」拆成你说得出口的东西，不然我做出来的会是互联网的平均值。",
    teaches: "你不能想要一个你说不出名字的东西。看很多个，每个只挑一处你真的喜欢的地方，写具体——「排版很好」不算，「标题比正文大四倍，正文一行只有 60 个字符」才算。",
    payoff:
      "你写的那几条「具体喜欢哪一点」，是我等下拉方案时唯一的依据。写「排版很好」我只能猜；写「正文一行 60 个字符」我能直接做出来给你看。",
    glyph: "⌖",
    hue: "var(--mk-lake)",
    minutes: 25,
    fields: [
      {
        id: "refs",
        kind: "rows",
        label: "至少找四个",
        rowsLabel: "参考",
        min: 4,
        hint: "可以是网站、海报、游戏、一份报告——只要和你要做的东西同类。",
        columns: [
          { id: "name", label: "它是什么", placeholder: "名字或网址" },
          { id: "like", label: "你具体喜欢它哪一点", placeholder: "写到「一眼能验证」的程度", wide: true },
        ],
      },
      {
        id: "hate",
        kind: "textarea",
        label: "再写一个你明确不想要的",
        hint: "反例常常比正例好用。你讨厌的那一点，就是你的底线。",
        placeholder: "例如：那种一进去就自动播视频、还得找关闭按钮的。",
      },
    ],
  },

  style: {
    id: "style",
    title: "风格定义",
    kind: "frame",
    surface: "panel",
    reason: "你挑的四个参考里有一条共同的线。我们把它写成一份说明书，这样我做出来的东西你才有资格说「不对」。",
    teaches: "把喜欢变成规格。挑一个参考，先用自己的话描述它，再把这段描述当成给 AI 的约束——这是专业的人真正在用的做法。含糊的指令换来的是含糊的东西。",
    payoff:
      "这份说明书会变成我的约束。特别是你禁止的那几条——有了它们，你才有资格对我做出来的东西说「不对」，而不是只能说「再改改」。",
    glyph: "◈",
    hue: "var(--mk-taro)",
    minutes: 18,
    fields: [
      {
        id: "density",
        kind: "choice",
        label: "密度",
        options: [
          { id: "airy", label: "很空", blurb: "留白多，一屏只放一件事" },
          { id: "medium", label: "适中", blurb: "有呼吸，但不浪费" },
          { id: "dense", label: "很密", blurb: "信息量优先，像一份索引" },
        ],
      },
      {
        id: "voice",
        kind: "choice",
        label: "语气",
        options: [
          { id: "plain", label: "平实", blurb: "把话说清楚，不表演" },
          { id: "warm", label: "亲近", blurb: "像在跟一个人说话" },
          { id: "sharp", label: "利落", blurb: "短句，几乎不解释" },
        ],
      },
      {
        id: "color",
        kind: "choice",
        label: "颜色",
        options: [
          { id: "mono", label: "只有黑白", blurb: "加一个强调色，仅此而已" },
          { id: "warm", label: "暖色纸感", blurb: "米、棕、锈红" },
          { id: "cool", label: "冷色", blurb: "灰蓝、墨绿" },
          { id: "bold", label: "高对比", blurb: "大色块，看一眼就记住" },
        ],
      },
      {
        id: "spec",
        kind: "textarea",
        label: "用你自己的话，把这份风格写成三句话",
        hint: "这三句是给 AI 的约束。写完读一遍：如果换个人照着做，能做出差不多的东西吗？",
        placeholder: "例如：字只有两种大小。除了链接以外没有蓝色。每一屏只讲一件事，讲完就空一大截。",
      },
      {
        id: "forbid",
        kind: "textarea",
        label: "明确禁止什么",
        hint: "禁令比要求更有效。写三条。",
        placeholder: "例如：不要圆角卡片堆成网格。不要图标 + 标题 + 一句话的三段式。不要渐变。",
      },
    ],
  },

  teardown: {
    id: "teardown",
    title: "结构拆解",
    kind: "frame",
    surface: "panel",
    reason: "你最喜欢的那个参考，我想请你把它拆开——不然你学到的只是它的外表。",
    teaches: "任何好东西都有骨架。把它的部分依次写出来、写出每一部分在干什么，你才拿得走它的做法，而不是它的样子。",
    payoff:
      "你拆出来的那个结构，我会照着搭。这比任何形容词都管用：你说得出别人是怎么搭的，我就能搭一个你认得出的。",
    glyph: "▤",
    hue: "var(--mk-mist)",
    minutes: 20,
    fields: [
      { id: "target", kind: "text", label: "你要拆的是哪一个", placeholder: "参考里的名字" },
      {
        id: "parts",
        kind: "rows",
        label: "从上到下，它由哪几块组成",
        rowsLabel: "组成部分",
        min: 3,
        columns: [
          { id: "part", label: "这一块是什么", placeholder: "例如：开头的一句话" },
          { id: "job", label: "它在替作者干什么活", placeholder: "例如：让你知道要不要继续读", wide: true },
        ],
      },
      {
        id: "steal",
        kind: "textarea",
        label: "你要拿走的是哪一招",
        hint: "一招就够。写成一句你能对自己下达的指令。",
        placeholder: "例如：第一屏不放导航，只放一句说清我是谁的话。",
      },
    ],
  },

  /* ── 组织 ─────────────────────────────────────────────────────────────── */
  inventory: {
    id: "inventory",
    title: "内容盘点",
    kind: "plan",
    surface: "panel",
    reason: "在讨论它长什么样之前：你手上现在真的有什么？",
    teaches: "先看你有什么，再决定结构。反过来做，你会先做出一个漂亮的空壳，然后花三周去填它。",
    payoff:
      "你手上真的有什么，决定了这一页能长成什么样。这一栏填完，我拟内容草稿的时候就不会再编——你的库里没有的东西我不会替你写上去。",
    glyph: "▥",
    hue: "var(--mk-matcha)",
    minutes: 20,
    fields: [
      {
        id: "items",
        kind: "rows",
        label: "把要放进去的东西列出来",
        rowsLabel: "内容块",
        min: 3,
        columns: [
          { id: "block", label: "板块", placeholder: "例如：我写过的东西" },
          { id: "have", label: "现在已经有的", placeholder: "例如：4 篇，其中 2 篇能见人" },
          { id: "need", label: "还缺什么", placeholder: "例如：每篇要写一句介绍", wide: true },
        ],
      },
      {
        id: "cut",
        kind: "textarea",
        label: "如果只能留三块，你砍掉哪些？",
        hint: "先砍一次。真做起来你会庆幸自己砍过。",
      },
    ],
  },

  command: {
    id: "command",
    title: "提示词设计",
    kind: "frame",
    surface: "panel",
    reason: "接下来我要动手了。你先把要求写清楚——写得越具体，我做出来的越接近你脑子里的东西。",
    teaches: "AI 在明确的指令下才好用。明确 = 三件事：① 一个直接的例子（像谁），② 内容的结构（按什么顺序放什么），③ 你想怎么和 AI 配合。少一件，你拿到的就是它的默认口味。",
    payoff:
      "这张卡最后会拼出一段可以复制走的指令。它在这里管用，在任何一个 AI 工具里也管用——这是这套流程里你能带走的、最不依赖我们的一样东西。",
    glyph: "⌘",
    hue: "var(--mk-accent-400)",
    minutes: 15,
    fields: [
      {
        id: "like",
        kind: "textarea",
        label: "① 像谁 —— 举一个直接的例子",
        hint: "指名道姓，再写出你要学的那一招。",
        placeholder: "例如：像 Bartosz Ciechanowski 的文章页。我要学的是：解释一个东西的时候，先给一个能拖动的图，再讲字。",
      },
      {
        id: "structure",
        kind: "textarea",
        label: "② 结构 —— 按什么顺序，放什么",
        hint: "编号列出来。规定不许多加也不许少。",
        placeholder: "1. 一句话说我是谁\n2. 我做过的三件事\n3. 我在想的问题\n4. 怎么找到我",
      },
      {
        id: "mode",
        kind: "choice",
        label: "③ 我们怎么配合",
        hint: "这一条最常被跳过，也最影响结果。",
        options: [
          {
            id: "ask",
            label: "你问我答",
            blurb: "印记一次问一个问题，你回答，它只负责整理你的话。适合你已经有东西要说。",
          },
          {
            id: "propose",
            label: "你给方案我来定",
            blurb: "印记给两三个空的骨架，你挑一个再自己填。适合你知道要什么但没头绪怎么摆。",
          },
          {
            id: "tidy",
            label: "我先写你再挑毛病",
            blurb: "你先写，哪怕写得很烂，印记只指出重复、含糊和缺例子的地方。适合你写得动。",
          },
        ],
      },
    ],
  },

  /* ── 动手 ─────────────────────────────────────────────────────────────── */
  variants: {
    id: "variants",
    title: "方案对比",
    kind: "decide",
    surface: "panel",
    reason: "我按你的指令做了三版。它们都是可以的，但都不对——请你告诉我各拿走哪一块。",
    teaches: "让 AI 给选项，不要让它给答案。三个版本摆在一起，你才看得出你真正在意什么；而「A 的结构 + B 的开头 + C 都不要」这句话，只有你说得出来。",
    payoff:
      "你每一次取舍的理由，会变成我后面判断的依据。你说过「不要自动播放」，我就不会再提第二次。",
    method: "gold-standard",
    glyph: "◫",
    hue: "var(--mk-peach)",
    minutes: 25,
    fields: [
      {
        id: "picks",
        kind: "rows",
        label: "从每一版里拿走什么、扔掉什么",
        rowsLabel: "取舍",
        min: 2,
        columns: [
          { id: "from", label: "从哪一版", placeholder: "A / B / C" },
          { id: "take", label: "拿走什么", placeholder: "具体到哪一块" },
          { id: "why", label: "为什么是它", placeholder: "这一栏是这张卡的重点", wide: true },
        ],
      },
      {
        id: "none",
        kind: "textarea",
        label: "三版都没有、但你想要的东西",
        hint: "常常最重要的一条在这里。",
      },
    ],
  },

  proto: {
    id: "proto",
    title: "原型测试",
    kind: "review",
    surface: "chat",
    reason: "在你把它做完美之前——先做一个丑的、能用的，拿去给一个真人试。",
    teaches: "最小版本不是「做一半」，是「小而完整」。定义清楚：什么算做完，谁来试，什么结果算失败。写下失败标准的人，才不会自己骗自己。",
    payoff:
      "你在这里定的「什么算失败」，是这个项目唯一一次能在结果出来之前定标准的机会。做完再定标准的人，永远都会成功。",
    method: "gold-standard",
    glyph: "▣",
    hue: "var(--mk-butter)",
    minutes: 30,
    fields: [
      {
        id: "smallest",
        kind: "textarea",
        label: "最小的一版是什么样？",
        hint: "一句话。它要小到你这周就能做完。",
        placeholder: "例如：一张纸做的药盒标签，只做周一到周三。",
      },
      { id: "who", kind: "text", label: "谁来试", placeholder: "一个具体的人的名字" },
      {
        id: "fail",
        kind: "textarea",
        label: "什么结果算失败？",
        hint: "先写下来。做完之后再定标准的人，永远都会成功。",
        placeholder: "例如：他还是要问我「今天吃哪个」——那就是失败。",
      },
      {
        id: "result",
        kind: "textarea",
        label: "试完了，真实发生了什么",
        hint: "试之前可以先空着。回来填的时候，写你没料到的那部分。",
        optional: true,
      },
    ],
  },

  /* ── 查证 ─────────────────────────────────────────────────────────────── */
  questions: {
    id: "questions",
    title: "问题拆解",
    kind: "frame",
    surface: "panel",
    reason: "你的题目现在还太大。我们把它拆成能一条一条去查的问题。",
    teaches: "把一个笼统的兴趣拆成可回答的问题，并且分清哪些是「查得到的事实」、哪些是「要你判断的」、哪些是「要你设计的」。混在一起问，就会用查资料代替思考。",
    payoff:
      "这份清单会直接变成问卷的题目。你分的类也有用：判断类的问题搜不到答案，只搜得到别人的立场，那类问题我会拦下来。",
    glyph: "?",
    hue: "var(--mk-lake)",
    minutes: 20,
    fields: [
      {
        id: "qs",
        kind: "rows",
        label: "拆成至少五个问题",
        rowsLabel: "问题",
        min: 5,
        columns: [
          { id: "q", label: "问题", placeholder: "写成一句真正的疑问句", wide: true },
          { id: "type", label: "类型", placeholder: "事实 / 判断 / 设计" },
        ],
      },
      {
        id: "first",
        kind: "textarea",
        label: "先查哪一个？为什么是它？",
        hint: "选那个「答案会改变你后面所有决定」的。",
      },
    ],
  },

  sources: {
    id: "sources",
    title: "来源核查",
    kind: "question",
    surface: "chat",
    reason: "你刚才说了一个数字，但没说它是谁量的。我们把来源补上。",
    teaches: "查资料不是找一个支持你的链接。每一条来源要写清楚：谁做的、什么时候、它能证明什么、以及它不能证明什么。最后一栏是分水岭。",
    payoff:
      "你找的来源会跟着结论一起写进报告。尤其是那条反对你的——报告里有它，别人才会认真读你的结论。",
    glyph: "◎",
    hue: "var(--mk-matcha)",
    minutes: 30,
    fields: [
      {
        id: "srcs",
        kind: "rows",
        label: "至少三条，尽量不是同一边的",
        rowsLabel: "来源",
        min: 3,
        columns: [
          { id: "what", label: "来源", placeholder: "标题 / 链接" },
          { id: "who", label: "谁做的", placeholder: "机构、作者" },
          { id: "when", label: "什么时候", placeholder: "年份" },
          { id: "proves", label: "它能证明什么 / 不能证明什么", placeholder: "两半都要写", wide: true },
        ],
      },
      {
        id: "against",
        kind: "textarea",
        label: "有没有一条是反对你的？",
        hint: "如果三条都同意你，多半是你只搜了同一个说法。",
      },
    ],
  },

  clinic: {
    id: "clinic",
    title: "问卷诊断",
    kind: "question",
    surface: "panel",
    reason: "在你把问卷发出去之前——发出去就收不回来了，我们先逐条检查一遍。",
    teaches: "问题的问法会决定答案。逐条对着几种常见的坏问法检查：一句话问了两件事、用词在引导、「同意/不同意」式提问、要人回忆太久、选项顺序有偏。",
    payoff:
      "你在这儿改过的每一道题，都会被真的人读到。一道有毛病的题收回来的是有毛病的数据，而数据一旦收完就改不了了。",
    method: "survey-wording",
    glyph: "✚",
    hue: "var(--mk-berry)",
    minutes: 30,
    fields: [
      {
        id: "items",
        kind: "rows",
        label: "把你的题目一条条贴进来",
        rowsLabel: "题目",
        min: 3,
        columns: [
          { id: "q", label: "你写的题目", placeholder: "原样贴进来", wide: true },
          { id: "flaw", label: "查出的毛病", placeholder: "双重 / 引导 / 同意式 / 回忆 / 顺序 / 没问题" },
          { id: "fix", label: "改成", placeholder: "改完的版本", wide: true },
        ],
      },
      {
        id: "checks",
        kind: "multi",
        label: "整份问卷检查",
        options: [
          { id: "balanced", label: "「同意/不同意」的题，我改成了在两个说法之间选一个" },
          { id: "shuffle", label: "选项顺序会打乱" },
          { id: "open", label: "留了至少一道开放题" },
          { id: "pilot", label: "先找 3 个人试填过" },
          { id: "anon", label: "说清楚了是不是匿名、数据会怎么用" },
        ],
      },
    ],
  },

  sample: {
    id: "sample",
    title: "样本与范围",
    kind: "question",
    surface: "chat",
    reason: "你收到了数据。在算平均数之前，先说清楚这些数据是谁给的。",
    teaches: "结论的适用范围由样本决定，不由样本量决定。写下你问了谁、谁没被问到、以及这件事让你的结论只能说到哪一步。",
    payoff:
      "「谁没被问到」这一栏会原样写进报告。说清楚适用范围的报告更有说服力，而不是更弱。",
    method: "survey-wording",
    glyph: "◐",
    hue: "var(--mk-mist)",
    minutes: 20,
    fields: [
      { id: "asked", kind: "textarea", label: "你问了谁？多少人？怎么找到他们的？" },
      {
        id: "missing",
        kind: "textarea",
        label: "谁系统性地没被问到？",
        hint: "「没空填的人」也是一类人，而且往往是最关键的一类。",
      },
      {
        id: "limit",
        kind: "textarea",
        label: "所以你的结论只能说到哪一步？",
        hint: "把这句话原样写进报告里。它会让你的报告更有说服力，不是更弱。",
      },
    ],
  },

  /* ── 改与发 ───────────────────────────────────────────────────────────── */
  critique: {
    id: "critique",
    title: "同伴评审",
    kind: "review",
    surface: "panel",
    reason: "第一版出来了。按规矩来：先说好的，再说不行的，每一条都要指到具体的地方。",
    teaches: "善意、具体、有用——这三条是给别人提意见的规矩，也是给自己提意见的。顺序也是规矩：先 warm 后 cool。改稿次数是作品的一部分。",
    payoff:
      "每一条 cool feedback 都会变成一次具体的修改。你记下来的稿次也会留在项目里——改了几稿本身就是作品的一部分。",
    method: "critique",
    glyph: "⟳",
    hue: "var(--mk-peach)",
    minutes: 25,
    fields: [
      {
        id: "warm",
        kind: "rows",
        label: "先说好的（warm）",
        rowsLabel: "优点",
        min: 2,
        columns: [
          { id: "where", label: "哪一处", placeholder: "指到具体位置" },
          { id: "why", label: "好在哪", placeholder: "不能写「挺好的」", wide: true },
        ],
      },
      {
        id: "cool",
        kind: "rows",
        label: "再说不行的（cool）",
        rowsLabel: "问题",
        min: 2,
        columns: [
          { id: "where", label: "哪一处", placeholder: "指到具体位置" },
          { id: "problem", label: "问题是什么", placeholder: "描述现象，不下判语" },
          { id: "fix", label: "能动手的建议", placeholder: "对方看完就知道要改什么", wide: true },
        ],
      },
      {
        id: "who",
        kind: "text",
        label: "这轮意见是谁给的",
        placeholder: "同学的名字 / 印记 / 我自己",
      },
    ],
  },

  tradeoff: {
    id: "tradeoff",
    title: "权衡分析",
    kind: "decide",
    surface: "chat",
    reason: "你想要的两件事现在打架了。这一步不能绕过去。",
    teaches: "做东西就是不断放弃。写清楚你放弃了什么、代价是谁承担——一个说得出取舍的人，作品才有立场。",
    payoff:
      "你放弃了什么、代价由谁承担，会写在你交出去的东西上。这一段是别人判断你有没有想清楚的地方。",
    glyph: "⇄",
    hue: "var(--mk-taro)",
    minutes: 15,
    fields: [
      { id: "a", kind: "text", label: "你想要的 A" },
      { id: "b", kind: "text", label: "你想要的 B" },
      { id: "pick", kind: "textarea", label: "你选哪个？放弃的那个，代价是什么？" },
      {
        id: "who",
        kind: "textarea",
        label: "这个代价由谁承担？",
        hint: "如果答案是「没人」，那多半不是一次真的取舍。",
      },
    ],
  },

  ship: {
    id: "ship",
    title: "发布前自查",
    kind: "reflect",
    surface: "chat",
    reason: "最后一步。半成品可以发布，但要说清楚它是半成品。",
    teaches: "诚实地发布：说清楚哪里做完了、哪里没有、你从中学到什么。把没做完的部分写出来的人，得到的是真的反馈。",
    payoff:
      "你写的这段说明会跟着作品一起发出去。写清楚哪儿还不够好的人，收到的才是真的反馈。",
    method: "gold-standard",
    glyph: "▲",
    hue: "var(--mk-accent-400)",
    minutes: 20,
    fields: [
      {
        id: "made",
        kind: "textarea",
        label: "你做出了什么？两三句。",
        hint: "写真实发生的事：你做了什么、遇到了什么、改了什么。这段会出现在你的主页上。",
        placeholder: "例如：我发了 87 份问卷，收回 61 份。最意外的是……",
      },
      {
        id: "unfinished",
        kind: "textarea",
        label: "哪里还是半成品？你打算怎么说明？",
      },
      {
        id: "learned",
        kind: "textarea",
        label: "如果重做一次，你第一件会改的事",
        hint: "不用写「我学到了很多」。写你会怎么改。",
      },
    ],
  },
};

export function cardById(id: string): CardSpec | undefined {
  return CARDS[id];
}

/**
 * The card sequence 印记 works through per track.
 *
 * It is a SEQUENCE, not a schedule: 印记 summons the next one when the
 * previous comes back, and she can open any card from the rail at any time.
 * The order encodes the argument — look before you want, want before you
 * instruct, instruct before you build, build before you polish.
 */
export const TRACK_CARDS: Record<TrackId, string[]> = {
  website: ["motive", "keywords", "sweep", "teardown", "style", "inventory", "command", "variants", "proto", "critique", "ship"],
  design: ["motive", "questions", "sweep", "style", "variants", "proto", "critique", "tradeoff", "ship"],
  game: ["motive", "questions", "sweep", "variants", "proto", "critique", "tradeoff", "ship"],
  survey: ["motive", "questions", "clinic", "sources", "sample", "proto", "critique", "ship"],
  other: ["motive", "questions", "sweep", "command", "variants", "proto", "critique", "ship"],
};

export function cardsForTrack(track: TrackId): string[] {
  return TRACK_CARDS[track];
}

/* ── value helpers ────────────────────────────────────────────────────────
 * The store keeps card values as a loose `Record<string, CardValue>` because
 * the shape is the SPEC's business, not the store's. These three readers are
 * the only place that assumption is cashed in, so a malformed stored value
 * degrades to empty instead of throwing somewhere in a render.
 * ---------------------------------------------------------------------- */

export function asText(v: CardValue | undefined): string {
  return typeof v === "string" ? v : "";
}

export function asList(v: CardValue | undefined): string[] {
  return Array.isArray(v) && v.every((x) => typeof x === "string") ? (v as string[]) : [];
}

export function asRows(v: CardValue | undefined): CardRow[] {
  return Array.isArray(v) && v.every((x) => typeof x === "object" && x !== null)
    ? (v as CardRow[])
    : [];
}

/** A row counts only when it has something in every non-empty column. */
export function filledRows(rows: CardRow[]): CardRow[] {
  return rows.filter((r) => Object.values(r).some((v) => v.trim().length > 0));
}

/** Has she answered enough of this card for it to be worth feeding back? */
export function cardAnswered(spec: CardSpec, values: Record<string, CardValue>): boolean {
  return spec.fields.every((f) => {
    const v = values[f.id];
    switch (f.kind) {
      case "rows":
        return filledRows(asRows(v)).length >= (f.min ?? 1);
      case "multi":
        // Checklists are allowed to be empty — an unchecked box is an answer.
        return true;
      case "choice":
        return asText(v).length > 0;
      default:
        // Fields she can only fill after being somewhere (a test result, what
        // the officer actually said) must not block the card from coming back.
        return f.optional ? true : asText(v).trim().length >= 2;
    }
  });
}

/**
 * 回灌 — what 印记 says when a card comes back.
 *
 * 🚨 This must QUOTE HER. A generic "很好，继续" after twenty minutes of work
 * teaches that nobody read it, which is worse than saying nothing. Every
 * branch below reaches into the actual values.
 */
export function refeed(cardId: string, values: Record<string, CardValue>): string {
  switch (cardId) {
    case "motive": {
      const who = asText(values.who).trim();
      const cost = asText(values.cost).trim();
      return `记下了。这个项目是为${firstClause(who)}做的，不做的代价是${firstClause(cost)}。\n\n我把这三句钉在上面了。到第三周你想放弃的时候，回来读一遍——多数时候管用。`;
    }
    case "sweep": {
      const rows = filledRows(asRows(values.refs));
      const first = rows[0]?.like ?? "";
      return `${rows.length} 个参考，收到。你写的第一条是「${trim(first, 40)}」——这种具体程度正好，我照着这个能干活。\n\n下一步我想请你把最喜欢的那个拆开看看骨架。`;
    }
    case "teardown":
      return `「${trim(asText(values.steal), 46)}」——这句可以直接当指令用了。\n\n接下来把风格定死，我就能动手了。`;
    case "style":
      return `风格说明书收到，尤其是你禁止的那几条。禁令比要求好用：它让我做出来的东西可以被你判定为「不对」。\n\n现在说说你手上到底有什么内容。`;
    case "inventory": {
      const rows = filledRows(asRows(values.items));
      return `${rows.length} 个板块。你砍掉的那些先别删，放在一边——做到一半你会想捡回来一个。\n\n下一张卡是这套流程里最值钱的一张：把要求写成明确指令。`;
    }
    case "command":
      return `好，我拿到了三样东西：像谁、什么结构、我们怎么配合。这就是一条真正能用的指令，你把它复制到任何一个 AI 工具里都成立。\n\n我按它做三版给你挑。`;
    case "variants": {
      const rows = filledRows(asRows(values.picks));
      return `你做了 ${rows.length} 次取舍，每一次都写了理由——这一栏是整张卡的意义。\n\n现在别追求完美。做一个丑的、能用的，找一个真人试。`;
    }
    case "proto": {
      const who = asText(values.who).trim() || "你找的那个人";
      return `失败标准你已经写下来了，这一点很关键：先定标准再做测试的人，才不会自己骗自己。\n\n${who}试完之后回来把最后一栏填上，我们再开批评会。`;
    }
    case "questions": {
      const rows = filledRows(asRows(values.qs));
      return `${rows.length} 个问题，而且你分了类。别把「判断」类的问题拿去搜——那类问题搜不到答案，只搜得到别人的立场。`;
    }
    case "sources": {
      const against = asText(values.against).trim();
      return against.length > 2
        ? `你找到了一条反对自己的来源。这是这张卡最难的一栏，你写了。`
        : `来源记下了。反对你的那一条还空着——三条都同意你，通常说明你只搜了同一个说法。什么时候补上都行。`;
    }
    case "clinic": {
      const rows = filledRows(asRows(values.items));
      return `${rows.length} 道题过了一遍。改完的版本比原来的长，这很正常：把一句话问的两件事拆开，本来就要两句。`;
    }
    case "sample":
      return `「谁没被问到」这一栏写得比结论重要。把它原样写进报告里——说清楚适用范围的报告更有说服力，不是更弱。`;
    case "critique": {
      const cool = filledRows(asRows(values.cool));
      return `${cool.length} 条 cool feedback，每条都带了能动手的建议。\n\n现在去改。改完记一下这是第几稿——改稿次数是作品的一部分。`;
    }
    case "tradeoff":
      return `你说得出放弃了什么，也说得出代价由谁承担。这就是有立场。`;
    case "ship":
      return `写完了。你把没做完的部分也写出来了——这样你收到的才是真的反馈。`;
    case "persona": {
      const name = asText(values.name).trim() || "他";
      const feel = asText(values.feel).trim();
      return `${name} 记下了。你希望他看完觉得「${trim(feel, 30)}」——这句我当成标准用：我做的每一版，你都可以拿它来判断我做得对不对。`;
    }
    case "frame": {
      const seen = asText(values.seen).trim();
      return `清楚了。最重要的是最后一栏：你亲眼看到过——${firstClause(seen)}。

这说明它不是你想象出来的问题。接下来我给你几条不同的路，选哪条由你定。`;
    }
    case "keywords": {
      const words = asText(values.words).trim();
      const after = asText(values.after).trim();
      return after.length > 1
        ? `你回来改了。这一栏写的是「${trim(after, 34)}」——看完一圈能说出自己哪个词变了，比一开始写得多准有用得多。`
        : `记下了：${trim(words, 30)}。现在去看别人的——看完回来把最后一栏填上，那一栏才是这张卡的意义。`;
    }
    case "recon": {
      const lost = filledRows(asRows(values.lost));
      const sight = asText(values.sight);
      const tail =
        sight === "no"
          ? "\n\n你选了「基本看不见下一个点」——这条很重要，它意味着光有标记不够，人在两个标记之间会怕。"
          : "";
      return `你真的去走了。${lost.length} 个迷路点——这份清单坐在屋里是写不出来的。${tail}\n\n我接下来的每一步都会用它。`;
    }
    case "talk": {
      const said = asText(values.said).trim();
      return said.length > 2
        ? `他说的是「${trim(said, 40)}」。把原话记下来而不是记你的总结，这一点做得对——条件里往往藏着下一版的答案。`
        : `想清楚了再去，这比空手去强很多。谈完回来把最后一栏填上，包括被拒绝的话。`;
    }
    default:
      return `收到了。`;
  }
}

function trim(s: string, n: number): string {
  const t = s.trim();
  return t.length > n ? `${t.slice(0, n)}…` : t;
}

/** First sentence-ish chunk of her answer, for quoting back inline. */
function firstClause(s: string): string {
  const t = s.trim().split(/[。；;\n]/)[0] ?? s.trim();
  return trim(t, 34) || "（你还没写）";
}

/**
 * Her three why-answers, read back off the 动机三问 card.
 *
 * The motivation used to be its own field on `Project`, which meant the same
 * three sentences existed in two places and could disagree. The card IS the
 * record; everything that wants to quote her reads it from here.
 */
export function motiveOf(p: Project): { who: string; cost: string; mine: string } | null {
  const entry = p.cards.find((c) => c.cardId === "motive" && c.status === "done");
  if (!entry) return null;
  return {
    who: asText(entry.values.who),
    cost: asText(entry.values.cost),
    mine: asText(entry.values.mine),
  };
}
