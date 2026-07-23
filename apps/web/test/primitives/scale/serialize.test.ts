import { describe, it, expect } from "vitest";
import { scaleStateToAnchors, anchorsToScaleState } from "@/primitives/scale/serialize";
import type { Bucket } from "@/primitives/scale/serialize";
import type { ScaleState, Anchor } from "@mind-imprint/contracts";

const stops: Bucket[] = [
  { id: "个人猜测", label: "个人猜测", hint: "只有直觉，没有证据" },
  { id: "有据推断", label: "有据推断", hint: "有证据，但推理仍可争议" },
  { id: "强证据", label: "强证据", hint: "多个独立来源支撑" },
  { id: "科学共识", label: "科学共识", hint: "领域内已达成共识" },
  { id: "逻辑必然", label: "逻辑必然", hint: "由定义或推理必然为真" },
];

describe("scaleStateToAnchors", () => {
  it("maps each item's text/stop/reason onto quote/dimension/answer, author student", () => {
    const state: ScaleState = {
      items: [{ id: "i1", text: "中国的碳排放正在下降", stop: "有据推断", reason: "有统计数据但口径可争议", author: "student" }],
      rewrite: "",
    };
    const anchors = scaleStateToAnchors(state);
    expect(anchors).toHaveLength(1);
    expect(anchors[0]).toMatchObject({
      quote: "中国的碳排放正在下降",
      dimension: "有据推断",
      answer: "有统计数据但口径可争议",
      author: "student",
      material_id: "",
      block_id: "",
      start: 0,
      end: 0,
      question: "",
    });
  });

  it("drops items with blank text — an empty row the student never filled is not data", () => {
    const state: ScaleState = {
      items: [
        { id: "i1", text: "", stop: "强证据", reason: "some reason", author: "student" },
        { id: "i2", text: "   ", stop: "科学共识", reason: "some reason", author: "student" },
        { id: "i3", text: "地球在变暖", stop: "科学共识", reason: "IPCC 多份报告共识", author: "student" },
      ],
      rewrite: "",
    };
    const anchors = scaleStateToAnchors(state);
    expect(anchors).toHaveLength(1);
    expect(anchors[0]!.quote).toBe("地球在变暖");
  });

  it("emits the rewrite anchor separately, only when non-blank, with dimension 'rewrite'", () => {
    const withRewrite: ScaleState = {
      items: [{ id: "i1", text: "中国的碳排放正在下降", stop: "有据推断", reason: "有统计数据", author: "student" }],
      rewrite: "中国的碳排放大概正在下降",
    };
    const anchors = scaleStateToAnchors(withRewrite);
    expect(anchors).toHaveLength(2);
    const rewriteAnchor = anchors.find((a) => a.dimension === "rewrite");
    expect(rewriteAnchor).toMatchObject({
      dimension: "rewrite",
      author: "student",
      answer: "中国的碳排放大概正在下降",
      quote: "",
    });

    const blankRewrite: ScaleState = {
      items: [{ id: "i1", text: "中国的碳排放正在下降", stop: "有据推断", reason: "有统计数据", author: "student" }],
      rewrite: "   ",
    };
    const anchorsNoRewrite = scaleStateToAnchors(blankRewrite);
    expect(anchorsNoRewrite).toHaveLength(1);
    expect(anchorsNoRewrite.find((a) => a.dimension === "rewrite")).toBeUndefined();
  });
});

describe("anchorsToScaleState", () => {
  it("drops anchors whose dimension is not a declared stop id", () => {
    const anchors: Anchor[] = [
      { id: "a1", material_id: "", block_id: "", start: 0, end: 0, quote: "text one", dimension: "强证据", author: "student", question: "", answer: "reason one" },
      { id: "a2", material_id: "", block_id: "", start: 0, end: 0, quote: "orphaned", dimension: "不存在的维度", author: "student", question: "", answer: "" },
    ];
    const state = anchorsToScaleState(anchors, stops);
    expect(state.items).toHaveLength(1);
    expect(state.items[0]).toMatchObject({ text: "text one", stop: "强证据", reason: "reason one", author: "student" });
  });

  it("routes the 'rewrite' dimension anchor into state.rewrite and NEVER into items", () => {
    const anchors: Anchor[] = [
      { id: "a1", material_id: "", block_id: "", start: 0, end: 0, quote: "中国的碳排放正在下降", dimension: "有据推断", author: "student", question: "", answer: "有统计数据" },
      { id: "a2", material_id: "", block_id: "", start: 0, end: 0, quote: "", dimension: "rewrite", author: "student", question: "", answer: "中国的碳排放大概正在下降" },
    ];
    const state = anchorsToScaleState(anchors, stops);
    expect(state.rewrite).toBe("中国的碳排放大概正在下降");
    expect(state.items).toHaveLength(1);
    expect(state.items.some((it) => it.stop === "rewrite")).toBe(false);
    expect(state.items.every((it) => it.text !== "中国的碳排放大概正在下降")).toBe(true);
  });

  it("defaults rewrite to '' when no rewrite anchor is present", () => {
    const anchors: Anchor[] = [
      { id: "a1", material_id: "", block_id: "", start: 0, end: 0, quote: "text one", dimension: "强证据", author: "student", question: "", answer: "reason one" },
    ];
    const state = anchorsToScaleState(anchors, stops);
    expect(state.rewrite).toBe("");
  });
});

describe("round-trip", () => {
  it("is exact (field-for-field) for a well-formed state including rewrite", () => {
    const state: ScaleState = {
      items: [
        { id: "i1", text: "中国的碳排放正在下降", stop: "有据推断", reason: "有统计数据但口径可争议", author: "student" },
        { id: "i2", text: "地球在变暖", stop: "科学共识", reason: "IPCC 多份报告共识", author: "student" },
      ],
      rewrite: "地球大概率正在变暖",
    };
    const roundTripped = anchorsToScaleState(scaleStateToAnchors(state), stops);
    // ids are freshly minted on the way back (anchors carry no persisted item
    // id), so compare everything except id — the same convention sort's
    // round trip uses.
    const strip = (s: ScaleState) => ({ items: s.items.map(({ id: _id, ...rest }) => rest), rewrite: s.rewrite });
    expect(strip(roundTripped)).toEqual(strip(state));
  });

  it("round-trips exactly when rewrite is blank (no rewrite anchor at all)", () => {
    const state: ScaleState = {
      items: [{ id: "i1", text: "中国的碳排放正在下降", stop: "有据推断", reason: "有统计数据", author: "student" }],
      rewrite: "",
    };
    const roundTripped = anchorsToScaleState(scaleStateToAnchors(state), stops);
    const strip = (s: ScaleState) => ({ items: s.items.map(({ id: _id, ...rest }) => rest), rewrite: s.rewrite });
    expect(strip(roundTripped)).toEqual(strip(state));
  });
});
