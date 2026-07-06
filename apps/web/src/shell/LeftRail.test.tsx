import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { LeftRail } from "./LeftRail";

describe("LeftRail", () => {
  it("renders the four pillar tabs in order", () => {
    render(<LeftRail tab="tasks" onTab={() => {}} />);
    const tabs = screen.getAllByRole("tab");
    expect(tabs.map((t) => t.textContent)).toEqual(["课程", "批判思维", "我的评估", "设置"]);
  });

  it("marks the active tab via aria-selected", () => {
    render(<LeftRail tab="records" onTab={() => {}} />);
    const tabs = screen.getAllByRole("tab");
    const records = tabs.find((t) => t.textContent?.includes("我的评估"))!;
    expect(records.getAttribute("aria-selected")).toBe("true");
  });

  it("fires onTab when nav items are clicked", () => {
    const onTab = vi.fn();
    render(<LeftRail tab="tasks" onTab={onTab} />);
    fireEvent.click(screen.getByText("课程"));
    expect(onTab).toHaveBeenCalledWith("courses");
    fireEvent.click(screen.getByText("设置"));
    expect(onTab).toHaveBeenCalledWith("settings");
  });
});
