import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CoursesView } from "./CoursesView";

describe("CoursesView", () => {
  it("renders the header and the mock course card", () => {
    render(<CoursesView />);
    expect(screen.getByText("系统地学会一种思考方式")).toBeInTheDocument();
    expect(screen.getByText("一条网络信息，该不该信")).toBeInTheDocument();
    expect(screen.getByText("3 个任务 · 4 个工具")).toBeInTheDocument();
    expect(screen.getByText("约 40 分钟")).toBeInTheDocument();
    expect(screen.getByText("未开始")).toBeInTheDocument();
    expect(screen.getByText("开始学习")).toBeInTheDocument();
  });

  it("clicking the course CTA is an inert no-op that does not throw", () => {
    render(<CoursesView />);
    expect(() => fireEvent.click(screen.getByText("开始学习"))).not.toThrow();
  });
});
