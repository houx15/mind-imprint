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

function course(
  slug: string,
  title: string,
  category: string | null,
  progress: { status: string; completedSteps: number; updatedAt: string } | null = null,
) {
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
    progress,
  };
}

// Catalog order as the list endpoint returns it, progress included (the server
// resolves it per student — the client no longer asks course by course).
const CATALOG = [
  course("money", "追踪信息背后的利益链", "source-check"),
  course("multimodal", "生成式 AI 时代的多模态信息甄别", "media-literacy", {
    status: "in-progress",
    completedSteps: 1,
    updatedAt: "2026-08-21T09:00:00Z",
  }),
  course("stance", "把争议放回证据里", "stance-value", {
    status: "in-progress",
    completedSteps: 2,
    updatedAt: "2026-08-19T10:00:00Z",
  }),
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
    (api.getCourseHistory as any).mockResolvedValue([]);
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

  it("still renders the catalog when no course has been touched — recency just leaves catalog order alone", async () => {
    (api.listCourses as any).mockResolvedValue(CATALOG.map((c) => ({ ...c, progress: null })));
    render(<CoursesView />);
    await waitFor(() => expect(screen.getByText("追踪信息背后的利益链")).toBeInTheDocument());
    expect(shownSlugs()).toEqual(CATALOG.map((c) => c.slug));
  });

  it("gets the whole catalog — cards, rings and recency — from ONE request", async () => {
    render(<CoursesView />);
    await waitFor(() => expect(screen.getByText("追踪信息背后的利益链")).toBeInTheDocument());
    expect(api.listCourses).toHaveBeenCalledTimes(1);
    // The N+1 this replaced: one /progress per card, plus a /history call.
    expect(api.getCourseProgress).not.toHaveBeenCalled();
    expect(api.getCourseHistory).not.toHaveBeenCalled();
  });

  it("draws each card's ring from the progress the list already carried", async () => {
    render(<CoursesView />);
    await waitFor(() => expect(screen.getByText("追踪信息背后的利益链")).toBeInTheDocument());
    // multimodal: 1 of 4 steps; stance: 2 of 4; money: untouched → no ring.
    expect(screen.getByText("25%")).toBeInTheDocument();
    expect(screen.getByText("50%")).toBeInTheDocument();
    expect(screen.getAllByText("未开始")).toHaveLength(1);
  });
});
