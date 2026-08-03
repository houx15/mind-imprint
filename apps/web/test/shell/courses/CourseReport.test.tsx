import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { CourseReport as CourseReportT, CardCatalogEntry } from "@mind-imprint/contracts";

vi.mock("@/api", async (orig) => {
  const real = await orig<typeof import("@/api")>();
  return {
    ...real,
    api: {
      ...real.api,
      getCourseReport: vi.fn(),
      getCardsCatalog: vi.fn(),
    },
  };
});

import { api } from "@/api";
import { CourseReport, formatSpent } from "@/shell/courses/CourseReport";

const report: CourseReportT = {
  title: "一条网络信息，该不该信",
  goal: "学会在下判断前先追问信息从哪里来",
  teaching_thread: "从接受说法，转向追问说法的来源。",
  completedStepTitles: ["建立停一下的习惯", "教横向溯源"],
  cardIds: ["concession"],
  secondsSpent: 185,
  quiz: { total: 4, correct: 3 },
};

const catalogCard: CardCatalogEntry = {
  cardId: "concession",
  name: "让步段 · 以退为进",
  nameEn: "Concession",
  category: "论证结构",
  purpose: "…",
  stages: [],
  example: "",
  hasAsset: false,
  coverUrl: "",
  courseId: "",
  encountered: true,
  score: 0,
  stars: 3,
  uses: 1,
  surfaces: ["course"],
  lastUsed: "",
};

describe("CourseReport", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.getCourseReport as any).mockResolvedValue(report);
    (api.getCardsCatalog as any).mockResolvedValue({ cards: [catalogCard], theme: "light" });
  });

  it("renders the hero, learned goal/thread/steps, card chip, time, and quiz score", async () => {
    render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);

    expect(await screen.findByText("一条网络信息，该不该信")).toBeInTheDocument();
    expect(screen.getByText("学习报告 · 课程完成")).toBeInTheDocument();

    // 学到了什么
    expect(screen.getByText("你学到了什么")).toBeInTheDocument();
    expect(screen.getByText("学会在下判断前先追问信息从哪里来")).toBeInTheDocument();
    expect(screen.getByText("从接受说法，转向追问说法的来源。")).toBeInTheDocument();
    expect(screen.getByText("建立停一下的习惯")).toBeInTheDocument();
    expect(screen.getByText("教横向溯源")).toBeInTheDocument();

    // 学到的工具卡 — chip label comes from the catalog, not the raw card id
    expect(screen.getByText("学到的工具卡")).toBeInTheDocument();
    expect(await screen.findByText("让步段 · 以退为进")).toBeInTheDocument();
    expect(screen.queryByText("concession")).toBeNull();

    // 用时
    expect(screen.getByText("3 分 5 秒")).toBeInTheDocument();

    // 小测表现
    expect(screen.getByText("3 / 4")).toBeInTheDocument();
  });

  it("falls back to the raw card id when the catalog fetch fails, without blocking the report", async () => {
    (api.getCardsCatalog as any).mockRejectedValue(new Error("boom"));
    render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);
    expect(await screen.findByText("一条网络信息，该不该信")).toBeInTheDocument();
    expect(await screen.findByText("concession")).toBeInTheDocument();
  });

  it("omits the 学到的工具卡 block when no cards were part of the course", async () => {
    (api.getCourseReport as any).mockResolvedValue({ ...report, cardIds: [] });
    render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);
    await screen.findByText("一条网络信息，该不该信");
    expect(screen.queryByText("学到的工具卡")).toBeNull();
  });

  it("wires the two actions", async () => {
    const onBack = vi.fn(); const onPortal = vi.fn();
    render(<CourseReport courseId="co1" onBackToCourses={onBack} onGoPortal={onPortal} />);
    await screen.findByText("一条网络信息，该不该信");
    fireEvent.click(screen.getByText("返回课程"));
    expect(onBack).toHaveBeenCalled();
    fireEvent.click(screen.getByText(/去写作工作室/));
    expect(onPortal).toHaveBeenCalled();
  });

  it("shows an honest error when the report fails to load", async () => {
    (api.getCourseReport as any).mockRejectedValue(new Error("boom"));
    render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);
    expect(await screen.findByText("学习报告暂时没能生成，稍后再看看。")).toBeInTheDocument();
  });

  // No DualAxisReport / LLM assessment anywhere on this surface anymore —
  // the course report is now simple stats, not a diagnostic evaluation.
  it("never renders an ability/DualAxisReport section", async () => {
    render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);
    await screen.findByText("一条网络信息，该不该信");
    expect(screen.queryByText("能力评估")).toBeNull();
    expect(screen.queryByText(/两轴永不合成总分/)).toBeNull();
  });
});

describe("formatSpent", () => {
  it("formats sub-minute durations in seconds", () => {
    expect(formatSpent(45)).toBe("45 秒");
  });
  it("formats whole minutes without a seconds remainder", () => {
    expect(formatSpent(180)).toBe("约 3 分钟");
  });
  it("formats minutes with a leftover seconds remainder", () => {
    expect(formatSpent(185)).toBe("3 分 5 秒");
  });
  it("formats zero as 0 分钟", () => {
    expect(formatSpent(0)).toBe("0 分钟");
  });
});
