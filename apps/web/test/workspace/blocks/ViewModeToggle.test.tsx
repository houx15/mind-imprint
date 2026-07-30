import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ViewModeToggle } from "@/workspace/blocks/ReadingBlock";

// EB · the 探索图谱 toggle advertises pending leads/dangling so the rabbit-hole
// graph is discoverable — but opening it stays the student's click (不操纵).
describe("ViewModeToggle (EB)", () => {
  it("shows the lead/dangling count badge on 探索图谱 when signal > 0", () => {
    render(<ViewModeToggle mode="list" onChange={() => {}} signal={3} />);
    expect(screen.getByText("3")).toBeInTheDocument();
  });

  it("shows no badge when there is nothing pending", () => {
    render(<ViewModeToggle mode="list" onChange={() => {}} signal={0} />);
    expect(screen.queryByText("0")).not.toBeInTheDocument();
  });

  it("switching to the graph is an explicit click (no auto-switch)", () => {
    const onChange = vi.fn();
    render(<ViewModeToggle mode="list" onChange={onChange} signal={5} />);
    // rendering with a signal does NOT switch — only the click does
    expect(onChange).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: /探索图谱/ }));
    expect(onChange).toHaveBeenCalledWith("graph");
  });
});
