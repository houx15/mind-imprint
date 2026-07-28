import { describe, it, expect } from "vitest";
import { OutlineNode } from "../src/outlineNode";

describe("OutlineNode", () => {
  it("parses a valid node", () => {
    const ok = OutlineNode.parse({ id: "o1", text: "引言", depth: 0, position: 0 });
    expect(ok.depth).toBe(0);
  });
  it("rejects a non-integer depth", () => {
    expect(() => OutlineNode.parse({ id: "o1", text: "x", depth: 1.5, position: 0 })).toThrow();
  });
});
