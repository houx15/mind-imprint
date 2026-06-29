import { describe, it, expect } from "vitest";
import { evalView } from "./evalView";
import { FULL_RUBRIC, SOLO_LABELS } from "@mind-imprint/contracts";
import type { Evaluation } from "@mind-imprint/contracts";

// ─── fixtures ────────────────────────────────────────────────────────────────

const baseEvaluation: Evaluation = {
  id: "ev-fixture", task_id: "task-phoebe-001", status: "done", completed_at: null,
  narrative:
    "这一程你最大的转变发生在两处。你没有直接采信那篇公众号文章，而是横向找到了 NASA 与 Nature Sustainability（IF 32.1）两个独立来源——来源意识从被动变主动。面对「碳排放全球第一」这个对你不利的事实，你选择正面接住、写让步段，而不是绕开。",
  created_at: "2026-06-21T00:00:00.000Z",
  scores: [],
};

// ─── evalView tests ───────────────────────────────────────────────────────────

describe("evalView", () => {
  it("D4 L4 → dim='多视角与让步', levelLabel contains 'L4' and '卓越', 4 segs all filled", () => {
    const evaluation: Evaluation = {
      ...baseEvaluation,
      scores: [
        { dim_id: "D4", level: "L4", note: "构建了反方最强论证后再让步反驳，让步段写作达到 L4 级" },
      ],
    };
    const result = evalView(evaluation, FULL_RUBRIC);
    expect(result.dims).toHaveLength(1);
    // Guard: verify first dim exists before accessing properties
    if (!result.dims[0]) throw new Error("Expected dims[0] to exist");
    const dim = result.dims[0];
    expect(dim.dim).toBe("多视角与让步");
    expect(dim.levelLabel).toContain("L4");
    expect(dim.levelLabel).toContain("卓越");
    expect(dim.segs).toHaveLength(4);
    expect(dim.segs.every((s) => s.filled)).toBe(true);
  });

  it("L2 → 2 filled, 2 unfilled", () => {
    const evaluation: Evaluation = {
      ...baseEvaluation,
      scores: [{ dim_id: "D4", level: "L2", note: "提到了反方但轻描淡写" }],
    };
    const result = evalView(evaluation, FULL_RUBRIC);
    if (!result.dims[0]) throw new Error("Expected dims[0] to exist");
    const dim = result.dims[0];
    expect(dim.segs).toHaveLength(4);
    const filled = dim.segs.filter((s) => s.filled).length;
    const unfilled = dim.segs.filter((s) => !s.filled).length;
    expect(filled).toBe(2);
    expect(unfilled).toBe(2);
    // First 2 filled, last 2 unfilled
    if (
      !dim.segs[0] ||
      !dim.segs[1] ||
      !dim.segs[2] ||
      !dim.segs[3]
    )
      throw new Error("Expected 4 segs");
    expect(dim.segs[0].filled).toBe(true);
    expect(dim.segs[1].filled).toBe(true);
    expect(dim.segs[2].filled).toBe(false);
    expect(dim.segs[3].filled).toBe(false);
  });

  it("L1 → 1 filled, 3 unfilled", () => {
    const evaluation: Evaluation = {
      ...baseEvaluation,
      scores: [{ dim_id: "D2", level: "L1", note: "完全信任 AI 来源" }],
    };
    const result = evalView(evaluation, FULL_RUBRIC);
    if (!result.dims[0]) throw new Error("Expected dims[0] to exist");
    const segs = result.dims[0].segs;
    if (!segs[0] || !segs[1] || !segs[2] || !segs[3]) throw new Error("Expected 4 segs");
    expect(segs[0].filled).toBe(true);
    expect(segs[1].filled).toBe(false);
    expect(segs[2].filled).toBe(false);
    expect(segs[3].filled).toBe(false);
  });

  it("L3 → 3 filled, 1 unfilled", () => {
    const evaluation: Evaluation = {
      ...baseEvaluation,
      scores: [{ dim_id: "D3", level: "L3", note: "主动多源对照，找到 2+ 独立来源" }],
    };
    const result = evalView(evaluation, FULL_RUBRIC);
    if (!result.dims[0]) throw new Error("Expected dims[0] to exist");
    const segs = result.dims[0].segs;
    if (!segs[3]) throw new Error("Expected 4 segs");
    expect(segs.filter((s) => s.filled).length).toBe(3);
    expect(segs[3].filled).toBe(false);
  });

  it("levelLabel format: 'L4 · 卓越'", () => {
    const evaluation: Evaluation = {
      ...baseEvaluation,
      scores: [{ dim_id: "D4", level: "L4", note: "正面接住碳排放反例" }],
    };
    const result = evalView(evaluation, FULL_RUBRIC);
    if (!result.dims[0]) throw new Error("Expected dims[0] to exist");
    expect(result.dims[0].levelLabel).toBe(`L4 · ${SOLO_LABELS["L4"]}`);
  });

  it("unknown dim_id falls back to the dim_id string", () => {
    const evaluation: Evaluation = {
      ...baseEvaluation,
      scores: [{ dim_id: "UNKNOWN_DIM", level: "L3", note: "some note" }],
    };
    const result = evalView(evaluation, FULL_RUBRIC);
    if (!result.dims[0]) throw new Error("Expected dims[0] to exist");
    expect(result.dims[0].dim).toBe("UNKNOWN_DIM");
  });

  it("note is preserved verbatim from the score", () => {
    const note = "从被动采信公众号文章 → 主动溯源到 NASA Earth Observatory";
    const evaluation: Evaluation = {
      ...baseEvaluation,
      scores: [{ dim_id: "D2", level: "L3", note }],
    };
    const result = evalView(evaluation, FULL_RUBRIC);
    if (!result.dims[0]) throw new Error("Expected dims[0] to exist");
    expect(result.dims[0].note).toBe(note);
  });

  it("multiple scores produce multiple dims in same order", () => {
    const evaluation: Evaluation = {
      ...baseEvaluation,
      scores: [
        { dim_id: "D2", level: "L3", note: "来源意识从被动变主动" },
        { dim_id: "D3", level: "L3", note: "找到 NASA 与 Nature Sustainability 两个独立来源" },
        { dim_id: "D4", level: "L4", note: "正面接住「碳排放全球第一」写让步段" },
        { dim_id: "D5", level: "L3", note: "能识别论点-论据-假设结构" },
        { dim_id: "D6", level: "L2", note: "可再多问：来源各自的立场？" },
      ],
    };
    const result = evalView(evaluation, FULL_RUBRIC);
    expect(result.dims).toHaveLength(5);
    if (
      !result.dims[0] ||
      !result.dims[1] ||
      !result.dims[2] ||
      !result.dims[3] ||
      !result.dims[4]
    )
      throw new Error("Expected 5 dims");
    expect(result.dims[0].dim).toBe("信源辨识");
    expect(result.dims[1].dim).toBe("横向验证");
    expect(result.dims[2].dim).toBe("多视角与让步");
    expect(result.dims[3].dim).toBe("论证拆解");
    expect(result.dims[4].dim).toBe("反思与元认知");
  });
});
