import { apiFetch } from "./client";
import type { ToneName } from "../shared/tone";

// api/lookback.ts —— 复盘，和上线之后。
// 形状读自 apps/api/internal/api/pbl_lookback.go 和 pbl_keep.go。

const base = (id: string) => `/api/v1/pbl/projects/${id}`;

/**
 * 她现在还这么想吗。
 *
 * 🚨 「当时没想清楚」必须是可点的一项：在空白框里承认这件事要写一段话，成本
 * 太高，她于是写「挺好的」。一下点掉，诚实就变便宜了（铁律④）。
 */
export type Stance = "" | "still" | "changed" | "unclear";

export const STANCES: { key: Exclude<Stance, "">; label: string }[] = [
  { key: "still", label: "现在仍这么想" },
  { key: "changed", label: "现在会改" },
  { key: "unclear", label: "当时没想清楚" },
];

export type ReviewSection = "what" | "how" | "moment" | "praise" | "improve" | "with_ai";

/**
 * 复盘的六段（产品负责人 2026-09-02）。段是固定的，段里的问题由印记按这个项目
 * 真发生过的事现写——见 apps/api/internal/pbl/lookback.go。
 */
export const REVIEW_SECTIONS: { key: ReviewSection; title: string; hue: ToneName }[] = [
  { key: "what", title: "做了什么", hue: "mist" },
  { key: "how", title: "感受如何", hue: "taro" },
  { key: "moment", title: "印象最深的一件事", hue: "peach" },
  { key: "praise", title: "值得肯定的地方", hue: "matcha" },
  { key: "improve", title: "还能更好的地方", hue: "berry" },
  { key: "with_ai", title: "和 AI 的协作", hue: "lake" },
];

/**
 * 「感受如何」那一段的几个词。
 *
 * 🚨 产品负责人 2026-09-03：「not just typing texts, but different hints,
 * clickable or draggable or selectable」。六段全是空白文本框，最难下笔的就是
 * 感受这一段——问一个中学生"你感觉如何"，她面对的是一个空框和一个不知道该
 * 多正式的期待。点一个词起头，她接着往下写就容易多了。
 *
 * 用的是真实的过程感受，不是"很有收获"这种交差话——铁律④要的是真信号。
 */
export const FEELING_WORDS = [
  "比想象中顺",
  "中间卡住过",
  "一开始没头绪",
  "比想象中难",
  "出乎意料",
  "有点枯燥",
  "越做越有意思",
  "时间不够用",
] as const;

/** 一段答完了几题。段头上那个 n/m 用它。 */
export function sectionProgress(prompts: LookbackPrompt[]): { done: number; total: number } {
  return {
    done: prompts.filter((p) => p.answer.trim() !== "").length,
    total: prompts.length,
  };
}

export interface LookbackPrompt {
  revision?: number;
  id: string;
  section: ReviewSection;
  prompt: string;
  answer: string;
  /** 这一问对应的项目记录，可能来自学生、计划或系统回执。空 = 感受问题。 */
  evidence: string;
  /** 她现在怎么看当初那句话。空 = 还没表态。 */
  stance: Stance;
  ordinal: number;
}

/** 按六段分组，空段不出现。 */
export function bySection(
  prompts: LookbackPrompt[],
): { key: ReviewSection; title: string; hue: ToneName; prompts: LookbackPrompt[] }[] {
  return REVIEW_SECTIONS.map((s) => ({
    ...s,
    prompts: prompts.filter((p) => p.section === s.key),
  })).filter((g) => g.prompts.length > 0);
}

export function getLookback(projectId: string): Promise<LookbackPrompt[]> {
  return apiFetch<LookbackPrompt[]>(`${base(projectId)}/lookback`);
}

export function regenerateLookback(projectId: string): Promise<LookbackPrompt[]> {
  return apiFetch(`${base(projectId)}/lookback/regenerate`, { method: "POST" });
}

export function answerLookback(
  projectId: string,
  promptId: string,
  answer: string,
): Promise<LookbackPrompt> {
  return apiFetch<LookbackPrompt>(`${base(projectId)}/lookback/${promptId}`, {
    method: "PATCH",
    body: JSON.stringify({ answer }),
  });
}

/**
 * 她对当初那句话现在的看法。
 *
 * 🚨 和 answerLookback 分开发：她先点态度、后写字（或者反过来）都不该把另一样
 * 清掉，所以这一枪只带 stance，服务端也只在带了这一格时才动它。
 */
export function setStance(
  projectId: string,
  promptId: string,
  stance: Stance,
): Promise<LookbackPrompt> {
  return apiFetch<LookbackPrompt>(`${base(projectId)}/lookback/${promptId}`, {
    method: "PATCH",
    body: JSON.stringify({ stance }),
  });
}

/**
 * 还差什么才算复盘过。
 *
 * 🚨 一条都没写才算没做。
 *
 * 原来要八问全答才准点完成，底下挂着「还有 5 问没答」。八个输入框加一个进度
 * 计数，就是一份作业——而她敷衍着填满八格，比认真写透两格差得多。复盘本来
 * 就是隔几天回来慢慢写的，答过的都留着（服务端只生成一次）。
 */
export function lookbackTodo(prompts: LookbackPrompt[]): string {
  // 🚨 还没生成出来时也要拦住「完成」。
  //
  // 原来这里返回空串——空串的意思是「齐了，可以收工」，于是印记读项目、写问题
  // 的那两三分钟里，「完成」一直是亮的。她点下去，就把一次一道题都没有的复盘
  // 交掉了，回灌带给印记的是 answered: 0。整件工具最后一步，恰恰在它还没有内容
  // 的时候敞着。
  if (prompts.length === 0) return "复盘问题还在生成";
  return prompts.some((p) => p.answer.trim()) ? "" : "至少写下一条";
}

/* ── 上线之后 ───────────────────────────────────────────────────────────── */

export type KeepKind = "stat" | "feedback" | "thought";
export type KeepStage = "ship" | "observe" | "interpret" | "change";

export interface KeepEntry {
  id: string;
  kind: KeepKind;
  body: string;
  stage: KeepStage;
  /** 一个数字、它的单位，和上一次是多少。数字的意思在变化里，不在数值里。 */
  metric: string;
  value: number | null;
  prev: number | null;
  unit: string;
  /** 改这一件事时她的预期，以及后来兑现了没有。 */
  expect: string;
  verdict: "" | "met" | "missed";
  /** 这条数据长出来的那一轮思考。 */
  sessionId: string | null;
  createdAt: string;
}

/** 循环里的四步。颜色是为了让她一眼看出自己停在哪一步。 */
export const KEEP_STAGES: { stage: KeepStage; label: string; hint: string; hue: string }[] = [
  { stage: "ship", label: "发布上线", hint: "让真实用户开始使用你的成果", hue: "var(--mk-mist)" },
  { stage: "observe", label: "收集数据", hint: "整理访谈、问卷、后台数据，拿到真实反馈", hue: "var(--mk-matcha)" },
  { stage: "interpret", label: "数据分析", hint: "从这些数据里提取对成果有用的洞察", hue: "var(--mk-peach)" },
  { stage: "change", label: "产品迭代", hint: "形成迭代方案，并把它做出来", hue: "var(--mk-taro)" },
];

export const KEEP_KINDS: { kind: KeepKind; label: string; placeholder: string }[] = [
  { kind: "stat", label: "产品数据", placeholder: "这周的一个数字，以及它是从哪看到的" },
  { kind: "feedback", label: "访谈 / 调查结果", placeholder: "针对谁的访谈，ta 表达了什么？" },
  { kind: "thought", label: "我的灵感", placeholder: "你的灵感是什么，为什么？" },
];

export interface KeepMetric {
  name: string;
  what: string;
}

/**
 * 每个项目都用得上的三种取数方式。
 *
 * 不管她做的是一个网站、一场活动还是一次调查，「有多少人用过」「问一批人」
 * 「找几个人聊」这三件事都成立，而且都是真正的专业词（AGENTS.md 界面文案 6）。
 */
const KEEP_METRICS_ANY: KeepMetric[] = [
  { name: "使用人数", what: "一段时间里真正用过它的有多少人。先有这个数，别的比例才有意义。" },
  { name: "问卷调查", what: "同一组问题发给一批人，看回答的分布。人数够了，个别意见才不会盖过整体。" },
  { name: "访谈记录", what: "找几个用过的人当面聊：他当时想做什么、卡在哪一步、最后怎么解决的。" },
];

/**
 * 只有屏幕上的东西才量得出来的三个。
 *
 * 产品负责人 2026-09-02：「停留时长、点击率、留存率(with each can click to see
 * what are they)」。直接甩三个词给一个中学生，等于没说——她多半会跳过这一档，
 * 而这一档恰恰是"用数据说话"这件事的入口。
 */
const KEEP_METRICS_SCREEN: KeepMetric[] = [
  { name: "停留时长", what: "一个人打开之后待了多久。太短通常说明他没找到想看的东西。" },
  { name: "点击率", what: "看到入口的人里，有多少真的点进来了。" },
  { name: "留存率", what: "上次来过的人，这次还回来的比例。" },
];

/**
 * 现场做的事，量的是现场发生了什么。
 */
const KEEP_METRICS_SCENE: KeepMetric[] = [
  { name: "参与率", what: "在场的人里，实际参加进来的比例。" },
  { name: "现场计数", what: "同一个时间、同一个位置数三天，数出来的才是常态，不是那一天。" },
];

/** 屏幕上的项目——这几类才有点击率、留存率可言。 */
const SCREEN_KINDS = new Set(["网站搭建", "产品原型", "内容设计", "website", "making", "design"]);
/** 线下发生的项目。 */
const SCENE_KINDS = new Set(["田野调查", "活动策划", "investigation"]);

/**
 * 这个项目该看哪几种数据。
 *
 * 🚨 原来这里是写死的三个：停留时长 / 点击率 / 留存率。2026-09-02 线上实测走
 * 的是一个走廊上的项目——她要改的是课间十分钟，界面却请她填「点击率」。一个
 * 量不出来的指标不只是没用，它在教她「这一步跟我的项目无关」，而这一步恰恰是
 * 整个长期迭代的入口。
 *
 * 类别还没定（刚建的项目 kind 是空的）就只给通用那三种——问卷、访谈、人数对
 * 任何项目都成立，不会闹笑话。
 */
export function keepMetricsFor(kind: string): KeepMetric[] {
  const k = kind.trim();
  if (SCREEN_KINDS.has(k)) return [...KEEP_METRICS_SCREEN, ...KEEP_METRICS_ANY];
  if (SCENE_KINDS.has(k)) return [...KEEP_METRICS_SCENE, ...KEEP_METRICS_ANY];
  return KEEP_METRICS_ANY;
}

export function listKeepEntries(projectId: string): Promise<KeepEntry[]> {
  return apiFetch<KeepEntry[]>(`${base(projectId)}/keep`);
}

export function addKeepEntry(
  projectId: string,
  body: {
    kind: KeepKind;
    body: string;
    stage: KeepStage;
    metric?: string;
    value?: number;
    unit?: string;
    expect?: string;
  },
): Promise<KeepEntry> {
  return apiFetch<KeepEntry>(`${base(projectId)}/keep`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

/** 让这一条长出一轮新的思考。已经有的话，回到原来那一轮。 */
export function openKeepSession(
  projectId: string,
  entryId: string,
): Promise<{ sessionId: string }> {
  return apiFetch<{ sessionId: string }>(`${base(projectId)}/keep/${entryId}/session`, {
    method: "POST",
  });
}

/**
 * 她走到循环的哪一步了。
 *
 * 「改一件事」之后回到「放出去」——这是一个圈，不是一条队列。停在"看数据"上
 * 三个月和还没上线是两种完全不同的状态，而这行字是唯一说得出区别的地方。
 */
export function keepStage(entries: KeepEntry[]): KeepStage {
  if (entries.length === 0) return "ship";
  const latest = entries[0]!; // 服务端按时间倒序
  if (latest.stage === "change") return "ship";
  return latest.stage;
}

/**
 * 她转过几圈了。
 *
 * 🚨 迭代的意思是**重复**。一张勾一次就完的清单不是迭代——四个阶段走完一轮，
 * 下一轮从头再来，圈数才是这件工具真正要她看见的东西。
 *
 * 一圈的界线是「产品迭代」那一步：改完一件事，就该回到发布、再看数据。所以每
 * 出现一条 change，就算走完一圈。
 *
 * 纯派生，不进库——阶段和时间本来就都记着了。
 */
export function keepLaps(entries: KeepEntry[]): number {
  return entries.filter((e) => e.stage === "change").length;
}

/**
 * 一次改动的预期后来兑现了没有。
 *
 * 🚨 **没兑现才是最值钱的那一次**：它说明她原来想错了，而那正是迭代要教的东西。
 * 所以这里不庆祝兑现、也不惩罚没兑现，只是记下来。
 */
export function settlePrediction(
  projectId: string,
  entryId: string,
  verdict: "met" | "missed",
): Promise<KeepEntry> {
  return apiFetch<KeepEntry>(`${base(projectId)}/keep/${entryId}/settle`, {
    method: "POST",
    body: JSON.stringify({ verdict }),
  });
}

/** 变化，不是数值。没有上一次就是 null——那时候界面照实说「首次记录」。 */
export function keepDelta(e: KeepEntry): number | null {
  if (e.value === null || e.prev === null) return null;
  return e.value - e.prev;
}
