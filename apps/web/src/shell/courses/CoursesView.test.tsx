import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

vi.mock("@/api", async (orig) => {
  const real = await orig<typeof import("@/api")>();
  return {
    ...real,
    api: {
      ...real.api,
      listCourses: vi.fn(),
      getCourseProgress: vi.fn(),
      getCourseHistory: vi.fn(),
    },
  };
});

import { api } from "@/api";
import { CoursesView } from "@/shell/courses/CoursesView";

function course(slug: string, title: string, category: string | null) {
  return {
    slug,
    branch: "批判性思维",
    title,
    blurb: "",
    time_label: "约 30 分钟",
    card_ids: [],
    step_count: 4,
    category,
    introduction: null,
    featuredRank: null,
  };
}

// Catalog order as the list endpoint returns it.
const CATALOG = [
  course("money", "追踪信息背后的利益链", "source-check"),
  course("multimodal", "生成式 AI 时代的多模态信息甄别", "media-literacy"),
  course("stance", "把争议放回证据里", "stance-value"),
];

/** The rendered course cards' slugs, in DOM order (top to bottom). */
function shownSlugs(): string[] {
  return Array.from(document.querySelectorAll("[data-course-card]")).map(
    (el) => el.getAttribute("data-course-card") ?? "",
  );
}

describe("CoursesView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.listCourses as any).mockResolvedValue(CATALOG);
    (api.getCourseProgress as any).mockResolvedValue(null);
    (api.getCourseHistory as any).mockResolvedValue([
      { slug: "stance", status: "in-progress", completedCount: 2, updatedAt: "2026-08-19T10:00:00Z" },
      { slug: "multimodal", status: "in-progress", completedCount: 1, updatedAt: "2026-08-21T09:00:00Z" },
    ]);
  });

  it("defaults to 最近学习 — the courses the student actually touched come first, newest first", async () => {
    render(<CoursesView />);
    await waitFor(() => expect(screen.getByText("追踪信息背后的利益链")).toBeInTheDocument());
    expect(shownSlugs()).toEqual([
      "multimodal", // touched today
      "stance", // touched 2 days ago
      "money", // untouched → catalog order
    ]);
  });

  it("does NOT group the list under category headings — 全部 is one flat list", async () => {
    render(<CoursesView />);
    await waitFor(() => expect(screen.getByText("追踪信息背后的利益链")).toBeInTheDocument());
    // The category names still exist as filter CHIPS; none of them is a section
    // heading over part of the list. Each appears exactly once (its chip).
    expect(screen.getAllByText("信源核查")).toHaveLength(1);
    expect(screen.getAllByText("媒介与信息素养")).toHaveLength(1);
  });

  it("searches by course name and shows an empty state rather than the whole catalog", async () => {
    const user = userEvent.setup();
    render(<CoursesView />);
    await waitFor(() => expect(screen.getByText("追踪信息背后的利益链")).toBeInTheDocument());

    await user.type(screen.getByLabelText("搜索课程名称"), "利益链");
    expect(shownSlugs()).toEqual(["money"]);

    await user.clear(screen.getByLabelText("搜索课程名称"));
    await user.type(screen.getByLabelText("搜索课程名称"), "量子力学");
    expect(shownSlugs()).toEqual([]);
    expect(screen.getByText(/没有名字里含/)).toBeInTheDocument();
  });

  it("switches to 按名称 order on demand", async () => {
    const user = userEvent.setup();
    render(<CoursesView />);
    await waitFor(() => expect(screen.getByText("追踪信息背后的利益链")).toBeInTheDocument());

    await user.click(screen.getByRole("button", { name: "按名称" }));
    const byName = [...CATALOG].sort((a, b) => a.title.localeCompare(b.title, "zh-Hans-CN")).map((c) => c.slug);
    expect(shownSlugs()).toEqual(byName);
  });

  it("still renders the catalog when the history call fails — recency is a nice-to-have, not a dependency", async () => {
    (api.getCourseHistory as any).mockRejectedValue(new Error("boom"));
    render(<CoursesView />);
    await waitFor(() => expect(screen.getByText("追踪信息背后的利益链")).toBeInTheDocument());
    expect(shownSlugs()).toEqual(CATALOG.map((c) => c.slug));
  });
});
