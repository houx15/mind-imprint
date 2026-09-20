import type { WritingOutlineItem, WritingSnippet } from "../api/writingRoom";
import { outlineKindIsInlineContent, outlineKindOf } from "./outlineKind";

/**
 * 段落那一步的一叠卡片 —— 从结构图派生出来的写作结构。
 *
 *   开头（提出中心论点） → 每条分论点一张（它下面的例子是这一段的材料） → 结尾
 *
 * 🚨 2026-09-18 产品负责人：「我有两个例子，结果就变成了两段。应该根据例子设计
 * 一个写作结构，而不是把例子直接变成段落。」原来是**每个节点一块**：规划时例子
 * 常常直接挂在中心论点下面，于是两个例子 = 两块，没有开头也没有结尾。
 *
 * 从标准篇章骨架（总—分—总）派生卡片是确定性的系统步骤（AGENTS.md 铁律适用边界）；
 * 卡片上的字只有她的节点文字和骨架的名字（开头 / 结尾），一个字都不是 AI 写的。
 *
 * 规则和服务端 writing_blocks.go 的 writingCards 逐条一致（编号也是），
 * 所以 印记 说的「第 N 块」和屏幕上的号对得上：
 *   - kind 是 opening → 开头卡；closing → 结尾卡；thesis 是中心论点，
 *     没有单独的开篇节点时开头卡就绑在它上面。
 *   - kind 是 point / counter → 各一张主体卡。
 *   - 材料（evidence / reference / reasoning / rebuttal / gap）并进前面最近的主体卡；
 *     前面还没有主体卡的例子自己成一张主体卡（`needsPoint`：这一段要先说清
 *     它证明了什么）。
 *   🚨 **一条都不看深度。** 深度曾经是这里的判据，而它是模型定的 ——
 *     一个被挂到深度 1 的结尾因此被印成「分论点 3」（同事 2026-09-20 意见 3）。
 *   - 她已经在某个节点上写过字的，那个节点一定有自己的卡。
 *   - 没有结尾节点就补一张虚拟结尾卡（片段存在 CLOSING_POSITION）；没有可绑定的
 *     开头节点就补虚拟开头卡（OPENING_POSITION）。
 *   - 片段只按 outlineId 对到卡上，从不按位置；没挂在当前结构图上的排在最后。
 */

/** 虚拟开头 / 结尾卡的片段位置。和 writing_blocks.go 的保留位置一致；
 *  结构图的位置是 0..N-1，自由段落从 1000 起，两边都够不着。 */
export const OPENING_POSITION = 998;
export const CLOSING_POSITION = 999;

export type SlotKind = "opening" | "point" | "closing" | "free";

export type Slot = {
  /** 1-based, in screen order. */
  number: number;
  kind: SlotKind;
  position: number;
  outlineId: string | null;
  /** Her node text; empty for a virtual card and for an example-only card. */
  heading: string;
  role: string;
  snippet: WritingSnippet | null;
  /** Material planned under this card, in map order. */
  materials: string[];
  /** 中心论点 —— 开头要提出、结尾要回到的那句话（只在开头 / 结尾卡上）。 */
  claim: string;
  /** 这张卡只有例子、还没有一句分论点：这一段要先说清例子证明了什么。 */
  needsPoint: boolean;
  /**
   * 这张卡底下那个结构图节点的 kind（闭表，见 outlineKind.ts）。
   *
   * 比 `kind` 细：卡片那一层把反方观点和分论点都画成「主体卡」，而段落引导
   * 里「这一段里的几步」对这两种是不一样的（一个要承认再转，一个要分论点
   * 句 → 论据 → 分析 → 回扣）。虚拟卡和自由段落没有节点，留空。
   */
  outlineKind: string;
};

/**
 * 🚨 2026-09-20：这里原来有三张关键词表（EXAMPLE_ROLE_WORDS /
 * OPENING_ROLE_WORDS / CLOSING_ROLE_WORDS）和两个靠它们猜的函数。
 * 那套猜法有一个静默的失败：它只在 `depth === 0` 时认结尾，于是一个被模型挂到
 * 深度 1 的结尾掉进最后那个 else，被印成「分论点 3」（同事的意见 3）。
 *
 * 现在节点自己带着 kind（服务端的闭表，见 outlineKind.ts），这里不再猜。
 */

/** 材料：写进某一段里的东西（例子、数据、道理、待补），而不是自己成为一段。 */
export function nodeIsMaterial(o: WritingOutlineItem): boolean {
  return outlineKindIsInlineContent(outlineKindOf(o));
}

export function buildSlots(outline: WritingOutlineItem[], snippets: WritingSnippet[]): Slot[] {
  const sorted = outline.slice().sort((a, b) => a.position - b.position);
  const inOutline = new Set(sorted.map((o) => o.id));
  const snippetOf = (id: string) => snippets.find((s) => s.outlineId === id) ?? null;
  const written = (o: WritingOutlineItem) => (snippetOf(o.id)?.text ?? "").trim() !== "";

  let openingFree: WritingSnippet | null = null;
  let closingFree: WritingSnippet | null = null;
  let free: WritingSnippet[] = [];
  for (const s of snippets) {
    if (s.outlineId && inOutline.has(s.outlineId)) continue;
    if (!s.outlineId && s.position === OPENING_POSITION && !openingFree) {
      openingFree = s;
      continue;
    }
    if (!s.outlineId && s.position === CLOSING_POSITION && !closingFree) {
      closingFree = s;
      continue;
    }
    free.push(s);
  }
  free.sort((a, b) => a.position - b.position);

  const card = (kind: SlotKind, o: WritingOutlineItem, over: Partial<Slot> = {}): Slot => ({
    number: 0,
    kind,
    position: o.position,
    outlineId: o.id,
    heading: o.text.trim() || o.role,
    role: o.role,
    snippet: snippetOf(o.id),
    materials: [],
    claim: "",
    needsPoint: false,
    outlineKind: outlineKindOf(o),
    ...over,
  });
  const virtual = (kind: "opening" | "closing", snippet: WritingSnippet | null, claim: string): Slot => ({
    number: 0,
    kind,
    position: kind === "opening" ? OPENING_POSITION : CLOSING_POSITION,
    outlineId: null,
    heading: "",
    role: "",
    snippet,
    materials: [],
    claim,
    needsPoint: false,
    // 虚拟的开头 / 结尾卡没有节点，但它们的活是确定的。
    outlineKind: kind,
  });

  const slots: Slot[] = [];
  if (sorted.length > 0) {
    // 🚨 **按 kind 找，不按深度。** 原来这里有一个 `o.depth !== 0` 的闸，
    // 于是一个被挂到深度 1 的开篇或结尾根本进不了这个循环。
    let openingNode: WritingOutlineItem | null = null;
    let thesisNode: WritingOutlineItem | null = null;
    for (const o of sorted) {
      const k = outlineKindOf(o);
      if (k === "opening") openingNode ??= o;
      else if (k === "thesis") thesisNode ??= o;
    }
    const claim = thesisNode?.text.trim() ?? "";
    const opening: Slot = openingNode
      ? card("opening", openingNode, { claim })
      : thesisNode
        ? // 中心论点就是开头要提出的那句话：卡片的标题是「开头」，那句话放在 claim。
          card("opening", thesisNode, { heading: "", claim })
        : virtual("opening", openingFree, "");
    slots.push(opening);

    const closings: Slot[] = [];
    let lastBody: Slot | null = null;
    for (const o of sorted) {
      if (opening.outlineId === o.id) continue;
      if (outlineKindOf(o) === "opening") {
        if (written(o)) slots.push(card("opening", o));
        else if (o.text.trim()) opening.materials.push(o.text.trim());
      } else if (outlineKindOf(o) === "closing") {
        closings.push(card("closing", o, { claim }));
      } else if (thesisNode && o.id === thesisNode.id) {
        if (written(o)) slots.push(card("point", o));
      } else if (nodeIsMaterial(o)) {
        if (written(o)) {
          const s = card("point", o);
          slots.push(s);
          lastBody = s;
        } else if (lastBody) {
          if (o.text.trim()) lastBody.materials.push(o.text.trim());
        } else {
          const s = card("point", o, { heading: "", needsPoint: true, materials: o.text.trim() ? [o.text.trim()] : [] });
          slots.push(s);
          lastBody = s;
        }
      } else {
        const s = card("point", o);
        slots.push(s);
        lastBody = s;
      }
    }
    if (closings.length === 0) closings.push(virtual("closing", closingFree, claim));
    else if (closingFree) free = [closingFree, ...free];
    slots.push(...closings);
    if (opening.outlineId && openingFree) free = [openingFree, ...free];
  } else {
    for (const s of [openingFree, closingFree]) if (s) free.push(s);
    free.sort((a, b) => a.position - b.position);
  }

  for (const s of free) {
    slots.push({
      number: 0,
      kind: "free",
      position: s.position,
      outlineId: s.outlineId,
      heading: s.outlineHeading,
      role: "",
      snippet: s,
      materials: [],
      claim: "",
      needsPoint: false,
      outlineKind: "",
    });
  }

  slots.forEach((s, i) => {
    s.number = i + 1;
  });
  return slots;
}

/** 卡片的名字：屏幕上、卡片叠里都用这一个。 */
export function slotTitle(s: Slot, pointIndex: number): string {
  switch (s.kind) {
    case "opening":
      return "开头";
    case "closing":
      return "结尾";
    case "free":
      return "自由段落";
    default:
      return `分论点 ${pointIndex}`;
  }
}
