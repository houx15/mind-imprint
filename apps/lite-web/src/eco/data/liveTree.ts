import type { InterestKeyword as ApiKeyword, InterestTree } from "../../api/interest";
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

/**
 * 把「第一次出现的时间」映射到成长回放的四个刻度。
 *
 * 🚨 刻度是**相对她自己的起点**的，不是绝对月份。一个昨天注册的学生不该被展示
 * 一段五个月的成长故事——那不是她的（`data/tree.ts` 的 GROWTH_STOPS 里记着这条
 * 教训）。所以这里用「她最早的那个词到今天」的跨度去分四段。
 *
 * 索引 3 永远是「现在」：总有一个现在。
 */
export function bornAtFor(firstSeenAt: string, earliest: number, now: number): number {
  const t = Date.parse(firstSeenAt);
  if (!Number.isFinite(t)) return 3;
  const span = now - earliest;
  // 跨度太短（一天之内），全部算在起点上——把一天切成四段是假的精度。
  if (span <= 0) return 0;
  const ratio = (t - earliest) / span;
  if (ratio < 0.25) return 0;
  if (ratio < 0.6) return 1;
  if (ratio < 0.9) return 2;
  return 3;
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
        text: k.textZh,
        en: k.textEn,
        field: field as FieldId,
        strength: Math.min(5, Math.max(1, k.strength)),
        bornAt: bornAtFor(k.firstSeenAt, earliest, now),
        note: k.note,
        sources: toSources(k),
        at: places[i]!,
      });
    });
  }
  return out;
}
