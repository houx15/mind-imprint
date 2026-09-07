import type { InterestKeyword as ApiKeyword, InterestTree } from "../api/interest";
import type { FieldId, Keyword, KeywordSource } from "./types";

/**
 * liveTree — 把服务端的兴趣树接到那张画上。
 *
 * 原型里每个关键词的位置（`at.t` 沿枝的远近、`at.spread` 离枝的偏移）是**手摆**
 * 的，因为手摆才能让 mock 看起来是设计过的。真关键词不能手摆，所以位置必须算
 * 出来——而这个算法要同时满足两件事：
 *
 *  1. **同一棵树每次打开长得一样。** 一张每次刷新都换个样子的图，说不出「这就是
 *     你的模型」；它只是装饰。
 *  2. **同一根枝上的词不能叠在一起。** 纯哈希做不到这件事——哈希会撞，而撞了就
 *     是两个标签压在同一个像素上。
 *
 * 所以位置由**顺序**决定，不由哈希决定：同一根枝上的词按「第一次出现的时间」
 * 排序，均匀铺在 `[T_NEAR, T_FAR]` 上。哈希只用来抖动横向偏移，让它不像刻度尺。
 *
 * 这样得到的语义正好是对的：**老的兴趣靠近树干，新长出来的在梢头。** 一个新词
 * 到来会把同枝的词整体往里挪一点——枝就是这么长的，这不是抖动，这是生长。
 */

/** 沿枝的可用区间。太靠 0 会挤在树干上，太靠 1 会顶出梢外。 */
const T_NEAR = 0.3;
const T_FAR = 0.92;

/** 横向偏移的基准值。左右交替，幅度由哈希在 ±30% 内抖动。 */
const SPREAD_BASE = 52;

/**
 * 一个稳定的 32 位字符串哈希（FNV-1a）。
 *
 * 用它而不是 `Math.random`，是因为位置必须可复现；用它而不是把 id 当数字，
 * 是因为 id 是 uuid，相邻的 uuid 之间没有任何有意义的距离。
 */
export function hashString(s: string): number {
  let h = 0x811c9dc5;
  for (let i = 0; i < s.length; i += 1) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return h >>> 0;
}

/**
 * 把一根枝上的关键词铺开。
 *
 * `ordered` 必须已经按时间从早到晚排好。返回的数组与入参一一对应。
 */
export function placeOnBranch(ordered: { id: string }[]): { t: number; spread: number }[] {
  const n = ordered.length;
  return ordered.map((k, i) => {
    // 一根枝上只有一个词时放在中间偏外，而不是紧贴树干或吊在梢尖上。
    const ratio = n === 1 ? 0.55 : i / (n - 1);
    const t = T_NEAR + (T_FAR - T_NEAR) * ratio;
    const h = hashString(k.id);
    // ±30% 的幅度抖动，左右交替。交替保证相邻两个词不会挤在同一侧。
    const jitter = 0.7 + ((h % 61) / 60) * 0.6;
    const side = i % 2 === 0 ? 1 : -1;
    return { t, spread: SPREAD_BASE * jitter * side };
  });
}

/* ── 成长回放的那条轴 ───────────────────────────────────────────────────── */

const DAY = 86_400_000;

/**
 * 这条轴上有几个刻度。**由她的树活了多久决定，不是写死的四个。**
 *
 * 🚨 2026-09-07 改。上一版永远是四个刻度，标签写死成
 * 「起点 · 半年前 · 近两个月 · 现在」，只有位置是相对的。一个上周才开始的学生
 * 因此会看到一条写着「半年前」的轴 —— 那六个月不存在。产品负责人：
 *
 *   > we don't need to always show a 起点-半年前-近2个月-现在 timeline.
 *   > the timeline should not be fixed, it should be a relative one. for a new
 *   > student, we don't have that line. but when they came after some time, the
 *   > line gradually becomes longer. until this four-dot version.
 *
 * 所以刻度数是跨度的函数，标签由真实时间算出来（见 `growthStops`）：
 *
 *   三周以内   1 个 —— 只有现在，界面据此**不画这条轴**
 *   三个月以内 2 个 —— 起点 / 现在
 *   八个月以内 3 个
 *   再往上     4 个
 *
 * 三周这条线不是随手定的：短于三周，四个刻度之间相差几天，点哪个看到的都是
 * 同一棵树 —— 那是一个不动的控件，不是一段成长。
 */
export function stopCount(spanMs: number): number {
  if (spanMs < 21 * DAY) return 1;
  if (spanMs < 92 * DAY) return 2;
  if (spanMs < 240 * DAY) return 3;
  return 4;
}

/**
 * 每个刻度落在她那段跨度的哪个比例上。第一个永远是 0（起点），最后一个永远是
 * 1（现在）。
 */
export function stopPositions(count: number): number[] {
  switch (count) {
    case 1:
      return [1];
    case 2:
      return [0, 1];
    case 3:
      return [0, 0.55, 1];
    default:
      return [0, 0.4, 0.75, 1];
  }
}

/**
 * 把「第一次出现的时间」映射到那条轴上的一个刻度。
 *
 * 一个词属于**它出现时最近的那个刻度**（往前取）：刻度是一个时刻，词在那个时刻
 * 之后才有，所以从那一格起它就在树上。
 *
 * 最后一个刻度永远是「现在」，所以时间戳坏掉时退到它 —— 一个读不出时间的词
 * 仍然在她今天的树上。
 */
export function bornAtFor(firstSeenAt: string, earliest: number, now: number): number {
  const span = now - earliest;
  const stops = stopPositions(stopCount(span));
  const last = stops.length - 1;
  const t = Date.parse(firstSeenAt);
  if (!Number.isFinite(t)) return last;
  // 跨度太短，全部算在第一格上——把一天切成四段是假的精度。
  if (span <= 0) return 0;
  const ratio = (t - earliest) / span;
  let idx = 0;
  for (let i = 0; i < stops.length; i += 1) {
    if (ratio >= stops[i]!) idx = i;
  }
  return idx;
}

export interface GrowthStop {
  /** 轴上那个字：起点 / 4 个月前 / 现在。 */
  label: string;
  /** 它是哪一天。悬停时显示。 */
  sub: string;
}

/**
 * 这条轴上的几个刻度，标签由真实时间算出来。
 *
 * 中间那几格写的是「离今天多久」，不是一段编出来的情节。上一版的副标题是
 * 「读得多起来」「开始写」—— 我们并不知道她那两个月在做什么，那两句是替她写
 * 的故事。现在副标题是那一格对应的**日期**。
 */
export function growthStops(keywords: Keyword[], now: number = Date.now()): GrowthStop[] {
  const times = keywords
    .map((k) => Date.parse(k.firstSeenAt))
    .filter((n) => Number.isFinite(n));
  const earliest = times.length > 0 ? Math.min(...times) : now;
  const span = Math.max(0, now - earliest);
  const positions = stopPositions(stopCount(span));
  return positions.map((pos, i) => {
    const at = earliest + span * pos;
    if (i === positions.length - 1) return { label: "现在", sub: "今天" };
    return { label: i === 0 ? "起点" : agoLabel(now - at), sub: dayLabel(at) };
  });
}

/** 「多久以前」。天 → 周 → 月 → 年，只留一个单位。 */
function agoLabel(ms: number): string {
  const days = Math.max(1, Math.round(ms / DAY));
  if (days < 14) return `${days} 天前`;
  if (days < 60) return `${Math.round(days / 7)} 周前`;
  if (days < 730) return `${Math.max(1, Math.round(days / 30))} 个月前`;
  return `${Math.max(1, Math.round(days / 365))} 年前`;
}

/** `2026-03-14`。 */
function dayLabel(ms: number): string {
  const d = new Date(ms);
  const two = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${two(d.getMonth() + 1)}-${two(d.getDate())}`;
}

function toSources(k: ApiKeyword): KeywordSource[] {
  return k.sources.map((s) => ({
    kind: s.kind === "quiz" ? "course" : s.kind,
    id: s.refId,
    label: s.label,
    evidence: s.evidence,
    date: (s.happenedAt ?? "").slice(0, 10),
  }));
}

/**
 * API 的树 → 那张画要的 `Keyword[]`。
 *
 * `now` 可注入，因为一个读时钟的函数在午夜前后会给出不同结果，而测试需要它是
 * 确定的。
 */
export function toTreeKeywords(tree: InterestTree, now: number = Date.now()): Keyword[] {
  if (tree.keywords.length === 0) return [];

  const times = tree.keywords
    .map((k) => Date.parse(k.firstSeenAt))
    .filter((n) => Number.isFinite(n));
  const earliest = times.length > 0 ? Math.min(...times) : now;

  const byField = new Map<string, ApiKeyword[]>();
  for (const k of tree.keywords) {
    const list = byField.get(k.field) ?? [];
    list.push(k);
    byField.set(k.field, list);
  }

  const out: Keyword[] = [];
  for (const [field, list] of byField) {
    // 早的在里、晚的在外。同一毫秒的两个词按 id 定序，否则铺开的结果会随
    // 数组顺序漂移，而数组顺序来自 SQL，不保证稳定。
    const ordered = [...list].sort((a, b) => {
      const d = Date.parse(a.firstSeenAt) - Date.parse(b.firstSeenAt);
      return d !== 0 && Number.isFinite(d) ? d : a.id.localeCompare(b.id);
    });
    const places = placeOnBranch(ordered);
    ordered.forEach((k, i) => {
      out.push({
        id: k.id,
        interestId: k.interestId,
        text: k.textZh,
        en: k.textEn,
        field: field as FieldId,
        strength: Math.min(5, Math.max(1, k.strength)),
        bornAt: bornAtFor(k.firstSeenAt, earliest, now),
        firstSeenAt: k.firstSeenAt,
        note: k.note,
        sources: toSources(k),
        disciplineIds: (k.disciplines ?? []).map((d) => d.id),
        at: places[i]!,
      });
    });
  }
  return out;
}
