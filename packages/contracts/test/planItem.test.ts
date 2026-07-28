import { describe, it, expect } from "vitest";
import { PlanItem } from "../src/planItem";

const item = {
  id: "p1",
  title: "读 NASA 报告",
  tag: "read" as const,
  column: "todo" as const,
  stage: "溯源",
  refMaterialId: null,
  start: 0,
  days: 3,
  position: 0,
};

describe("PlanItem", () => {
  it("parses a valid item and keeps refMaterialId nullable", () => {
    expect(PlanItem.parse(item).refMaterialId).toBeNull();
    expect(PlanItem.parse({ ...item, refMaterialId: "m1" }).refMaterialId).toBe("m1");
  });
  it("rejects an unknown tag", () => {
    expect(() => PlanItem.parse({ ...item, tag: "cite" })).toThrow();
  });
});
