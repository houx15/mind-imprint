import { describe, expect, it } from "vitest";
import type { WritingOutlineItem, WritingSnippet } from "../api/writingRoom";
import { buildSlots } from "./slots";

const node = (id: string, depth: number, position: number, text: string): WritingOutlineItem =>
  ({ id, text, role: "", depth, position }) as WritingOutlineItem;
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
  node("t", 0, 0, "不该一刀切禁手机"),
  node("a", 1, 1, "手机能查资料"),
  node("a1", 2, 2, "上周查线粒体"),
  node("b", 1, 3, "手机容易分心"),
  node("b1", 2, 4, "晚自习刷视频"),
];

describe("buildSlots", () => {
  it("folds material into the paragraph above it and numbers blocks in screen order", () => {
    const slots = buildSlots(outline, []);
    expect(slots.map((s) => [s.outlineId, s.number])).toEqual([
      ["t", 1],
      ["a", 2],
      ["b", 3],
    ]);
    expect(slots[1]!.materials).toEqual(["上周查线粒体"]);
    expect(slots[2]!.materials).toEqual(["晚自习刷视频"]);
  });

  it("keeps a material node that already has her paragraph", () => {
    const slots = buildSlots(outline, [snip("s1", "a1", 2, "那天我查了线粒体……")]);
    expect(slots.map((s) => s.outlineId)).toEqual(["t", "a", "a1", "b"]);
    expect(slots[2]!.snippet?.id).toBe("s1");
    expect(slots.map((s) => s.number)).toEqual([1, 2, 3, 4]);
  });

  it("keeps free paragraphs at the end, numbered after the outline blocks", () => {
    const slots = buildSlots(outline, [snip("f", null, 9, "加的一段")]);
    expect(slots.at(-1)).toMatchObject({ outlineId: null, number: 4 });
  });

  it("a material node with no block above it stays a block", () => {
    const slots = buildSlots([node("m", 2, 0, "一个数据")], []);
    expect(slots.map((s) => s.outlineId)).toEqual(["m"]);
  });
});
