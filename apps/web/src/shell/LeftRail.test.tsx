import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { LeftRail } from "./LeftRail";

describe("LeftRail", () => {
  it("marks the active tab via aria-selected", () => {
    render(<LeftRail tab="records" onTab={() => {}} />);
    const tabs = screen.getAllByRole("tab");
    const records = tabs.find((t) => t.textContent?.includes("记录"))!;
    expect(records.getAttribute("aria-selected")).toBe("true");
  });
  it("fires onTab when a nav item is clicked", () => {
    const onTab = vi.fn();
    render(<LeftRail tab="tasks" onTab={onTab} />);
    fireEvent.click(screen.getByText("设置"));
    expect(onTab).toHaveBeenCalledWith("settings");
  });
});
