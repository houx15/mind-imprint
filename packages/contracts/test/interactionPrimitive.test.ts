import { describe, it, expect } from "vitest";
import { Author, AnnotateState, GraphState, PRIMITIVE_KINDS, SortState, ScaleState, MatrixState } from "../src/interactionPrimitive";
import { CompareState, ComparePair } from "../src/interactionPrimitive";

describe("interaction primitives (C1)", () => {
  it("author is student|ai|imported", () => {
    expect(Author.safeParse("student").success).toBe(true);
    expect(Author.safeParse("teacher").success).toBe(false);
  });
  it("annotate state requires author on every span", () => {
    const ok = AnnotateState.safeParse({
      material_id: "m1",
      spans: [{ id: "s1", block_ref: "b1", tag: "authority", note: "who?", author: "student" }],
    });
    expect(ok.success).toBe(true);
    const bad = AnnotateState.safeParse({ material_id: "m1", spans: [{ id: "s1", tag: "x", note: "" }] });
    expect(bad.success).toBe(false); // missing author
  });
  it("graph state carries typed nodes/edges with author", () => {
    const ok = GraphState.safeParse({
      nodes: [{ id: "n1", type: "claim", text: "China's build-out is additive", author: "student" }],
      edges: [{ id: "e1", from: "n1", to: "n2", type: "supports" }],
    });
    expect(ok.success).toBe(true);
  });
  it("registry lists the finite primitive kinds", () => {
    expect(PRIMITIVE_KINDS).toContain("annotate");
    expect(PRIMITIVE_KINDS).toContain("graph");
  });
});

const leftPane = {
  material_id: "mat-blog",
  spans: [{ id: "s1", block_ref: "b1", tag: "claim", note: "", author: "ai" as const }],
};

describe("CompareState", () => {
  it("accepts a right pane that is null — the empty right pane IS the assignment", () => {
    const parsed = CompareState.parse({ left: leftPane, right: null, pairs: [] });
    expect(parsed.right).toBeNull();
  });

  it("accepts a student-authored pair linking a left span to a right span", () => {
    const parsed = CompareState.parse({
      left: leftPane,
      right: { material_id: "mat-nasa", spans: [{ id: "s2", block_ref: "b9", tag: "find", note: "", author: "student" }] },
      pairs: [{ id: "p1", l_span: "s1", r_span: "s2", note: "NASA 只说绿化面积，没说可持续性", relation: "qualifies", author: "student" }],
    });
    expect(parsed.pairs[0]!.relation).toBe("qualifies");
  });

  it("rejects an AI-authored pair — the relation and the note are the student's judgment", () => {
    expect(() =>
      ComparePair.parse({ id: "p1", l_span: "s1", r_span: "s2", note: "x", relation: "corroborates", author: "ai" }),
    ).toThrow();
  });

  it("rejects a relation outside the closed set", () => {
    expect(() =>
      ComparePair.parse({ id: "p1", l_span: "s1", r_span: "s2", note: "x", relation: "debunks", author: "student" }),
    ).toThrow();
  });
});

describe("sort/scale/matrix states", () => {
  it("parses a sort state", () => {
    const s = SortState.parse({
      items: [{ id: "s1", text: "中国碳排放全球第一", bucket: "事实", reason: "可以去核查", author: "student" }],
    });
    expect(s.items[0]!.bucket).toBe("事实");
  });

  it("parses a scale state with a rewrite", () => {
    const s = ScaleState.parse({
      items: [{ id: "i1", text: "中国让地球更可持续", stop: "有据推断", reason: "证据只覆盖绿化", author: "student" }],
      rewrite: "中国很可能在绿化上做出了最大贡献",
    });
    expect(s.rewrite).toContain("很可能");
  });

  it("parses a matrix state", () => {
    const s = MatrixState.parse({
      rows: [{ id: "r1", label: "环保组织", cells: { position: "进展不足" }, author: "student" }],
    });
    expect(s.rows[0]!.cells.position).toBe("进展不足");
  });

  it("rejects a row with a non-string cell", () => {
    expect(() => MatrixState.parse({ rows: [{ id: "r1", label: "x", cells: { position: 1 }, author: "student" }] })).toThrow();
  });
});
