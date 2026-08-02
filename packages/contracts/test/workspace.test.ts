import { describe, it, expect } from "vitest";
import { WorkspaceProjection } from "../src/workspace";

const wire = {
  id: "proj1",
  title: "中国是否让地球变得更可持续？",
  qualification: "IBDP",
  status: "working",
  proposal: { objective: "o", reason: "r", activities: "a", resources: "s" },
};

describe("WorkspaceProjection", () => {
  it("parses the lean projection with status + an embedded proposal", () => {
    const p = WorkspaceProjection.parse(wire);
    expect(p.proposal.objective).toBe("o");
    expect(p.status).toBe("working");
  });
  it("rejects a malformed embedded proposal", () => {
    expect(() => WorkspaceProjection.parse({ ...wire, proposal: { objective: "o" } })).toThrow();
  });
  it("rejects an unknown status", () => {
    expect(() => WorkspaceProjection.parse({ ...wire, status: "archived" })).toThrow();
  });
  it("#20 · defaults writingFinished to false when absent, and parses it when present", () => {
    expect(WorkspaceProjection.parse(wire).writingFinished).toBe(false);
    expect(WorkspaceProjection.parse({ ...wire, writingFinished: true }).writingFinished).toBe(true);
  });
});
