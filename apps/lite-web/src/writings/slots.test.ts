import { describe, expect, it } from "vitest";
import type { WritingOutlineItem, WritingSnippet } from "../api/writingRoom";
import { buildSlots, CLOSING_POSITION } from "./slots";

// Same cases as apps/api/internal/api/writing_blocks_internal_test.go —
// the two sides must number cards the same way.

const node = (id: string, depth: number, position: number, text: string, role = ""): WritingOutlineItem =>
  ({ id, text, role, depth, position }) as WritingOutlineItem;
const snip = (id: string, outlineId: string | null, position: number, text: string): WritingSnippet => ({
  id,
  outlineId,
  outlineHeading: "",
  position,
  text,
  updatedAt: "",
});

// 中心论点 → 分论点A → 例子a ；分论点B → 例子b
const outline = [
  node("t", 0, 0, "不该一刀切禁手机", "中心论点"),
  node("a", 1, 1, "手机能查资料"),
  node("a1", 2, 2, "上周查线粒体"),
  node("b", 1, 3, "手机容易分心"),
  node("b1", 2, 4, "晚自习刷视频"),
];

describe("buildSlots", () => {
  it("builds 开头 · one card per point with its examples · 结尾", () => {
    const slots = buildSlots(outline, []);
    expect(slots.map((s) => [s.kind, s.outlineId, s.number])).toEqual([
      ["opening", "t", 1],
      ["point", "a", 2],
      ["point", "b", 3],
      ["closing", null, 4],
    ]);
    expect(slots[0]!.claim).toBe("不该一刀切禁手机");
    expect(slots[1]!.materials).toEqual(["上周查线粒体"]);
    expect(slots[2]!.materials).toEqual(["晚自习刷视频"]);
    expect(slots[3]!.position).toBe(CLOSING_POSITION);
  });

  // 2026-09-18：「我有两个例子，结果就变成了两段。」
  it("two examples under the thesis make one body card, not two parts", () => {
    const slots = buildSlots(
      [
        node("t", 0, 0, "苦乐在心不在事", "中心论点"),
        node("e1", 1, 1, "跳绳从烦到乐", "你经历过的事"),
        node("e2", 1, 2, "苏轼被贬黄州", "历史上的例子"),
      ],
      [],
    );
    expect(slots.map((s) => s.kind)).toEqual(["opening", "point", "closing"]);
    expect(slots[1]!.needsPoint).toBe(true);
    expect(slots[1]!.materials).toEqual(["跳绳从烦到乐", "苏轼被贬黄州"]);
  });

  it("keeps a material node that already has her paragraph", () => {
    const slots = buildSlots(outline, [snip("s1", "a1", 2, "那天我查了线粒体……")]);
    expect(slots.map((s) => s.outlineId)).toEqual(["t", "a", "a1", "b", null]);
    expect(slots[2]!.snippet?.id).toBe("s1");
    expect(slots.map((s) => s.number)).toEqual([1, 2, 3, 4, 5]);
  });

  it("keeps free paragraphs at the end, after 结尾", () => {
    const slots = buildSlots(outline, [snip("f", null, 1000, "加的一段")]);
    expect(slots.at(-1)).toMatchObject({ kind: "free", outlineId: null, number: 5 });
  });

  it("a virtual 结尾 finds its paragraph by the reserved position", () => {
    const slots = buildSlots(outline, [snip("end", null, CLOSING_POSITION, "所以……")]);
    expect(slots[3]).toMatchObject({ kind: "closing", number: 4 });
    expect(slots[3]!.snippet?.id).toBe("end");
    expect(slots).toHaveLength(4);
  });

  // 例子落在最上层（同一轮加的分论点它引用不到）也不当中心论点、不单独成段。
  it("a top-level example is material, not the thesis and not a card", () => {
    const slots = buildSlots(
      [
        node("t", 0, 0, "人可以脆弱", "中心论点"),
        node("a", 1, 1, "脆弱没有打垮我", "分论点"),
        node("x", 0, 2, "爸爸入狱、妹妹抑郁", "你经历过的事"),
      ],
      [],
    );
    expect(slots.map((s) => s.kind)).toEqual(["opening", "point", "closing"]);
    expect(slots[0]!.claim).toBe("人可以脆弱");
    expect(slots[1]!.materials).toEqual(["爸爸入狱、妹妹抑郁"]);
  });

  it("an empty map has no cards", () => {
    expect(buildSlots([], [])).toEqual([]);
  });
});
