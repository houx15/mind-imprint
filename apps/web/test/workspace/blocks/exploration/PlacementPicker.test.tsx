import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { PlacementPicker } from "../../../../src/workspace/blocks/exploration/PlacementPicker";

const qs = [
  { id: "q1", text: "主问题一", parentId: null },
  { id: "q1a", text: "子问题 A", parentId: "q1" },
  { id: "q2", text: "主问题二", parentId: null },
];

describe("PlacementPicker", () => {
  it("marks the suggested question and shows its reason", () => {
    render(<PlacementPicker questions={qs} suggestedLeadId="q1a" reason="贴子问题A" onPick={() => {}} />);
    expect(screen.getByText("贴子问题A")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /子问题 A/ })).toHaveAttribute("data-suggested", "true");
  });

  it("picks a question id", () => {
    const onPick = vi.fn();
    render(<PlacementPicker questions={qs} suggestedLeadId={null} reason="" onPick={onPick} />);
    fireEvent.click(screen.getByRole("button", { name: /主问题二/ }));
    expect(onPick).toHaveBeenCalledWith("q2");
  });

  it("picks 未归类 (null)", () => {
    const onPick = vi.fn();
    render(<PlacementPicker questions={qs} suggestedLeadId={null} reason="" onPick={onPick} />);
    fireEvent.click(screen.getByRole("button", { name: /先放进未归类/ }));
    expect(onPick).toHaveBeenCalledWith(null);
  });
});
