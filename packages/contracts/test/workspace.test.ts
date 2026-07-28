import { describe, it, expect } from "vitest";
import { WorkspaceProjection } from "../src/workspace";

const wire = {
  id: "proj1",
  title: "中国是否让地球变得更可持续？",
  qualification: "IBDP",
  proposal: { objective: "o", reason: "r", activities: "a", resources: "s" },
};

describe("WorkspaceProjection", () => {
  it("parses the lean projection with an embedded proposal", () => {
    expect(WorkspaceProjection.parse(wire).proposal.objective).toBe("o");
  });
  it("rejects a malformed embedded proposal", () => {
    expect(() => WorkspaceProjection.parse({ ...wire, proposal: { objective: "o" } })).toThrow();
  });
});
