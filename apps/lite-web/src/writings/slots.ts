import type { WritingOutlineItem, WritingSnippet } from "../api/writingRoom";

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
 *   - 深度 0：开头类 role → 开头卡；结尾类 role → 结尾卡；第一个其余节点是中心论点，
 *     没有单独的开头节点时开头卡就绑在它上面；再有别的深度 0 节点，各自一张主体卡。
 *   - 深度 1 的分论点 → 一张主体卡。
 *   - 材料（深度 ≥ 2，或挂在上层的例子）并进前面最近的主体卡；前面还没有主体卡的
 *     例子自己成一张主体卡（`needsPoint`：这一段要先说清它证明了什么）。
 *   - 她已经在某个节点上写过字的，那个节点一定有自己的卡。
 *   - 没有结尾节点就补一张虚拟结尾卡（片段存在 CLOSING_POSITION）；没有可绑定的
 *     开头节点就补虚拟开头卡（OPENING_POSITION）。
 *   - 片段只按 outlineId 对到卡上，从不按位置；没挂在当前结构图上的排在最后。
 */

/** Depth at and below which an outline node is her material, not a paragraph. */
export const MATERIAL_DEPTH = 2;
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
};

const EXAMPLE_ROLE_WORDS = [
  "例", "经历", "的事", "事件", "故事", "材料", "数据", "研究", "报道", "访谈", "调查",
  "案例", "引用", "名言", "人物", "史实", "素材", "证据", "新闻", "实验", "统计", "场景", "现象",
  "example", "experience", "evidence", "data", "study", "research", "report",
  "story", "quote", "case", "survey", "statistic", "source",
];
const OPENING_ROLE_WORDS = ["开头", "引言", "开篇", "钩子", "导入", "opening", "hook", "introduction", "intro"];
const CLOSING_ROLE_WORDS = ["结尾", "结论", "总结", "收尾", "落点", "结语", "closing", "conclusion", "ending"];

const hasAny = (role: string, words: string[]) => {
  const r = role.toLowerCase();
  return words.some((w) => r.includes(w));
};

/** 这个节点的 role 说的是一份材料（例子、经历、数据……），不是一条分论点。 */
export function roleIsExample(role: string, source?: string): boolean {
  if ((source ?? "").trim() !== "") return true;
  return hasAny(role, EXAMPLE_ROLE_WORDS);
}

/** 材料：深度 ≥ 2，或者 role 说它是例子（挂在中心论点下面、甚至落在最上层的）。 */
export function nodeIsMaterial(o: WritingOutlineItem): boolean {
  if (o.depth >= MATERIAL_DEPTH) return true;
  return roleIsExample(o.role, o.source);
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
  });

  const slots: Slot[] = [];
  if (sorted.length > 0) {
    let openingNode: WritingOutlineItem | null = null;
    let thesisNode: WritingOutlineItem | null = null;
    for (const o of sorted) {
      if (o.depth !== 0) continue;
      if (hasAny(o.role, OPENING_ROLE_WORDS)) {
        openingNode ??= o;
        continue;
      }
      if (hasAny(o.role, CLOSING_ROLE_WORDS) || nodeIsMaterial(o)) continue;
      thesisNode ??= o;
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
      if (o.depth === 0 && hasAny(o.role, OPENING_ROLE_WORDS)) {
        if (written(o)) slots.push(card("opening", o));
        else if (o.text.trim()) opening.materials.push(o.text.trim());
      } else if (o.depth === 0 && hasAny(o.role, CLOSING_ROLE_WORDS)) {
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
