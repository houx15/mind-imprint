import type { WritingOutlineItem, WritingSnippet } from "../api/writingRoom";
import { outlineKindIsInlineContent, outlineKindLabel, outlineKindOf } from "./outlineKind";

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

/**
 * 卡片的名字：屏幕上、卡片叠里都用这一个。
 *
 * 🚨 2026-09-23 产品负责人报的断点，这一行就是它：
 *
 *   「在「结构」这一步，它们标的是 场景、转折、感悟。在「行文」这一步，
 *     还是 场景、转折、感悟。到了「段落」这一步，全部变成
 *     分论点 1、分论点 2、分论点 3。」
 *
 * 前两步按节点的 kind 印名字，这一步原来只看卡片那一层的 `kind`
 *（opening/point/closing/free 四种），于是一篇记叙文的场景、转折、感悟
 * 三张卡全被印成「分论点 N」。同一条轴在最后一段路上断了。
 *
 * 现在主体卡走 `outlineKindLabel(s.outlineKind, lang)` —— 和图上、行文那一步
 * **同一个函数**，所以三处印的必然是同一个词，英文那边也自动跟着走
 *（原来一篇英文议论文在这里印的是「分论点」，而图上印的是 "Topic sentence"）。
 *
 * 编号只在同一种 kind 内部连排：三张场景卡是「场景 1/2/3」，
 * 场景两张加一张转折是「场景 1」「场景 2」「转折 1」。原来的
 * `pointIndex` 是一路数下来的，混着两种 kind 时会印出「分论点 1、分论点 3」。
 */
export function slotTitle(s: Slot, kindIndex: number, lang = "zh", genre = "argument"): string {
  const en = lang === "en";
  switch (s.kind) {
    case "opening":
      return en ? "Introduction" : "开头";
    case "closing":
      return en ? "Conclusion" : "结尾";
    case "free":
      return en ? "Loose paragraph" : "自由段落";
    default: {
      const label = outlineKindLabel(slotBodyKind(s, genre), lang);
      return kindIndex > 0 ? `${label} ${kindIndex}` : label;
    }
  }
}

/**
 * 一张主体卡该按哪一种 kind 命名。
 *
 * 🚨 节点的 kind 不能直接拿来用。`buildSlots` 会把一些**不是段落层**的节点
 * 摆成主体卡：第二个深度 0 的节点（老数据里它算 thesis）、一条她已经写过字的
 * 论据、前面还没有分论点的第一条材料。那几张卡的活是「先写出这一段要说的那
 * 句话」，所以印成「中心论点」或者「论据 · 你见过的事」都是错的 ——
 * 那不是这一段的名字，是它里面那样东西的名字。
 *
 * 挡的只有**深度 0 那三种**（中心论点 / 开篇 / 结尾）。它们不是段落层的
 * 名字：一个被摆成主体卡的 thesis 节点，这张卡的活是「先写出这一段要说的
 * 那句话」，印成「中心论点」是把整篇的那一句安到一段上。
 *
 * 别的都按自己的名字印，方向和 2026-09-20 那次修的一样（那次是「结尾」被
 * 印成「分论点 3」，产品负责人的原话：「这个是总结，不是分论点」）——
 * 她在图上把一条标成什么，这张卡就叫什么。
 *
 * 退回时按文体挑默认段落名：议论文是分论点，记叙文是场景。
 */
const NOT_BODY_KINDS = new Set(["thesis", "opening", "closing", ""]);

export function slotBodyKind(s: Slot, genre: string): string {
  if (!NOT_BODY_KINDS.has(s.outlineKind)) return s.outlineKind;
  return genre === "narrative" ? "scene" : "point";
}
