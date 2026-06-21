import { describe, it, expect } from "vitest";
import { nodeView } from "./nodeView";
import type { ProcessNode } from "./processTree";

// ─── fixtures ─────────────────────────────────────────────────────────────────

function makeNode(overrides: Partial<ProcessNode> = {}): ProcessNode {
  return {
    id: "n0",
    type: "task_root",
    parent_id: null,
    title: "中国是否让地球变得更可持续？",
    ...overrides,
  };
}

// ─── tag labels ──────────────────────────────────────────────────────────────

describe("nodeView tag labels", () => {
  it("task_root → tag「任务」 (or matching label)", () => {
    const result = nodeView(makeNode({ type: "task_root" }));
    expect(result.tag).toBe("任务根");
  });

  it("card_use → tag「工具卡」", () => {
    const result = nodeView(makeNode({ type: "card_use", parent_id: "n0", title: "SIFT×CRAAP 信息核查" }));
    expect(result.tag).toBe("工具卡使用");
  });

  it("concession → tag「让步」", () => {
    const result = nodeView(makeNode({ type: "concession", parent_id: "n0", title: "让步段" }));
    expect(result.tag).toBe("让步修订");
  });

  it("reflection → tag「反身」", () => {
    const result = nodeView(makeNode({ type: "reflection", parent_id: "n0", title: "反身收口" }));
    expect(result.tag).toBe("反身收口");
  });

  it("sub_question → tag「子问题」 (S5 forward-compat)", () => {
    const result = nodeView(makeNode({ type: "sub_question", parent_id: "n0", title: "这个来源可信吗？" }));
    expect(result.tag).toBe("子问题");
  });

  it("key_knowledge → tag「关键知识」 (S5 forward-compat)", () => {
    const result = nodeView(makeNode({ type: "key_knowledge", parent_id: "n0", title: "NASA 数据" }));
    expect(result.tag).toBe("关键知识");
  });

  it("attempt → tag「尝试草稿」 (S5 forward-compat)", () => {
    const result = nodeView(makeNode({ type: "attempt", parent_id: "n0", title: "初稿" }));
    expect(result.tag).toBe("尝试草稿");
  });
});

// ─── indent ──────────────────────────────────────────────────────────────────

describe("nodeView indent", () => {
  it("root (parent_id === null) → indent false", () => {
    const result = nodeView(makeNode({ parent_id: null }));
    expect(result.indent).toBe(false);
  });

  it("child node (parent_id !== null) → indent true", () => {
    const result = nodeView(makeNode({ type: "card_use", parent_id: "n0" }));
    expect(result.indent).toBe(true);
  });
});

// ─── style objects ───────────────────────────────────────────────────────────

describe("nodeView style objects", () => {
  it("returns non-empty tag for every node type", () => {
    const types: ProcessNode["type"][] = [
      "task_root", "card_use", "concession", "reflection",
      "sub_question", "key_knowledge", "attempt",
    ];
    for (const type of types) {
      const result = nodeView(makeNode({ type, parent_id: type === "task_root" ? null : "n0" }));
      expect(result.tag.length).toBeGreaterThan(0);
    }
  });

  it("returns tagStyle object with expected shape for task_root", () => {
    const result = nodeView(makeNode({ type: "task_root" }));
    expect(typeof result.tagStyle).toBe("object");
    expect(result.tagStyle).toHaveProperty("display", "inline-block");
    expect(result.tagStyle).toHaveProperty("fontSize", "11px");
    expect(result.tagStyle).toHaveProperty("fontWeight", 700);
  });

  it("returns markerStyle object with expected shape", () => {
    const result = nodeView(makeNode({ type: "task_root" }));
    expect(typeof result.markerStyle).toBe("object");
    expect(result.markerStyle).toHaveProperty("width", "12px");
    expect(result.markerStyle).toHaveProperty("height", "12px");
    expect(result.markerStyle).toHaveProperty("borderRadius", "50%");
  });

  it("returns rowStyle object with display flex for task_root", () => {
    const result = nodeView(makeNode({ type: "task_root" }));
    expect(typeof result.rowStyle).toBe("object");
    expect(result.rowStyle).toHaveProperty("display", "flex");
    expect(result.rowStyle).toHaveProperty("gap", "11px");
  });

  it("rowStyle.marginLeft is 0 for root, 18px for child", () => {
    const root = nodeView(makeNode({ parent_id: null }));
    expect(root.rowStyle.marginLeft).toBe("0px");
    const child = nodeView(makeNode({ type: "card_use", parent_id: "n0" }));
    expect(child.rowStyle.marginLeft).toBe("18px");
  });
});
