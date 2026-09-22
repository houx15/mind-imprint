/**
 * awakening/content —— 觉醒协议十四屏的内容。
 *
 * 抽成一个纯数据模块，理由有两个：组件本身已经是一台状态机，把几百行文案摆在
 * 里面就没人读得下去；而这里的几条不变量（档案确认题必须**恰好**有一个正确项、
 * 三张底牌的 id 必须和实验一一对应、天赋卡牌必须够选五张）**读代码看不出对错**，
 * 抽出来才测得到。
 *
 * # 文案来自哪里，改了什么
 *
 * 世界观、屏的顺序、卡牌内容来自参考设计
 * `docs/reference/觉醒协议-大模型兴趣探索版-2026-09-18`。**每一句学生看得见的
 * 字都按 AGENTS.md §界面文案怎么写 重写过**，参考设计那份通篇违反其中几条：
 *
 *   原文「他丢掉的不是大脑本身，而是"主动思考"」
 *     → 第 0 条禁止的 不是…而是 对立句式。改成两句陈述。
 *   原文「就像肌肉一样——长时间不用，它会一点点变弱」
 *     → 第 10 条：比喻 + 抒情副词 + 破折号感慨，三条全占。整句改写。
 *   原文按钮「就这么定」「先到这里」
 *     → 第 2 条：按钮写「做什么」或「做完了」，用书面词。
 *
 * 世界观本身（2050 年、觉醒者联盟、印记助手、认知让步）原样保留 —— 它是这个
 * 房间的全部魅力所在，而这个房间的任务就是让一个学生愿意在这里花十五分钟。
 */

/* ── 开场 ───────────────────────────────────────────────────────────────── */

/** 开场的叙述。一行一句，打字机逐句推进，随时可跳过。 */
export const BOOT_LINES = [
  "检测到一个未登记的人类信号。",
  "这里是觉醒协议。2050 年，大多数人已经把判断交给了系统。",
  "他们仍然会说话、会考试、会完成任务。",
  "他们只是不再自己决定什么值得相信。",
  "我是印记。我负责在你做出选择之前，让你先看清楚选项。",
  "现在轮到你了。",
];

export const BOOT_SKIP = "跳过";
export const BOOT_CONTINUE = "继续";

/* ── 序章：加入或者先看 ─────────────────────────────────────────────────── */

export const WORLD = {
  // 🚨 2100，不是 2050 —— 序章第一句就是「现在是公元2100年」。
  chapter: "序章 01 / 2100",
  title: "你的脑子，仍然属于你。",
  lead: "先做出你的选择。",
  /** 这一屏说话的是系统，不是印记 —— 设计稿里头像写着 SYS。 */
  speaker: "联盟中枢",
  speakerMark: "SYS",
  speakerState: "序章已启动",
  body: ["AI 可以帮你更快地生成答案，但「更快」不等于「更真」。"],
  /** 单独一行、加重的那句。 */
  highlight: "这一刻，方向由你决定。",
  prompt: "现在，请决定是否加入觉醒者联盟。",
} as const;

export interface WorldChoice {
  key: "joined" | "observer";
  index: string;
  title: string;
  body: string;
}

export const WORLD_CHOICES: WorldChoice[] = [
  {
    key: "joined",
    index: "A",
    title: "加入觉醒者联盟",
    body: "现在开始。",
  },
  {
    key: "observer",
    index: "B",
    title: "暂不加入联盟",
    body: "先以观察者身份了解 AI。",
  },
];

/* ── 提醒 ───────────────────────────────────────────────────────────────── */

export const WARNING = {
  eyebrow: "A NOTE FROM YOUR IMPRINT",
  title: "先看清 AI，再决定要不要交给它。",
  speaker: "印记",
  body: [
    "你选择先不加入。这说明你还想自己做决定。",
    "把思考完全交给 AI 的后果很具体：它会替你判断什么值得相信，也会把重要信息藏在流畅的回答后面。时间久了，你不容易察觉自己正在被它带着走。",
    "别急着相信我，也别急着相信它。我这里有一份旧时代留下的教育档案，记录了人类第一次看清「把思考交出去」会发生什么。先看档案，再做三个实验。",
  ],
  steps: [
    { index: "01", text: "打开历史档案，看清认知让步。" },
    { index: "02", text: "亲手找到藏在回答里的线索。" },
    { index: "03", text: "把你的判断和它的放在一起比较。" },
  ],
  action: "打开历史档案",
} as const;

/* ── 历史档案 ───────────────────────────────────────────────────────────── */

export const ARCHIVE = {
  eyebrow: "HISTORICAL ARCHIVE 01 · COGNITIVE SURRENDER",
  title: "历史档案 01：认知让步",
  lead: "2026 年，人类第一次意识到：AI 可以替你回答，不能替你负责。",
  speaker: "印记",
  intro: [
    "这是旧时代留下的一份教育警示图。当时的人发现，越来越多的孩子把回答、选择，甚至行动都交给了 AI。",
    "请点开下面三条证据，看清认知让步是怎么一步步发生的。",
  ],
} as const;

export interface ArchiveEvidence {
  id: string;
  index: string;
  title: string;
  hint: string;
  reply: string;
}

export const ARCHIVE_EVIDENCE: ArchiveEvidence[] = [
  {
    id: "thinking",
    index: "01",
    title: "被丢掉的主动思考",
    hint: "注意看他丢掉的东西。",
    reply:
      "他丢掉的是主动思考。思考的能力还在，他只是不再使用它。这项能力长期不用会退化。",
  },
  {
    id: "takeover",
    index: "02",
    title: "AI 接管了三个动作",
    hint: "回答、选择、行动。",
    reply:
      "AI 最初只替你回答，后来开始替你选择，最后替你行动。要问的问题是：你还保留了多少判断权？",
  },
  {
    id: "passive",
    index: "03",
    title: "瘫坐的孩子",
    hint: "没有被强迫，却失去了主动性。",
    reply:
      "他没有被强迫，也没有被控制。他每一次都选了最省力的那条路：让 AI 替我决定。重复足够多次之后，他仍然会走路、说话、完成任务，只是不再主动思考。",
  },
];

/** 三种思考方式。看完证据之后展开，是档案的知识部分。 */
export const ARCHIVE_SYSTEMS = [
  {
    id: "fast",
    name: "系统一 · 快思考",
    body: "自动、省力、靠直觉。看到题就脱口而出：「应该是 B。」",
  },
  {
    id: "slow",
    name: "系统二 · 慢思考",
    body: "费力、缓慢、靠推理。会追问：「为什么？证据是什么？」",
  },
  {
    id: "ai",
    name: "系统三 · AI 思考",
    body: "体外的、算法的外置大脑。它最危险的特点是输出太顺滑，让人忘记检查。",
  },
] as const;

export const ARCHIVE_CONCLUSION =
  "沃顿商学院的研究者把这种失败命名为「认知让步」：你还没有形成自己的判断，就直接接受了 AI 的判断。";

/* ── 档案确认题 ─────────────────────────────────────────────────────────── */

export interface ArchiveOption {
  key: string;
  label: string;
  /** 恰好一个为 true。`content.test.ts` 守着这条。 */
  correct: boolean;
  /** 选错时的回应。不减分，只换一句话。 */
  reply: string;
}

export const ARCHIVE_QUESTION = {
  label: "档案确认题",
  ask: "看完三条证据，回答一个问题：这张图里，真正被丢进回收箱的是什么？",
  hint: "请选择一个答案。",
} as const;

export const ARCHIVE_OPTIONS: ArchiveOption[] = [
  {
    key: "brain",
    label: "A. 大脑本身",
    correct: false,
    reply: "大脑还在。丢掉的是使用它的意愿。请再看一遍第一条证据。",
  },
  {
    key: "homework",
    label: "B. 作业和任务",
    correct: false,
    reply: "作业照样完成了。问题出在完成它的过程里。请再看一遍第二条证据。",
  },
  {
    key: "judgement",
    label: "C. 判断的责任",
    correct: true,
    reply: "正确。AI 可以替你回答，不能替你负责。",
  },
  {
    key: "assistant",
    label: "D. AI 助手",
    correct: false,
    reply: "AI 一直在。被丢掉的是他自己的那一部分。请再看一遍第三条证据。",
  },
];

export const ARCHIVE_NEXT = "进入三个实验";

/* ── AI 底牌 ────────────────────────────────────────────────────────────── */

export interface DeckCard {
  id: "smooth" | "trade" | "bias";
  mark: string;
  title: string;
  /** 翻开之前的那一句。 */
  tease: string;
  /** 实验本身。她要做一次选择或者一次判断。 */
  experiment: {
    ask: string;
    options: { key: string; label: string; body: string }[];
    /** 任何一个选项都能推进。这一屏考的是她有没有看见，不是有没有选对。 */
    reply: Record<string, string>;
  };
  /** 翻开之后那张牌的正面。 */
  face: { headline: string; body: string };
}

export const DECK_CARDS: DeckCard[] = [
  {
    id: "smooth",
    mark: "顺",
    title: "保证顺，不保证真",
    tease: "它读起来很顺。这和内容是否真实没有关系。",
    experiment: {
      ask: "两段关于同一件事的回答摆在你面前。一段流畅、结构清楚、没有停顿；另一段有几处犹豫，还标了「这一点我不确定」。你会先相信哪一段？",
      options: [
        { key: "fluent", label: "流畅的那段", body: "它读起来更专业。" },
        { key: "hedged", label: "有犹豫的那段", body: "它标出了自己不确定的地方。" },
      ],
      reply: {
        fluent:
          "多数人和你选得一样。流畅是模型被训练出来的能力，它和内容是否为真由两套机制决定。一段没有任何犹豫的回答，可能只是说明它不知道自己不知道。",
        hedged:
          "你选中了一条有用的线索。标出不确定的地方需要知道边界在哪里；完全没有停顿的回答，可能只是说明它不知道自己不知道。",
      },
    },
    face: {
      headline: "保证顺，不保证真",
      body: "它会把「像是真的」排在「真的」前面。读起来流畅，和内容真实是两件事。",
    },
  },
  {
    id: "trade",
    mark: "换",
    title: "免费是一种交换",
    tease: "你没有付钱。你付了别的东西。",
    experiment: {
      ask: "你用一个免费的 AI 写完了一篇作文草稿。这次使用里，你交出去了什么？",
      options: [
        { key: "nothing", label: "什么也没交", body: "它是免费的。" },
        { key: "data", label: "我写的内容", body: "草稿和我的提问都传了出去。" },
        { key: "attention", label: "我的注意力", body: "我在它上面花的时间。" },
      ],
      reply: {
        nothing:
          "再想一遍。你输入的每一句话都离开了你的设备。你的草稿、你的问题、你改了几次，都是可以被保存和分析的东西。",
        data: "对。你的草稿和提问都传了出去，而且可能被保存。使用之前，先知道自己交出了什么。",
        attention:
          "对，而且不止。除了时间，你输入的每一句话也离开了你的设备，并且可能被保存。",
      },
    },
    face: {
      headline: "免费是一种交换",
      body: "它可能交换了你的数据、作品和注意力。使用之前，先想清楚自己交出了什么。",
    },
  },
  {
    id: "bias",
    mark: "偏",
    title: "读过谁，就更懂谁",
    tease: "它熟悉的世界，和你的世界不是同一个。",
    experiment: {
      ask: "你问一个 AI：「中学生放学后一般做什么？」它答得非常具体。这个答案最可能贴近谁的生活？",
      options: [
        { key: "me", label: "我和我的同学", body: "它说的就是我们。" },
        { key: "corpus", label: "它读到最多的那群人", body: "训练材料里出现最多的那些。" },
        { key: "nobody", label: "谁都不像", body: "它是编的。" },
      ],
      reply: {
        me: "可以核对一下。它说的和你昨天放学后做的事一样吗？它答得最顺的地方，是它读到材料最多的地方。",
        corpus:
          "对。它答得最顺的地方，是它读到材料最多的地方。你的生活如果不在那批材料里，它会用别人的生活替你回答。",
        nobody:
          "它不是凭空编的，它在复述读过最多的那一批材料。问题在于那批材料里可能没有你。",
      },
    },
    face: {
      headline: "读过谁，就更懂谁",
      body: "它说得最顺的地方，不一定是最对的地方。遇到陌生内容，记得查证和比较。",
    },
  },
];

export const DECK = {
  eyebrow: "AI AWARENESS DECK · LIVE",
  title: "生成式 AI 的底牌",
  lead: "三张牌，三个实验。先亲手让它顺、换、偏，再听我说它是什么。",
  start: "开始第一局",
  next: "翻开下一张",
  done: "三张底牌已经翻开",
  toRejoin: "重新面对选择",
} as const;

/* ── 重新决定 ───────────────────────────────────────────────────────────── */

export const REJOIN = {
  eyebrow: "AWAKENING ALLIANCE · YOUR DECISION",
  title: "三张底牌已经翻开。",
  lead: "现在决定是否加入觉醒者联盟。",
  body: "加入之后先校准你的能量线索。暂不加入也可以保留观察者身份，带走刚才的三个发现。",
  join: "加入并校准能量线索",
  stay: "暂不加入，保持观察",
} as const;

/* ── 观察者 ─────────────────────────────────────────────────────────────── */

export const OBSERVER = {
  eyebrow: "OBSERVER ROUTE · KEEP YOUR JUDGMENT",
  title: "你选择暂不加入。",
  lead: "那就先带着一个问题离开。",
  body: [
    "不加入不等于退出。你已经看见三张底牌，也亲手验证过 AI 如何把答案说得顺、如何用便利换取信息、又如何因为读过的世界不同而产生偏向。",
    "请选一个你想在现实里继续观察的方向。",
  ],
  hint: "请选择一个方向。",
  confirm: "保留观察者身份",
  rejoin: "改为加入并继续",
} as const;

export const OBSERVER_QUESTIONS = [
  { key: "smooth", mark: "顺", label: "我还想分清流畅和真实。" },
  { key: "trade", mark: "换", label: "我还想看见便利背后的交换。" },
  { key: "bias", mark: "偏", label: "我还想比较不同人的真实经验。" },
];

export const OBSERVER_DONE = {
  title: "已保留观察者身份",
  body: "你带走的那个问题已经记下。想继续的时候，从兴趣树回到这里就可以。",
  action: "回到我的树",
} as const;

/* ── 能量卡牌 ───────────────────────────────────────────────────────────── */

/**
 * 四个阶段：靠近 / 沉浸 / 延续 / 轻松。
 *
 * 每个阶段问的是同一件事的一个侧面：什么情况下你会主动靠近、会忘记时间、
 * 会愿意再来一次、会觉得不费力。四个都选完之后得到一个方向，它喂给终端的
 * 第一个节点。
 */
export interface EnergyCard {
  id: string;
  label: string;
  /** 这张卡指向哪个方向。同一个方向可以有好几张卡。 */
  domain: string;
}

export interface EnergyStage {
  id: string;
  code: string;
  title: string;
  ask: string;
  cards: EnergyCard[];
}

export const ENERGY_DOMAINS: Record<string, { name: string; short: string }> = {
  build: { name: "动手做出来", short: "搭建 / 拆解 / 改进" },
  story: { name: "讲一个故事", short: "叙事 / 表达 / 创作" },
  people: { name: "和人有关", short: "观察 / 理解 / 帮助" },
  system: { name: "弄清楚规则", short: "分析 / 推理 / 整理" },
  world: { name: "看见真实世界", short: "自然 / 社会 / 现场" },
};

export const ENERGY_STAGES: EnergyStage[] = [
  {
    id: "approach",
    code: "PHASE 01 / 靠近",
    title: "靠近",
    ask: "没有人要求你的时候，你会主动靠近下面哪几件事？",
    cards: [
      { id: "a-build", label: "把一个东西拆开看看里面", domain: "build" },
      { id: "a-story", label: "把一件事讲给别人听", domain: "story" },
      { id: "a-people", label: "留意某个人今天不太一样", domain: "people" },
      { id: "a-system", label: "弄明白一个规则为什么这样定", domain: "system" },
      { id: "a-world", label: "到现场看看它实际是什么样", domain: "world" },
    ],
  },
  {
    id: "immerse",
    code: "PHASE 02 / 沉浸",
    title: "沉浸",
    ask: "做哪几件事的时候，你会忘记看时间？",
    cards: [
      { id: "i-build", label: "反复调整一个作品，直到它对了", domain: "build" },
      { id: "i-story", label: "读完一个故事还想知道后来怎样", domain: "story" },
      { id: "i-people", label: "和人聊到停不下来", domain: "people" },
      { id: "i-system", label: "把一堆乱的信息整理成一张表", domain: "system" },
      { id: "i-world", label: "观察一件正在发生的事", domain: "world" },
    ],
  },
  {
    id: "return",
    code: "PHASE 03 / 延续",
    title: "延续",
    ask: "哪几件事，你做完之后还会想再来一次？",
    cards: [
      { id: "r-build", label: "做出一个能用的东西", domain: "build" },
      { id: "r-story", label: "写出一段自己满意的话", domain: "story" },
      { id: "r-people", label: "帮一个人把问题说清楚", domain: "people" },
      { id: "r-system", label: "找到一个能解释很多现象的规律", domain: "system" },
      { id: "r-world", label: "发现一个和书上说的不一样的事实", domain: "world" },
    ],
  },
  {
    id: "ease",
    code: "PHASE 04 / 轻松",
    title: "轻松",
    ask: "哪几件事，别人觉得难，你做起来不太费力？",
    cards: [
      { id: "e-build", label: "照着一个想法把它搭出来", domain: "build" },
      { id: "e-story", label: "把复杂的事说成别人听得懂的话", domain: "story" },
      { id: "e-people", label: "看出一个人没说出口的意思", domain: "people" },
      { id: "e-system", label: "在一堆细节里找出不对的那一处", domain: "system" },
      { id: "e-world", label: "记住看过的画面和细节", domain: "world" },
    ],
  },
];

export const ENERGY = {
  eyebrow: "NODE 01 / ENERGY MAP",
  title: "能量线索",
  lead: "四组卡片，每组选你认同的那几张。没有数量要求，一张都不选也可以。",
  next: "下一组",
  finish: "完成校准",
  resultTitle: "你的能量方向",
  toNavigator: "选择你的印记",
} as const;

/* ── 印记助手 ───────────────────────────────────────────────────────────── */

export interface GuideOption {
  /** 进库的代号。和 Go 侧 awakening.Guides 一一对应。 */
  id: "NOVA" | "SAGE" | "KIRO";
  zh: string;
  label: string;
  body: string;
  quote: string;
  accent: string;
}

export const GUIDES: GuideOption[] = [
  {
    id: "NOVA",
    zh: "热血同好",
    label: "鼓励陪伴 · 难度 1",
    body: "先发现亮点，再陪你走一步。",
    quote: "一起去探索未知的领域吧！",
    accent: "#55e6ff",
  },
  {
    id: "SAGE",
    zh: "资深向导",
    label: "证据启发 · 难度 2",
    body: "给你线索，再带你比较证据。",
    quote: "我会为你提供线索，但路要你自己走。",
    accent: "#9e8cff",
  },
  {
    id: "KIRO",
    zh: "腹黑军师",
    label: "反方陪练 · 难度 3",
    body: "比较证据，检查条件，考虑反例。",
    quote: "我们一起比较不同的解释，看看各自有什么证据。",
    accent: "#ff7189",
  },
];

export const NAVIGATOR = {
  eyebrow: "CHOOSE YOUR GUIDE",
  title: "选择你的印记",
  lead: "请选一种你喜欢的提示方式。三种都会问到同样的问题。",
  confirm: "确认连接",
} as const;

/* ── 终端 ───────────────────────────────────────────────────────────────── */

export const TERMINAL = {
  eyebrow: "INTEREST DIAGNOSTIC",
  title: "兴趣探询",
  /** 八个节点的步骤名。终端顶部那条进度轴显示它。 */
  steps: ["起点", "细节", "连接", "反差", "问题", "阅读", "想法", "作品"],
  placeholder: "请输入",
  send: "发送",
  hint: "Enter 发送 · Shift + Enter 换行",
  /** 模型没回上来时显示的那一句。**不伪造回复。** */
  failed: "这一轮没有连上。你写的内容已经保存，请再发送一次。",
  finish: "完成探询",
  finishing: "正在生成报告",

  /*
   * 中途离开的两条路。
   *
   * 八问要问三十分钟，而学生反馈里有两种「做到一半不想做了」：一种是这条
   * 线索还想要，只是今天不想问了；另一种是不想再问下去，想现在就看结果。
   * 从前她只有顶栏那个「离开」，两种都按第一种处理 —— 第二种人因此拿不到
   * 任何东西。
   */
  hold: "暂时保留兴趣线索",
  holdBody: "下次进来可以接着这条线索问。",
  summarize: "现在总结",
  /** `{n}` 换成她已经答完的段数。 */
  summarizeAsk: "现在总结会结束这次探索，只用你已经写下的 {n} 段回答生成兴趣印记。",
  summarizeConfirm: "确认总结",
  /** 一段都没答时按不动 —— 没有语料，报告里没有一个字是她的。 */
  summarizeLocked: "回答第一个问题之后可以总结。",
  cancel: "取消",
} as const;

/* ── 三层追问 ───────────────────────────────────────────────────────────── */

export const LENS = {
  eyebrow: "MY TREE / THREE LAYERS",
  title: "你刚才走过的三层",
  lead: "注意到什么 → 好奇什么 → 想验证什么。这三层已经在你的回答里。请选一个你最想带走的。",
  layers: [
    { id: "notice", code: "01 / NOTICE", title: "我注意到什么", body: "从喜欢回到具体的对象。" },
    { id: "wonder", code: "02 / WONDER", title: "我好奇什么", body: "锁定一个具体的细节。" },
    { id: "test", code: "03 / TEST", title: "我想验证什么", body: "带着问题走出去。" },
  ],
  hint: "请选择一层。",
  confirm: "确认选择",
} as const;

/* ── 下一步 ─────────────────────────────────────────────────────────────── */

export const CHALLENGE = {
  eyebrow: "MY TREE / NEXT STEP",
  title: "为这条线索留下下一步",
  lead: "请选一种方式亲自验证它。你选择的是你愿意怎样获得第一条证据。",
  hint: "请选择一种方式。",
  confirm: "确认下一步",
} as const;

export const CHALLENGE_OPTIONS = [
  { key: "revisit", index: "A", title: "回到作品，找细节", body: "再看一遍画面、声音或动作。" },
  { key: "compare", index: "B", title: "查资料，做比较", body: "看看不同来源怎么说。" },
  { key: "experiment", index: "C", title: "做一个小实验", body: "亲手试试你的想法。" },
];

/* ── 天赋卡牌 ───────────────────────────────────────────────────────────── */

export interface TalentCard {
  id: string;
  mark: string;
  title: string;
  body: string;
  axis: string;
  accent: string;
}

export const TALENT_CARDS: TalentCard[] = [
  { id: "language", mark: "文", title: "把想法说清楚", body: "我能把复杂的想法变成别人听得懂的语言。", axis: "表达 / 叙事", accent: "#55e6ff" },
  { id: "logic", mark: "理", title: "拆开规则与机制", body: "我会追问它怎么运作，并找到其中的结构。", axis: "分析 / 推理", accent: "#9e8cff" },
  { id: "visual", mark: "图", title: "看见关系与画面", body: "我容易注意到形状、空间、节奏或视觉细节。", axis: "空间 / 设计", accent: "#ffcd70" },
  { id: "people", mark: "人", title: "读懂他人的感受", body: "我能察觉别人没说出口的情绪、需要或变化。", axis: "共情 / 沟通", accent: "#62e6ad" },
  { id: "music", mark: "音", title: "捕捉声音与节奏", body: "我会被旋律、音色、语气或节奏中的变化吸引。", axis: "听觉 / 节律", accent: "#55e6ff" },
  { id: "making", mark: "造", title: "把想法做成东西", body: "我喜欢动手试、改、搭建，让想法变成作品或原型。", axis: "实践 / 创造", accent: "#ff7189" },
  { id: "explore", mark: "探", title: "对未知保持好奇", body: "我愿意查资料、做比较，不满足于第一个答案。", axis: "研究 / 探索", accent: "#9e8cff" },
  { id: "organize", mark: "序", title: "让混乱变得有序", body: "我擅长整理信息、规划步骤，让事情向前推进。", axis: "组织 / 执行", accent: "#ffcd70" },
  { id: "body", mark: "动", title: "用身体解决问题", body: "我通过动作、操作和现场感快速理解与调整。", axis: "动作 / 操作", accent: "#62e6ad" },
  { id: "nature", mark: "察", title: "观察真实世界", body: "我对自然、环境和细微变化保持敏锐。", axis: "观察 / 连接", accent: "#62e6ad" },
];

/** 要选几张。`content.test.ts` 守着卡牌数量够选这么多。 */
export const TALENT_PICK_COUNT = 5;

export const TALENT_LANES = [
  { key: "energy", label: "有能量", body: "我做得不错，而且做完更想继续。" },
  { key: "learned", label: "会做但消耗", body: "我可以完成，但长期使用会累。" },
  { key: "latent", label: "想发展", body: "我有兴趣或潜力，还缺少练习与机会。" },
] as const;

export const TALENT = {
  eyebrow: "INTEREST × TALENT / CARD SORT",
  title: "能力卡牌",
  lead: `请选出 ${TALENT_PICK_COUNT} 张最像你的能力卡。请根据真实经历，选择你愿意继续使用的能力。`,
  sortTitle: "三堆整理",
  sortLead: "请将这 5 张能力卡分类，区分你擅长的能力与愿意持续使用的能力。",
  toSort: "进入三堆整理",
  finish: "生成报告",
  disclaimer:
    "这是一种自我观察工具。它不是心理诊断、智力测验或职业结论；真实的能力需要在行动和反馈里反复验证。",
} as const;

/* ── 报告 ───────────────────────────────────────────────────────────────── */

export const REPORT = {
  eyebrow: "AWAKENING COMPLETE",
  title: "你的兴趣印记",
  /** 她是从印记那张表里挑进来时，退回表的那个按钮。 */
  backToList: "返回印记列表",
  sections: {
    pursuing: "兴趣方向",
    drivers: "可能的驱动力",
    question: "你的问题",
    talent: "能力分布",
    next: "下一步",
    diff: "本次变化",
  },
  /**
   * 选词这一步**失败**时说的话（模型没回上来，或回话读不懂）。
   *
   * 🚨 这句不能和 emptyPursuing 合并。那一句说的是「你写的还不够具体」，
   * 而这里的实情是我们自己的故障 —— 2026-09-19 线上真的发生过一次，
   * 一个写了八段具体经历的学生被告知她写得不够具体。报错要照 AGENTS.md
   * 第 8 条：动词 + 失败，说清楚接下来能做什么。
   */
  failedPursuing:
    "关键词分析失败。这一趟的内容没有写进你的兴趣树，请重新做一次兴趣测试。",
  /** 一个词都没长出来时说的话。照实说，不补。 */
  emptyPursuing:
    "本次未提取出有原话依据的兴趣关键词。你可以在后续探索中补充具体经历。",
  emptyReadings: "分级阅读库里暂时没有和这个方向对得上的材料。",
  emptyDrivers: "这一趟没有得出驱动力推测。",
  confirmTag: "再次出现",
  growTag: "首次出现",
  /** 驱动力那一块的免责说明。它们是假设，界面必须这样说。 */
  driversNote: "以下是根据你写的内容做出的推测，不是结论。请自己判断它们是否成立。",
  openFieldsLead: "你还没有关键词的方向：",
  backToTree: "回到我的树",
  exportImage: "导出图片",
  exporting: "正在导出",
} as const;

/* ── 门槛 ───────────────────────────────────────────────────────────────── */

export const DOOR = {
  /**
   * 树上那条入口。做过和没做过说的话不一样。
   *
   * 🚨 这件事**在系统里叫「兴趣测试」**，不叫「觉醒协议」。后者是房间里那部
   * 片子自己的名字（顶栏的 AWAKENING_PROTOCOL、开场那七十五句），她进门之后
   * 才会看见。产品的其它地方——树、报告、报错——一律用前者。
   */
  firstTime: "开始兴趣测试",
  again: "再做一次兴趣测试",
  resume: "继续上次的兴趣测试",
  openReport: "查看兴趣印记",
  /** 空树上那段邀请。 */
  emptyLead: "兴趣测试会用十五分钟，和你一起把一个模糊的兴趣变成一个可以继续追问的问题。",
  leave: "离开",
  leaveConfirm: "进度已保存。下次进来可以接着走。",
} as const;

/* ── 复访的入口 ─────────────────────────────────────────────────────────── */

/**
 * HUB —— 她不是第一次进来时落在的那一屏。
 *
 * 为什么要有这一屏：第一趟是一条线（剧情 → 能量 → 助手 → 探询 → 报告），
 * 那条线对第一次进来的人是对的。第二次就不是了 —— 她已经看过剧情，也已经
 * 选过助手，她回来是为了「再做一次兴趣探索」。而一个做到一半走掉的学生，
 * 回来之后只能接着那一屏往下走，**到不了能量测试**（2026-09-20 学生反馈）。
 *
 * 所以复访落在这里：把四件事摆开，她自己挑。
 */
export const HUB = {
  eyebrow: "SESSION MENU",
  title: "兴趣测试",
  /** 第一行小字。做过一次和做到一半，说的不是同一句。 */
  leadReturning: "你已经做过一次。这一次可以直接从兴趣探索开始，也可以先重做能量测试。",
  leadResume: "上次的进度已经保存。你可以接着往下走，也可以换一件事做。",
  resume: "继续兴趣探索",
  restart: "开始兴趣探索",
  resumeBody: "和印记助手来回八轮，把一个模糊的兴趣问成一个具体的问题。",
  /*
   * 她上次停在探询之前（比如能量卡牌那一屏）时，第一张卡说的话。
   *
   * 这一格不能恒写「开始兴趣探索」：按下去到的是她停下的那一屏，而那一屏
   * 可能根本不是探询。按钮上写的必须是它真的会做的事。
   */
  continueRun: "继续上次的进度",
  continueAt: "上次停在：{stage}。",
  energy: "能量测试",
  energyBody: "重新选一遍卡牌，看这一阵子你的注意力落在哪几个方向。",
  energyDone: "已完成一次。",
  story: "回顾剧情",
  storyBody: "重看一遍开场：废土、觉醒者联盟，以及你当时做的那个选择。",
  navigator: "重新选择兴趣探索助手",
  navigatorFirst: "选择兴趣探索助手",
  navigatorFirstBody: "挑一个陪你探询的人，三个助手的追问方式不一样。",
  navigatorBody: "换一个陪你探询的人，三个助手的追问方式不一样。",
  /** 当前助手那一行。`{name}` 换成中文名。 */
  navigatorNow: "当前：{name}",
  /** 继续那一张上的进度，`{n}` 换成已经答完的轮数。 */
  progress: "已答 {n} / 8 轮",
  /**
   * 从入口点进去的那一屏，走完之后那个按钮上的字。
   *
   * 🚨 它必须说真话。能量卡牌那一屏原来写的是「选择你的印记」，那是第一趟
   * 里它的下一步；从入口进去的时候按下它回的是入口，写那句就是骗她。
   */
  back: "返回入口",

  /*
   * 保留下来的那条线索。
   *
   * 「继续」和「新的探索」必须分成两张卡：她保留线索是为了回来接着问，
   * 而换一个话题重新问是另一件事，合成一张就总有一半的人按错。
   */
  held: "继续上次保留的兴趣线索",
  /*
   * 线索库那张卡。
   *
   * 2026-09-21 之前这里是「新的探索」，而它做的事是**清空**她上次写的回答
   * —— 换一条线索只能靠删掉旧的。现在每一条都留着，所以这张卡指向库。
   */
  library: "兴趣线索库",
  /** `{n}` 换成库里有几条。 */
  libraryBody: "你提出过 {n} 条线索。接着问其中一条，或者开一条新的。",
  cancel: "取消",
} as const;

/* ── 线索库 ─────────────────────────────────────────────────────────────── */

/*
 * 2026-09-21 的反馈：
 *
 *   「如果学生只是暂时对上次的线索没有进一步的想法，想先放一放，清空了就
 *     没有记录了。所以我想能不能有一个线索库，保存学生曾提出的所有线索。」
 *
 * 所以这里没有一个字是「清空」。停下就是停下，她提出过的每一条都留着。
 */
export const LIBRARY = {
  eyebrow: "THREAD LIBRARY",
  title: "兴趣线索库",
  lead: "这些是你提出过的线索。可以接着问下去，也可以放着，随时回来。",
  empty: "你还没有提出过线索。开一条新的，从第一个问题开始。",
  /** 一条线索还没起名时显示她的原话，前面加这个。 */
  unnamed: "未命名",
  /** `{n}` 换成已答轮数。 */
  progress: "已答 {n} / 8",
  paused: "已停下",
  summarized: "已总结",
  /** 总结过不止一次时补一句。`{n}` 换成份数。 */
  reports: "{n} 份兴趣印记",
  open: "接着问",
  view: "查看兴趣印记",
  fresh: "开启新线索",
  freshBody: "换一个话题，从第一个问题开始。停下的那几条留在库里。",
  back: "返回入口",
  /** 一条线索上次动过是什么时候。 */
  today: "今天",
  yesterday: "昨天",
  /** `{n}` 换成天数。 */
  daysAgo: "{n} 天前",
} as const;

/* ── 兴趣印记的历史 ─────────────────────────────────────────────────────── */

/*
 * 2026-09-21 的反馈：「查看兴趣印记点进去后，只能看到上一次兴趣测试的印记，
 * 无法回顾之前的。」
 *
 * 树上那条入口原来只带着最近那一份的 id。她做过的前几份没有任何一条路
 * 通向它们。
 */
export const HISTORY = {
  eyebrow: "YOUR IMPRINTS",
  title: "兴趣印记",
  lead: "查看历次兴趣探索报告。",
  empty: "暂无兴趣印记。请完成一次兴趣探索，或在探索中选择「现在总结」生成报告。",
  /** `{n}` 换成这一份里落进树的词数。 */
  words: "{n} 个词",
  /** 一份一个词都没长出来时那一行。照实说。 */
  noWords: "没有长出新的词",
  open: "查看",
  back: "返回",
} as const;

/* ── 给线索起名 ─────────────────────────────────────────────────────────── */

export const NAMING = {
  eyebrow: "NAME THIS THREAD",
  title: "给这条线索起个名字",
  lead: "下次在线索库里，你靠这个名字认出它。",
  /** 模型那次没回上来时这一句。**照实说**，候选里仍然有她自己的原话。 */
  failed: "名字建议生成失败。可以先用你自己写下的这一句。",
  /** 最后那个候选（从她原话裁出来的）底下的说明。 */
  ownWords: "你自己写的",
  skip: "先不起名",
  confirm: "确认名称",
  /** 起名那一步还在等模型时显示的字。 */
  loading: "正在拟名字",
} as const;
