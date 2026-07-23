import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { WorkOrderItem, type WorkOrderRow } from "@/studio/WorkOrder";

const base: WorkOrderRow = {
  interventionId: "iv1",
  label: "表F 评估",
  evidence: "两个来源可信。",
  missing: "没写出各自的作用与风险。",
  fix: "在信源档案里补上每条的作用与风险",
  disposition: null,
};

describe("WorkOrderItem", () => {
  it("renders a band chip when band is present", () => {
    render(<WorkOrderItem row={{ ...base, band: "5–6 段" }} />);
    expect(screen.getByText("5–6 段")).toBeInTheDocument();
  });

  it("renders NO chip when band is absent (spot-check rows have no band)", () => {
    const { container } = render(<WorkOrderItem row={base} />);
    // The label must be the only text in the header row.
    expect(screen.getByText("表F 评估")).toBeInTheDocument();
    expect(container.querySelectorAll("span")).toHaveLength(1);
  });

  it("requires a ≥15-rune reason before a disposition can be recorded", async () => {
    const onDisposition = vi.fn();
    render(<WorkOrderItem row={base} onDisposition={onDisposition} />);
    await userEvent.click(screen.getByText("我来改"));
    const box = screen.getByPlaceholderText("写下你的理由（至少 15 字）");
    await userEvent.type(box, "太短了");
    expect(screen.getByRole("button", { name: "记录处置" })).toBeDisabled();
    await userEvent.clear(box);
    await userEvent.type(box, "这条来源只能说明现象存在，不能说明成因，我要改写这一段。");
    await userEvent.click(screen.getByRole("button", { name: "记录处置" }));
    expect(onDisposition).toHaveBeenCalledWith("iv1", "rewrite", expect.stringContaining("只能说明现象存在"));
  });
});
