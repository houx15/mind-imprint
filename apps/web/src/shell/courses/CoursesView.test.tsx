import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";

vi.mock("../../api", async (orig) => {
  const real = await orig<typeof import("../../api")>();
  return { ...real, api: { ...real.api, listCourses: vi.fn(), getCourseProgress: vi.fn() } };
});

import { api } from "../../api";
import { CoursesView } from "./CoursesView";

const course = { id: "co1", branch: "批判性思维", title: "一条网络信息，该不该信", blurb: "从一句…出发", tasks_count: 3, tools_count: 4, time_label: "约 40 分钟", step_count: 4 };

describe("CoursesView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });
  it("renders courses from the API", async () => {
    (api.listCourses as any).mockResolvedValue([course]);
    render(<CoursesView />);
    expect(await screen.findByText("一条网络信息，该不该信")).toBeInTheDocument();
    expect(screen.getByText("3 个任务 · 4 个工具")).toBeInTheDocument();
    expect(screen.getByText("约 40 分钟")).toBeInTheDocument();
  });
  it("renders the empty state when there are no courses", async () => {
    (api.listCourses as any).mockResolvedValue([]);
    render(<CoursesView />);
    expect(await screen.findByText("课程正在准备中，很快上线。")).toBeInTheDocument();
  });
});
