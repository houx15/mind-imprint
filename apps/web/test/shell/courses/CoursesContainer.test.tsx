import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { CourseSummary, CoursePlayerPayload, CourseReport as CourseReportT } from "@mind-imprint/contracts";

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
  slug: "co1", branch: "批判性思维", title: "一条网络信息，该不该信", blurb: "从一句…出发", time_label: "约 40 分钟", card_ids: ["concession"], step_count: 1,
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

const report: CourseReportT = {
  title: "一条网络信息，该不该信",
  goal: "学会先追问信息的来源",
  teaching_thread: "从接受说法转向追问来源。",
  completedStepTitles: ["开场"],
  cardIds: ["concession"],
  secondsSpent: 120,
  quiz: { total: 1, correct: 1 },
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
    (api.getCourseReport as any).mockResolvedValue(report);
    (api.getCardsCatalog as any).mockResolvedValue({ cards: [], theme: "light" });
  });

  it("opens the player when a course is clicked, and returns to the grid", async () => {
    render(<CoursesContainer />);
    fireEvent.click(await screen.findByText("开始学习"));
    expect(await screen.findByText("开场正文。")).toBeInTheDocument(); // player step
    fireEvent.click(screen.getByText("课程")); // back
    expect(await screen.findByText("系统地学会一种思考方式")).toBeInTheDocument(); // grid header
  });

  it("shows the course report after finishing the last (only) step", async () => {
    render(<CoursesContainer />);
    fireEvent.click(await screen.findByText("开始学习"));
    await screen.findByText("开场正文。");
    fireEvent.click(screen.getByLabelText("完成课程"));
    expect(await screen.findByText("学习报告 · 课程完成")).toBeInTheDocument();
    expect(await screen.findByText("学会先追问信息的来源")).toBeInTheDocument();
    fireEvent.click(screen.getByText("返回课程"));
    expect(await screen.findByText("系统地学会一种思考方式")).toBeInTheDocument(); // back to grid
  });
});
