import { describe, it, expect } from "vitest";
import { Author, AnnotateState, GraphState, PRIMITIVE_KINDS } from "../src/interactionPrimitive";

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
