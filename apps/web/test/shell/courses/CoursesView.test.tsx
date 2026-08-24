import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";

vi.mock("@/api", async (orig) => {
  const real = await orig<typeof import("@/api")>();
  return { ...real, api: { ...real.api, listCourses: vi.fn(), getCourseProgress: vi.fn() } };
});

import { api } from "@/api";
import { CoursesView } from "@/shell/courses/CoursesView";

const course = { slug: "co1", branch: "批判性思维", title: "一条网络信息，该不该信", blurb: "从一句…出发", time_label: "约 40 分钟", card_ids: ["craap", "concession", "toulmin", "sift"], step_count: 3 };

describe("CoursesView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });
  it("renders courses from the API", async () => {
    (api.listCourses as any).mockResolvedValue([course]);
    render(<CoursesView />);
    expect(await screen.findByText("一条网络信息，该不该信")).toBeInTheDocument();
    // The three metadata facts now render as separate coloured tags, not one
    // grey run of text.
    expect(screen.getByText("3 个任务")).toBeInTheDocument();
    expect(screen.getByText("4 个工具")).toBeInTheDocument();
    expect(screen.getByText("约 40 分钟")).toBeInTheDocument();
  });
  it("renders the empty state when there are no courses", async () => {
    (api.listCourses as any).mockResolvedValue([]);
    render(<CoursesView />);
    expect(await screen.findByText("课程正在准备中，很快上线。")).toBeInTheDocument();
  });
  it("renders the cover image when coverUrl is present", async () => {
    (api.listCourses as any).mockResolvedValue([{ ...course, coverUrl: "https://mind-oss.example.com/web/cover.webp?sig=abc" }]);
    render(<CoursesView />);
    const img = await screen.findByRole("img");
    expect(img).toHaveAttribute("src", "https://mind-oss.example.com/web/cover.webp?sig=abc");
  });
  it("falls back to the gradient face (no <img>) when coverUrl is absent", async () => {
    (api.listCourses as any).mockResolvedValue([course]);
    render(<CoursesView />);
    await screen.findByText("一条网络信息，该不该信");
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });
});
