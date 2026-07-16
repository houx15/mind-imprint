import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { LeftRail } from "./LeftRail";

describe("LeftRail", () => {
  it("renders the five pillar tabs in order", () => {
    render(<LeftRail tab="studio" onTab={() => {}} />);
    const tabs = screen.getAllByRole("tab");
    expect(tabs.map((t) => t.textContent)).toEqual(["课程", "聊天", "工作室", "成长报告", "设置"]);
  });

  it("marks the active tab via aria-selected", () => {
    render(<LeftRail tab="growth" onTab={() => {}} />);
    const tabs = screen.getAllByRole("tab");
    const growth = tabs.find((t) => t.textContent?.includes("成长报告"))!;
    expect(growth.getAttribute("aria-selected")).toBe("true");
  });

  it("fires onTab when nav items are clicked", () => {
    const onTab = vi.fn();
    render(<LeftRail tab="studio" onTab={onTab} />);
    fireEvent.click(screen.getByText("课程"));
    expect(onTab).toHaveBeenCalledWith("courses");
    fireEvent.click(screen.getByText("设置"));
    expect(onTab).toHaveBeenCalledWith("settings");
  });

  it("renders the five shipped rail items per the binding design", () => {
    render(<LeftRail tab="studio" onTab={() => {}} />);
    expect(screen.getByText("聊天")).toBeTruthy();
    expect(screen.getByText("课程")).toBeTruthy();
    expect(screen.getByText("工作室")).toBeTruthy();
    expect(screen.getByText("成长报告")).toBeTruthy();
    expect(screen.getByText("设置")).toBeTruthy();
    // The retired labels are gone.
    expect(screen.queryByText("批判思维")).toBeNull();
    expect(screen.queryByText("我的评估")).toBeNull();
  });
});
