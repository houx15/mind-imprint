import { describe, expect, it } from "vitest";
import { rekindChoices, rekindOutlineNode } from "./outlineRekind";
import { outlineKindOf } from "./outlineKind";
import type { WritingOutlineItem } from "../api/writingRoom";

/** 一份扁平清单：[文字, kind]，深度由 kind 定。 */
function outline(...rows: [string, string][]): WritingOutlineItem[] {
  const depthOf: Record<string, number> = {
    thesis: 0,
    opening: 0,
    closing: 0,
    point: 1,
    counter: 1,
    scene: 1,
    evidence: 2,
    reference: 2,
    reasoning: 2,
    rebuttal: 2,
    gap: 2,
    detail: 2,
  };
  return rows.map(([text, kind], i) => ({
    id: text,
    text,
    role: "",
    kind,
    depth: depthOf[kind] ?? 1,
    position: i,
  }));
}

function shape(items: WritingOutlineItem[]): string[] {
  return items
    .slice()
    .sort((a, b) => a.position - b.position)
    .map((r) => `${"  ".repeat(r.depth)}${r.text}[${outlineKindOf(r)}]`);
}

// 同事 2026-09-22 的意见 3，照那张图摆。
const SCREENSHOT = outline(
  ["我想写「成功」这个词", "thesis"],
  ["成功的定义太窄，成功的人就太少", "point"],
  ["人人有自己的贡献，平凡尽责也是成功", "point"],
  ["外卖员辛勤付出让人按时吃上饭，是成功", "reference"],
  ["黑心商家哪怕赚很多钱，也是失败", "reference"],
);

describe("rekindOutlineNode", () => {
  it("🚨 把一条被摆成论据的反面论证改成反方观点 —— 在这之前她做不到", () => {
    // 拖动只改深度，任何拖到深度 1 的东西一律变成 point，
    // 「反方观点」这一种根本到不了。
    const res = rekindOutlineNode(SCREENSHOT, "黑心商家哪怕赚很多钱，也是失败", "counter");
    expect(res.ok).toBe(true);
    if (!res.ok) return;
    expect(shape(res.items)).toEqual([
      "我想写「成功」这个词[thesis]",
      "  成功的定义太窄，成功的人就太少[point]",
      "  人人有自己的贡献，平凡尽责也是成功[point]",
      "    外卖员辛勤付出让人按时吃上饭，是成功[reference]",
      "  黑心商家哪怕赚很多钱，也是失败[counter]",
    ]);
  });

  it("改成分论点也行 —— 同一条动作，另一个答案", () => {
    const res = rekindOutlineNode(SCREENSHOT, "黑心商家哪怕赚很多钱，也是失败", "point");
    expect(res.ok).toBe(true);
    if (!res.ok) return;
    expect(shape(res.items).at(-1)).toBe("  黑心商家哪怕赚很多钱，也是失败[point]");
  });

  it("🚨 挂不上就说原因，不悄悄挂到别处", () => {
    // 一篇只有一个中心论点。
    const dup = rekindOutlineNode(SCREENSHOT, "成功的定义太窄，成功的人就太少", "thesis");
    expect(dup.ok).toBe(false);
    if (dup.ok) return;
    expect(dup.why).toContain("已经有中心论点");

    // 第一条论据前面没有分论点可挂 —— 服务端会让它掉到最上层去，
    // 被下游当成一条理由。那正是 2026-09-18 记下的那个毛病。
    const early = outline(["主张", "thesis"], ["一件事", "point"]);
    const orphan = rekindOutlineNode(early, "主张", "evidence");
    expect(orphan.ok).toBe(false);
    if (orphan.ok) return;
    expect(orphan.why).toContain("分论点");
  });

  it("底下挂着东西的那一条，改深了就拒绝 —— 不把它的孩子挤出去", () => {
    const res = rekindOutlineNode(SCREENSHOT, "人人有自己的贡献，平凡尽责也是成功", "evidence");
    expect(res.ok).toBe(false);
    if (res.ok) return;
    expect(res.why).toContain("没地方放");
  });

  it("改成同一种就什么都不动", () => {
    const res = rekindOutlineNode(SCREENSHOT, "黑心商家哪怕赚很多钱，也是失败", "reference");
    expect(res.ok).toBe(true);
    if (!res.ok) return;
    expect(res.items).toBe(SCREENSHOT);
  });

  it("position 重排成连续的 0..n-1 —— 服务端和树都按它读", () => {
    const res = rekindOutlineNode(SCREENSHOT, "黑心商家哪怕赚很多钱，也是失败", "counter");
    expect(res.ok).toBe(true);
    if (!res.ok) return;
    expect(res.items.map((r) => r.position)).toEqual([0, 1, 2, 3, 4]);
  });
});

describe("rekindChoices", () => {
  it("议论文那一份里有反方观点 —— 那是同事那条要改成的东西", () => {
    expect(rekindChoices("argument")).toContain("counter");
    expect(rekindChoices("argument")[0]).toBe("point");
  });

  it("🚨 记叙文那一份里没有分论点：一篇记叙文里没有那种东西", () => {
    const narrative = rekindChoices("narrative");
    for (const k of ["thesis", "point", "counter", "evidence", "reference"]) {
      expect(narrative, `${k} 不该摆给记叙文`).not.toContain(k);
    }
    expect(narrative).toContain("scene");
  });
});
