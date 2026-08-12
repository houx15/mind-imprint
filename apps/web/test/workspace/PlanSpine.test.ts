import { describe, it, expect } from "vitest";
import { derivePlanStages } from "@/workspace/blocks/PlanSpine";
import type { PlanItem } from "@mind-imprint/contracts";

function item(p: Partial<PlanItem>): PlanItem {
  return {
    id: p.id ?? "i",
    title: p.title ?? "t",
    tag: p.tag ?? "read",
    column: p.column ?? "todo",
    stage: p.stage ?? "阶段一",
    refMaterialId: null,
    start: p.start ?? 0,
    days: p.days ?? 1,
    position: p.position ?? 0,
  };
}

describe("derivePlanStages", () => {
  it("orders stages by earliest timeline start, not lexical/array order", () => {
    // Server returns them lexically (阶段一/三/二 by codepoint), out of schedule order.
    const items = [
      item({ stage: "阶段一 · 立项", start: 0 }),
      item({ stage: "阶段三 · 成文", start: 20 }),
      item({ stage: "阶段二 · 研究", start: 8 }),
    ];
    const { stages } = derivePlanStages(items);
    expect(stages).toEqual(["阶段一 · 立项", "阶段二 · 研究", "阶段三 · 成文"]);
  });

  it("current stage = first stage with an unfinished item, in timeline order", () => {
    const items = [
      item({ stage: "阶段一", start: 0, column: "done" }),
      item({ stage: "阶段三", start: 20, column: "todo" }),
      item({ stage: "阶段二", start: 8, column: "doing" }),
    ];
    const { stages, currentIndex } = derivePlanStages(items);
    expect(stages).toEqual(["阶段一", "阶段二", "阶段三"]);
    // 阶段一 all done → current is 阶段二 (index 1), the first with unfinished work.
    expect(currentIndex).toBe(1);
  });

  it("all done → last stage is current", () => {
    const items = [
      item({ stage: "阶段一", start: 0, column: "done" }),
      item({ stage: "阶段二", start: 8, column: "done" }),
    ];
    const { currentIndex } = derivePlanStages(items);
    expect(currentIndex).toBe(1);
  });

  it("empty plan → no stages", () => {
    expect(derivePlanStages([]).stages).toEqual([]);
  });
});
