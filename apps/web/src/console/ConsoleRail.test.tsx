import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConsoleRail } from "./ConsoleRail";

describe("ConsoleRail", () => {
  it("renders 班级 and 设置 tabs and marks the active one", () => {
    render(<ConsoleRail tab="classes" onTab={() => {}} />);
    const tabs = screen.getAllByRole("tab");
    expect(tabs).toHaveLength(2);
    expect(screen.getByText("班级")).toBeInTheDocument();
    expect(screen.getByText("设置")).toBeInTheDocument();
    expect(screen.getByText("班级").closest('[role="tab"]')).toHaveAttribute("aria-selected", "true");
  });

  it("calls onTab when a tab is clicked", async () => {
    const onTab = vi.fn();
    render(<ConsoleRail tab="classes" onTab={onTab} />);
    await userEvent.click(screen.getByText("设置"));
    expect(onTab).toHaveBeenCalledWith("settings");
  });
});
