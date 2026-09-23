import type { WritingOutlineItem } from "../api/writingRoom";
import type { WritingGenre } from "./outlineKind";
import { OUTLINE_KIND_DEPTH, outlineKindLabel, outlineKindOf, type OutlineKind } from "./outlineKind";

/**
 * 她自己改一个节点「是什么」。
 *
 * # 为什么要有这条路（同事 2026-09-22 的意见 3）
 *
 *	「如图，黑心商家那个点感觉应该是和分论点并列的一个反面论证，而不是论据。
 *	  AI 也没有提醒学生进行修改，只能靠学生自己判断、修改。
 *	  希望 AI 能够正确地识别出学生给的是分论点还是补充的案例还是反面论点，
 *	  或者学生自己辨别错误的时候，ai 能给出提示。」
 *
 * 两半。这个文件是后一半：**她自己改得动**。
 *
 * 🚨 在这之前她改不动。图上能拖，而拖动只改深度 —— `rekindForDepth` 把任何
 * 拖到深度 1 的东西一律变成 `point`。也就是说「反方观点」这一种**根本到不了**：
 * 同事那条「黑心商家哪怕赚很多钱，也是失败」拖上去会变成分论点，那也是错的。
 * 闭表里十种，她碰得到的只有三种。
 *
 * # 它不新建接口
 *
 * `PUT /outline` 本来就是「拿一整份新的扁平清单覆盖旧的」，而服务端按 kind
 * 重算深度和父节点（writing_kind.go）。所以这里只要算出一份**服务端认得出**
 * 的清单就行：把那一行的 kind 和 depth 一起改掉，位置不动。
 *
 * # 🚨 挂不上就拒绝，不悄悄挂到别处
 *
 * 服务端的 `writingKindParentOf` 找不到该挂的那一种父节点时返回 nil ——
 * 一条没有分论点可挂的论据会掉到最上层去，被下游当成一条理由（那正是
 * 2026-09-18 记下的那个毛病）。所以这里先算一遍：该有的父节点不在它前面，
 * 就返回一句话，让调用方说给她听，而不是让她看着那一下「成功了」然后图变形。
 */

/** 每一种块要挂在哪一种上面。和 Go 侧 writingKindParentOf 逐条一致。 */
const PARENT_KIND: Partial<Record<OutlineKind, OutlineKind[]>> = {
  point: ["thesis"],
  counter: ["thesis"],
  evidence: ["point"],
  reference: ["point"],
  reasoning: ["point"],
  gap: ["point"],
  rebuttal: ["counter"],
  scene: ["opening"],
  turn: ["opening"],
  feeling: ["opening"],
  detail: ["scene", "turn"],
};

export type RekindResult =
  | { ok: true; items: WritingOutlineItem[] }
  | { ok: false; why: string };

export function rekindOutlineNode(
  items: WritingOutlineItem[],
  id: string,
  kind: OutlineKind,
): RekindResult {
  const rows = items.slice().sort((a, b) => a.position - b.position);
  const at = rows.findIndex((r) => r.id === id);
  if (at < 0) return { ok: false, why: "修改失败：该条目已不存在。" };
  const row = rows[at];
  if (!row) return { ok: false, why: "修改失败：该条目已不存在。" };
  if (outlineKindOf(row) === kind) return { ok: true, items };

  // 一篇只有一个中心论点。
  if (kind === "thesis") {
    const other = rows.find((r) => r.id !== id && outlineKindOf(r) === "thesis");
    if (other) {
      return {
        ok: false,
        why: `修改失败：已经有中心论点「${other.text}」。请先修改或删除原有中心论点。`,
      };
    }
  }

  // 该挂的那一种父节点得在它前面。
  const parents = PARENT_KIND[kind];
  if (parents && parents.length > 0) {
    const has = rows
      .slice(0, at)
      .some((r) => parents.includes(outlineKindOf(r)));
    if (!has) {
      return {
        ok: false,
        why: `修改失败：「${outlineKindLabel(kind)}」需要归属于${parents
          .map((k) => `「${outlineKindLabel(k)}」`)
          .join("或")}，请先在它之前添加对应条目。`,
      };
    }
  }

  // 它自己底下的东西不能被压到深度上限以外。
  const depth = OUTLINE_KIND_DEPTH[kind];
  let end = at + 1;
  while (end < rows.length && (rows[end]?.depth ?? -1) > row.depth) end++;
  const delta = depth - row.depth;
  const deepest = Math.max(...rows.slice(at, end).map((r) => r.depth)) + delta;
  if (deepest > 2) {
    return {
      ok: false,
      why: `修改失败：改为「${outlineKindLabel(kind)}」会超出可用的内容层级。请先调整其下属条目的位置。`,
    };
  }

  const next = rows.map((r, i) => {
    if (i < at || i >= end) return r;
    const d = r.depth + delta;
    // 自己换成她挑的那一种；底下的跟着平移，种类不变（一条论据换了爸爸还是论据）。
    return { ...r, depth: d, kind: i === at ? kind : outlineKindOf(r) };
  });
  return { ok: true, items: next.map((r, i) => ({ ...r, position: i })) };
}

/**
 * 摆给她挑的那几种，按文体。
 *
 * 顺序是按「最常摆错的排前面」定的：同事那张图里错的正是论据和反方观点之间
 * 那一刀。开篇和结尾排在最后 —— 它们几乎不会被认错。
 */
export function rekindChoices(genre: WritingGenre): OutlineKind[] {
  // 散文和记叙文共用那一套块 —— 差别在整篇怎么合起来，不在某一段是什么。
  if (genre === "narrative" || genre === "prose") {
    return ["scene", "detail", "turn", "feeling", "opening", "closing"];
  }
  // 书信：一封信里没有分论点，也没有中心论点。
  if (genre === "letter") {
    return ["matter", "purpose", "courtesy", "opening", "closing"];
  }
  return [
    "point",
    "counter",
    "evidence",
    "reference",
    "reasoning",
    "rebuttal",
    "gap",
    "thesis",
    "opening",
    "closing",
  ];
}
