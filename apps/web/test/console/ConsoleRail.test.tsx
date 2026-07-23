import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConsoleRail } from "@/console/ConsoleRail";

describe("ConsoleRail", () => {
  it("admin sees all five tabs", () => {
    render(<ConsoleRail role="admin" tab="overview" onTab={() => {}} />);
    const tabs = screen.getAllByRole("tab");
    expect(tabs).toHaveLength(5);
    ["概览", "班级", "教师", "导入", "设置"].forEach((t) => expect(screen.getByText(t)).toBeInTheDocument());
    expect(screen.getByText("概览").closest('[role="tab"]')).toHaveAttribute("aria-selected", "true");
  });

  it("teacher sees only 班级 and 设置", () => {
    render(<ConsoleRail role="teacher" tab="classes" onTab={() => {}} />);
    const tabs = screen.getAllByRole("tab");
    expect(tabs).toHaveLength(2);
    expect(screen.getByText("班级")).toBeInTheDocument();
    expect(screen.queryByText("概览")).not.toBeInTheDocument();
    expect(screen.queryByText("导入")).not.toBeInTheDocument();
  });

  it("calls onTab when a tab is clicked", async () => {
    const onTab = vi.fn();
    render(<ConsoleRail role="admin" tab="overview" onTab={onTab} />);
    await userEvent.click(screen.getByText("导入"));
    expect(onTab).toHaveBeenCalledWith("import");
  });
});
