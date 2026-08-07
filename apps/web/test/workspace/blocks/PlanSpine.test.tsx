import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { PlanItem } from "@mind-imprint/contracts";
import { PlanSpine, derivePlanStages } from "@/workspace/blocks/PlanSpine";

function item(stage: string, column: PlanItem["column"], id: string): PlanItem {
  return { id, title: id, tag: "read", column, stage, start: 0, days: 1, position: 0, refMaterialId: null };
}

describe("derivePlanStages", () => {
  it("returns distinct stages in plan order", () => {
    const { stages } = derivePlanStages([
      item("阶段一 · 研究", "done", "a"),
      item("阶段一 · 研究", "todo", "b"),
      item("阶段二 · 答辩", "todo", "c"),
    ]);
    expect(stages).toEqual(["阶段一 · 研究", "阶段二 · 答辩"]);
  });

  it("current = the first stage with any unfinished item", () => {
    const { currentIndex } = derivePlanStages([
      item("阶段一 · 研究", "done", "a"),
      item("阶段二 · 答辩", "doing", "b"),
    ]);
    expect(currentIndex).toBe(1); // 阶段一 fully done → current is 阶段二
  });

  it("all items done → current is the last stage", () => {
    const { currentIndex } = derivePlanStages([
      item("阶段一 · 研究", "done", "a"),
      item("阶段二 · 答辩", "done", "b"),
    ]);
    expect(currentIndex).toBe(1);
  });

  it("no items → empty stages", () => {
    expect(derivePlanStages([])).toEqual({ stages: [], currentIndex: 0 });
  });
});

describe("PlanSpine", () => {
  it("renders nothing when there is no plan", () => {
    const { container } = render(<PlanSpine items={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders the stage heads and marks the current stage, and jumps to the plan on click", async () => {
    const onOpenPlan = vi.fn();
    render(
      <PlanSpine
        items={[item("阶段一 · 研究与写作", "done", "a"), item("阶段二 · 展示与答辩", "todo", "b")]}
        onOpenPlan={onOpenPlan}
      />,
    );
    // Short "阶段X" heads render (label before the ·).
    expect(screen.getByText("阶段一")).toBeInTheDocument();
    const current = screen.getByText("阶段二");
    expect(current).toHaveAttribute("data-state", "current");

    await userEvent.click(screen.getByRole("button", { name: /计划/ }));
    expect(onOpenPlan).toHaveBeenCalledOnce();
  });
});
