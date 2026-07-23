import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CriteriaCheckField } from "@/cards/fields/CriteriaCheckField";
import { fieldRegistry } from "@/cards/fieldRegistry";

const field = { type: "criteria_check" as const, key: "sci", label: "四标准",
  criteria: ["可证伪", "对照", "可重复", "同行评审"], levels: ["满足", "部分", "不满足"] };

describe("CriteriaCheckField", () => {
  it("renders one row per criterion and `levels` radios each", () => {
    render(<CriteriaCheckField field={field} value={undefined} onChange={vi.fn()} />);
    expect(screen.getByText("可证伪")).toBeTruthy();
    expect(screen.getByText("同行评审")).toBeTruthy();
    expect(screen.getAllByRole("radio")).toHaveLength(4 * 3);
  });
  it("clicking a level reports an array with that criterion set, others -1, input not mutated", async () => {
    const onChange = vi.fn();
    const initial: number[] = [];
    render(<CriteriaCheckField field={field} value={initial} onChange={onChange} />);
    // click the 3rd level (index 2) of the 1st criterion (index 0)
    const group0 = screen.getAllByRole("radiogroup")[0]!;
    await userEvent.click(within(group0).getAllByRole("radio")[2]!);
    expect(onChange).toHaveBeenLastCalledWith([2, -1, -1, -1]);
    expect(initial).toEqual([]);
  });
  it("marks the stored level as checked", () => {
    render(<CriteriaCheckField field={field} value={[0, -1, -1, 2]} onChange={vi.fn()} />);
    const groups = screen.getAllByRole("radiogroup");
    expect(within(groups[0]!).getAllByRole("radio")[0]!.getAttribute("aria-checked")).toBe("true");
    expect(within(groups[3]!).getAllByRole("radio")[2]!.getAttribute("aria-checked")).toBe("true");
  });
  it("is registered under `criteria_check`", () => {
    expect(fieldRegistry.criteria_check).toBe(CriteriaCheckField);
  });
});
