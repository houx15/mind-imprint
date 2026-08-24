import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { CourseSummary, CoursePlayerPayload } from "@mind-imprint/contracts";
import { ApiError } from "@/api/client";

// These tests exercise the LEGACY player flow, so the 2.0 definition endpoint
// must 404 (the authoritative "no 2.0 definition → legacy course" signal that
// PlayerRouter routes on). Without this, getCourseDefinition would hit a real
// fetch that rejects non-404 → PlayerRouter's P2-10 error state, not legacy.
vi.mock("@/api/courseDefinition", () => ({
  getCourseDefinition: vi.fn().mockRejectedValue(new ApiError("not_found", "no 2.0 definition", 404)),
}));

vi.mock("@/api", async (orig) => {
  const real = await orig<typeof import("@/api")>();
  return {
    ...real,
    api: {
      ...real.api,
      listCourses: vi.fn(),
      getCourseProgress: vi.fn(),
      getCourse: vi.fn(),
      saveCourseProgress: vi.fn(),
      answerCourseQuiz: vi.fn(),
      courseAsk: vi.fn(),
      getCourseReport: vi.fn(),
      getCardsCatalog: vi.fn(),
    },
  };
});

import { api } from "@/api";
import { CoursesContainer } from "@/shell/courses/CoursesContainer";

const summary: CourseSummary = {
  slug: "co1", branch: "批判性思维", title: "一条网络信息，该不该信", blurb: "从一句…出发", time_label: "约 40 分钟", card_ids: ["concession"], step_count: 1, coverUrl: "",
  category: null, introduction: null, featuredRank: null, progress: null
};

const payload: CoursePlayerPayload = {
  slug: "co1",
  title: "一条网络信息，该不该信",
  branch: "批判性思维",
  cardIds: ["concession"],
  structure: {
    id: "co1", title: "一条网络信息，该不该信", course_goal: "", teaching_thread: "",
    steps: [{ id: "s0", title: "第一步", materials: [] }],
    asset_library: [],
  },
  renderCache: {
    version: "1", courseId: "co1", courseTitle: "一条网络信息，该不该信",
    steps: [{ stepId: "s0", content: { title: "开场", subtitle: "s", segments: [{ kind: "teaching", flow_block_id: "", text: "开场正文。", asset_ids: [], items: [] }], interactions: [], board: [] } }],
  },
  audioKeys: {},
};

async function* gen(events: unknown[]) {
  for (const e of events) yield e;
}

describe("CoursesContainer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.listCourses as any).mockResolvedValue([summary]);
    (api.getCourseProgress as any).mockRejectedValue(new Error("no progress yet"));
    (api.getCourse as any).mockResolvedValue(payload);
    (api.saveCourseProgress as any).mockResolvedValue({ course_slug: "co1", current_ordinal: 0, completed_ordinals: [0], started_at: null, completed_at: null, updated_at: "" });
    (api.answerCourseQuiz as any).mockResolvedValue(undefined);
    (api.courseAsk as any).mockImplementation(() => gen([]));
    (api.getCourseReport as any).mockResolvedValue({
      title: "一条网络信息，该不该信",
      goal: "学会溯源",
      teaching_thread: "",
      completedStepTitles: ["第一步"],
      cardIds: [],
      secondsSpent: 90,
      quiz: { total: 0, correct: 0 },
    });
    (api.getCardsCatalog as any).mockResolvedValue({ cards: [], theme: "light" });
  });

  it("clicking a course from the grid opens its detail page (not the player); the detail CTA enters the player, which exits back to the grid", async () => {
    render(<CoursesContainer />);
    fireEvent.click(await screen.findByText("开始学习")); // grid card click -> detail, NOT the player
    await screen.findByText("从一句…出发"); // detail's blurb fallback (introduction is null) proves the detail page rendered
    expect(screen.queryByText("开场正文。")).toBeNull(); // the player has not mounted yet
    fireEvent.click(screen.getByText("开始学习")); // detail's own CTA -> player
    expect(await screen.findByText("开场正文。")).toBeInTheDocument(); // player step
    fireEvent.click(screen.getByText("课程")); // back
    expect(await screen.findByText("系统地学会一种思考方式")).toBeInTheDocument(); // grid header
  });

  it("opens straight on the detail page for a deep-link (initialCourseId), same landing as a grid click", async () => {
    render(<CoursesContainer initialOpen={{ slug: "co1", mode: "detail" }} />);
    await screen.findByText("从一句…出发"); // detail's blurb fallback renders
    expect(screen.queryByText("开场正文。")).toBeNull(); // NOT the player
    expect(screen.queryByText("系统地学会一种思考方式")).toBeNull(); // NOT the grid
  });

  it("reacts to an initialOpen that arrives AFTER mount (browser Back → /courses/:slug)", async () => {
    // Mount on the grid (no deep-link), as when the courses tab is already open.
    const { rerender } = render(<CoursesContainer initialOpen={null} onCourseConsumed={() => {}} />);
    await screen.findByText("系统地学会一种思考方式"); // grid header confirms we start on the grid

    // The shell's popstate handler feeds a fresh target into the already-mounted
    // container — it must navigate to the detail page, not stay on the grid.
    rerender(<CoursesContainer initialOpen={{ slug: "co1", mode: "detail" }} onCourseConsumed={() => {}} />);
    await screen.findByText("从一句…出发"); // detail landed
    expect(screen.queryByText("系统地学会一种思考方式")).toBeNull(); // no longer the grid
  });

  it("shows the course report after finishing the last (only) step, with a back-to-courses affordance", async () => {
    render(<CoursesContainer />);
    fireEvent.click(await screen.findByText("开始学习")); // grid card -> detail
    await screen.findByText("从一句…出发"); // confirm detail landing before entering the player
    fireEvent.click(screen.getByText("开始学习")); // detail CTA -> player
    await screen.findByText("开场正文。");
    fireEvent.click(screen.getByLabelText("完成课程"));
    expect(await screen.findByText("学习报告 · 课程完成")).toBeInTheDocument(); // the report hero
    fireEvent.click(screen.getByText("返回课程"));
    expect(await screen.findByText("系统地学会一种思考方式")).toBeInTheDocument(); // back to grid
  });
});
