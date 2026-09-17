import type { WritingOutlineItem, WritingSnippet } from "../api/writingRoom";

/**
 * 段落那一步屏幕上的「块」—— 从结构图算出来。
 *
 * 结构图有三层（writingPlanMaxDepth）：0 = 开头 / 中心论点 / 落点，
 * 1 = 分论点，2 及以下 = 她自己的材料（例子、数据、经历）。
 *
 * 🚨 2026-09-18 写作入口走查：原来**每一个节点**都是一块，于是一个分论点下面
 * 挂的例子单独成了一段。两个中文学生都在同一处卡住：
 *   「第4块『晚自习查物理题』原本是用来支撑第2段理由的具体例子，现在单独成段了，
 *    这里可能有点重复」
 *   「第2块和第5块的内容有点重复，都是讲压力和朋友熬夜的事」
 * 材料是写进它上面那个分论点那一段里的东西，不是一段。所以材料节点并进上一块，
 * 在那一块里列成「可用的材料」。
 *
 * 两条老规矩不变（见 SnippetsStage 里 buildSlots 原来的注释）：
 *   1. 每一条存过的片段都有一块 —— 包括写在材料节点上的老片段；
 *   2. 片段只按 outlineId 对到块上，从不按位置。
 *
 * 块的编号是它在屏幕上的顺序（1 起），服务端 writingBlockNumbers 用同一条规则，
 * 所以 印记 说的「第 N 块」和屏幕上的号对得上。
 */

/** Depth at and below which an outline node is her material, not a paragraph. */
export const MATERIAL_DEPTH = 2;

export type Slot = {
  /** 1-based, in screen order. */
  number: number;
  position: number;
  outlineId: string | null;
  heading: string;
  /** The generic block label from the skeleton, when this slot comes from one. */
  role: string;
  snippet: WritingSnippet | null;
  /** Material nodes planned under this block (depth ≥ 2), in map order. */
  materials: string[];
};

export function buildSlots(outline: WritingOutlineItem[], snippets: WritingSnippet[]): Slot[] {
  const sortedOutline = outline.slice().sort((a, b) => a.position - b.position);
  const outlineIds = new Set(sortedOutline.map((o) => o.id));
  const snippetOf = (id: string) => snippets.find((s) => s.outlineId === id) ?? null;

  const slots: Slot[] = [];
  let parent: Slot | null = null;
  for (const o of sortedOutline) {
    const snippet = snippetOf(o.id);
    const isMaterial = o.depth >= MATERIAL_DEPTH;
    // A material node folds into the paragraph above it — unless she already
    // wrote a paragraph on it, which must stay visible and editable.
    if (isMaterial && !(snippet && snippet.text.trim() !== "") && parent) {
      if (o.text.trim()) parent.materials.push(o.text.trim());
      continue;
    }
    const slot: Slot = {
      number: 0,
      position: o.position,
      outlineId: o.id,
      // Her own sentence is the heading when she has written one; the generic
      // role is the fallback.
      heading: o.text.trim() || o.role,
      role: o.role,
      snippet,
      materials: [],
    };
    slots.push(slot);
    if (!isMaterial) parent = slot;
  }

  const free = snippets
    .filter((s) => !s.outlineId || !outlineIds.has(s.outlineId))
    .slice()
    .sort((a, b) => a.position - b.position);
  for (const s of free) {
    slots.push({
      number: 0,
      position: s.position,
      outlineId: s.outlineId,
      heading: s.outlineHeading,
      role: "",
      snippet: s,
      materials: [],
    });
  }

  slots.forEach((s, i) => {
    s.number = i + 1;
  });
  return slots;
}
