import type { QuizHook } from "../../api/interestQuiz";

/**
 * tree/quiz/content — 觉醒协议七屏的内容。
 *
 * 抽成一个纯数据模块，理由有两个：组件本身已经是一台状态机，把三百行文案摆在
 * 里面就没人读得下去；而这里的几条不变量（钩子 id 必须和后端对得上、压力测试
 * 必须**恰好**有一个正确项）**读代码看不出对错**，抽出来才测得到。
 *
 * 文案取自 `docs/reference/explore_interest.html`，两处按 AGENTS.md 的
 * 「界面文案怎么写」调过：按钮用书面词（「确认连接」不是「就这个」），
 * 标签是名词。世界观（2050 年、导航员、觉醒者）原样保留 —— 它是这一屏的
 * 全部魅力所在，而这一屏的任务就是让一个学生愿意花五分钟。
 */

/* ── 序章：行动原则 ─────────────────────────────────────────────────────── */

export interface Principle {
  key: string;
  index: string;
  title: string;
  body: string;
  /** true = 这是「把大脑交出去」的那一条。选它不扣分，只换一句不同的回应。 */
  risky: boolean;
}

export const PRINCIPLES: Principle[] = [
  {
    key: "outsource",
    index: "A",
    title: "把任务完全交给 AI",
    body: "让系统给出答案，我负责提交。",
    risky: true,
  },
  {
    key: "partner",
    index: "B",
    title: "让 AI 参与，但判断归我",
    body: "它负责搜索与初稿，我负责问题、证据与取舍。",
    risky: false,
  },
];

/** 选完原则之后，中枢回的那一句。选了 A 不是失败，是被提醒。 */
export const PRINCIPLE_REPLY: Record<string, string> = {
  outsource: "记录在案。多数人选这一条，他们的思维印记也因此长得一模一样。你随时可以改。",
  partner: "记录在案。这条路更慢，但留下的印记是你自己的。",
};

/* ── 导航员 ─────────────────────────────────────────────────────────────── */

export interface Navigator {
  /** 存进库里的名字 —— 她看见的就是这个。 */
  name: string;
  codename: string;
  label: string;
  body: string;
  quote: string;
  accent: string;
}

export const NAVIGATORS: Navigator[] = [
  {
    name: "热血同好",
    codename: "NOVA",
    label: "高能陪伴型",
    body: "善于发现亮点、快速建立信心，适合需要灵感与行动力的探索者。",
    quote: "一起去探索未知的领域吧！",
    accent: "#55e6ff",
  },
  {
    name: "腹黑军师",
    codename: "KIRO",
    label: "压力测试型",
    body: "喜欢抛出反方观点、追问证据，适合享受挑战与逻辑交锋的探索者。",
    quote: "希望你的脑子转得够快，别让我觉得无聊。",
    accent: "#9e8cff",
  },
  {
    name: "资深向导",
    codename: "SAGE",
    label: "沉稳启发型",
    body: "擅长拆解问题、提供工具卡，适合希望稳步建立方法的探索者。",
    quote: "我会为你提供线索，但路要你自己走。",
    accent: "#ffcd70",
  },
];

/* ── 兴趣锚点 ───────────────────────────────────────────────────────────── */

/** 快速示例。点一个只是**填进输入框**，她仍然要自己写理由。 */
export const WORK_EXAMPLES = [
  "《进击的巨人》里的利威尔",
  "《流浪地球》",
  "《哈利·波特》里的赫敏",
  "我喜欢做游戏",
];

/** 与服务端 interest.maxWorkRunes / maxReasonRunes 对齐。 */
export const MAX_WORK = 40;
export const MAX_REASON = 180;

/**
 * 服务端 `interest.minReasonRunes` 是 8。
 *
 * 🚨 前端**不拦**低于这个长度的提交。她写多短都能走完这七屏 —— 拦住她只会让
 * 她卡在第四屏，而她本来就在这里是因为她还不知道自己喜欢什么。写得短的结果是
 * 结果页少几个词，并请她再补一句，那是一句邀请，不是一道门。
 */
export const REASON_HINT_AT = 8;

/* ── 兴趣钩子 ───────────────────────────────────────────────────────────── */

export interface HookOption {
  /** 🚨 必须与 Go 侧 `interest.Hook` 的三个值逐字一致。 */
  key: QuizHook;
  index: string;
  question: string;
  body: string;
}

export const HOOKS: HookOption[] = [
  {
    key: "character",
    index: "A",
    question: "为什么这个人会变成现在这样？",
    body: "我会被人物的创伤、选择、关系与命运吸引。",
  },
  {
    key: "craft",
    index: "B",
    question: "这么厉害的画面或机制是怎么做出来的？",
    body: "我会注意运动、镜头、结构、技术和实现原理。",
  },
  {
    key: "society",
    index: "C",
    question: "为什么大家会这样评价它？",
    body: "我会关注流行、舆论、身份、偏见和群体心理。",
  },
];

/* ── 反方压力测试 ───────────────────────────────────────────────────────── */

export const CHALLENGE_PROMPT =
  "最后一关。我看到一条反方评论：「大家喜欢这个对象，只是因为它被反复推荐；" +
  "离开流行标签，本身并没有多少值得研究的内容。」你会怎样回应？";

export const CHALLENGE_RULE =
  "有效回应至少要做到两件事：给出你自己的判断，并指出一条可以回到作品、数据或具体细节里验证的证据。";

export interface ChallengeOption {
  key: string;
  index: string;
  title: string;
  body: string;
  correct: boolean;
  /** 选错时中枢回的那一句 —— 说清楚差在哪，不是「再试一次」。 */
  feedback: string;
}

export const CHALLENGE_OPTIONS: ChallengeOption[] = [
  {
    key: "popular",
    index: "A",
    title: "很多人都喜欢，当然说明它好。",
    body: "用流行程度证明内容价值。",
    correct: false,
    feedback: "这一条正好落进对方的陷阱：他说的就是「大家喜欢不等于值得研究」。再想一条能被查证的。",
  },
  {
    key: "emotion",
    index: "B",
    title: "反正我就是喜欢，不需要理由。",
    body: "坚持个人感受，但不提供可讨论的依据。",
    correct: false,
    feedback: "喜欢不需要理由，这句是对的。但它结束了对话——对方无法反驳，你也无法推进。",
  },
  {
    key: "evidence",
    index: "C",
    title: "先给出判断，再用具体片段或机制检验。",
    body: "把「我喜欢」转成可追问、可举证、可反驳的观点。",
    correct: true,
    feedback: "",
  },
];

/** 选对之后中枢回的那一句。 */
export const CHALLENGE_PASS =
  "记录完成。你没有在证明自己的喜好正确，而是把它变成了一个可以被检验的说法——这一步才是研究的起点。";

/* ── 屏 ─────────────────────────────────────────────────────────────────── */

export const STEPS = [
  "boot",
  "world",
  "navigator",
  "interest",
  "lens",
  "challenge",
  "result",
] as const;

export type Step = (typeof STEPS)[number];

/** 任务进度条只数三个 MISSION（锚点 / 钩子 / 压力测试），不数序章与结果。 */
export const MISSION_STEPS: Step[] = ["interest", "lens", "challenge"];

export function missionIndex(step: Step): number {
  return MISSION_STEPS.indexOf(step);
}

export function nextStep(step: Step): Step {
  const i = STEPS.indexOf(step);
  return STEPS[Math.min(i + 1, STEPS.length - 1)]!;
}

export function prevStep(step: Step): Step {
  const i = STEPS.indexOf(step);
  return STEPS[Math.max(i - 1, 0)]!;
}
