import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import type { Course } from "@mind-imprint/contracts";

vi.mock("../../api", async (orig) => {
  const real = await orig<typeof import("../../api")>();
  return {
    ...real,
    api: {
      ...real.api,
      getCourse: vi.fn(),
      getCourseProgress: vi.fn(),
      getCourseAssessment: vi.fn(),
      generateCourseAssessment: vi.fn(),
      getCourseSession: vi.fn(),
    },
  };
});

import { api } from "../../api";
import { CourseReport } from "./CourseReport";

// Real DTO shape (checked against packages/contracts/src/dualAxisReport.ts):
// a full DualAxisReport, reusing the exact fixture shape from Task 8's
// apps/web/src/shell/report/DualAxisReport.test.tsx so it parses .strict()
// and renders via the shared <DualAxisReport> component.
const assessmentFixture = {
  depthAxis: { dims: [
    { code: "D1", name: "任务理解与问题表述", score: 3, evidence: "限定判断", promptEvidence: "R4" },
    { code: "D3", name: "证据与信源意识", score: 2, evidence: "学生在第 3 轮追问了来源的作者与机构。", promptEvidence: "" },
    { code: "D4", name: "论证结构意识", score: 3, evidence: "warrant", promptEvidence: "" },
    { code: "D5", name: "反馈理解与修改理由", score: 3, evidence: "理由", promptEvidence: "" },
  ], subtotal: 11 },
  autonomyAxis: { code: "D2", name: "学生主体性 / AI 依赖度", observation: "入场即设边界", anchoredSignals: ["R1"], promptedSignals: ["R3"], adversaryInvites: 0, promptEvidence: "" },
  crossAxis: { code: "D6", name: "元认知与反思", depthLevel: "L3", initiative: "引导后", prose: "能反思，尚未自发反思", promptEvidence: "" },
  solo: [{ round: 4, excerpt: "限定判断", level: "L3", rationale: "组织者", initiative: "自发" }],
  promptLens: { directiveRounds: 3, totalRounds: 10, boundarySettings: 3, adversaryInvites: 0,
    questions: [{ title: "一问 · 任务说清了吗", body: "…" }],
    bestPrompt: { round: 8, quote: "检查是否回扣 thesis", annotation: "齐备" },
    takeaway: { round: 0, quote: "扮演苛刻审稿人", annotation: "P4 模板" },
    perRound: [{ round: 1, tier: "P3", label: "要过程·设边界" }] },
  timeline: [{ round: 1, task: "上传草稿", prompt: "不要直接重写", pTag: "P3", dimTags: ["D1=2"] }],
  keyEvidence: [{ label: "任务理解", quote: "我想把 thesis 改成…" }],
  guidance: { anchored: "主动限定 thesis", prompted: "SIFT 核查", risk: "D3 仍停留在来源等级", nextSteps: [{ title: "下一步强化 D3", body: "跑一张 SIFT 记录" }] },
  narrative: "这门课里，你从接受说法转向了追问说法的来源。",
  axiom: "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
  generatedAt: "2026-07-17T09:00:00Z",
};

const course: Course = {
  id: "co1", branch: "批判性思维", title: "一条网络信息，该不该信", blurb: "…", tasks_count: 3, tools_count: 4, time_label: "约 40 分钟", step_count: 3,
  steps: [
    { id: "s0", course_id: "co1", ordinal: 0, kind: "teaching", purpose: "建立停一下的习惯", assets: [], challenge_type: null, authored_content: {} },
    { id: "s1", course_id: "co1", ordinal: 1, kind: "teaching", purpose: "教横向溯源", assets: [], challenge_type: null, authored_content: {} },
    { id: "s2", course_id: "co1", ordinal: 2, kind: "challenge", purpose: "亲手核查一处断言", assets: [], challenge_type: "verify_claim", authored_content: { title: "现在轮到你" } },
  ],
};

describe("CourseReport", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.getCourse as any).mockResolvedValue(course);
    (api.getCourseProgress as any).mockResolvedValue({ course_id: "co1", current_ordinal: 2, completed_ordinals: [0, 1, 2], updated_at: "" });
    (api.getCourseAssessment as any).mockResolvedValue(assessmentFixture);
    (api.generateCourseAssessment as any).mockResolvedValue(assessmentFixture);
    (api.getCourseSession as any).mockResolvedValue({
      id: "sess1", courseId: "co1", phase: "done", phaseTitle: "完成", status: "finished",
      messages: [], openCards: [], collectedCards: [],
    });
  });

  it("renders the hero, stats, learnings and challenge review", async () => {
    render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);
    expect(await screen.findByText("一条网络信息，该不该信")).toBeInTheDocument();
    expect(screen.getByText("学习报告 · 课程完成")).toBeInTheDocument();
    expect(screen.getByText("约 40 分钟")).toBeInTheDocument(); // 用时 stat
    expect(screen.getByText("建立停一下的习惯")).toBeInTheDocument(); // a learning
    expect(screen.getByText("你学到了什么")).toBeInTheDocument();
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

  // A1: 挑战通过 counted challenges that EXIST, not ones the student reached —
  // every student was told they passed every challenge, including ones never
  // seen. 通过 is defined as engagement (DEC-A1.5): reached, i.e. the ordinal is
  // in the server-recorded completed_ordinals.
  it("counts only challenges the student actually reached", async () => {
    (api.getCourse as any).mockResolvedValue({
      ...course,
      steps: [
        { id: "s0", course_id: "co1", ordinal: 0, kind: "teaching", purpose: "看一遍", assets: [], challenge_type: null, authored_content: {} },
        { id: "s1", course_id: "co1", ordinal: 1, kind: "challenge", purpose: "核查一处断言", assets: [], challenge_type: "verify_claim", authored_content: {} },
        { id: "s2", course_id: "co1", ordinal: 2, kind: "challenge", purpose: "找一个反例", assets: [], challenge_type: "verify_claim", authored_content: {} },
      ],
    });
    (api.getCourseProgress as any).mockResolvedValue({ course_id: "co1", current_ordinal: 1, completed_ordinals: [0, 1], updated_at: "" });
    (api.getCourseAssessment as any).mockResolvedValue(null);
    (api.generateCourseAssessment as any).mockResolvedValue(assessmentFixture);

    render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);

    const tile = await screen.findByText("挑战通过");
    // Reached ordinal 1 only — ordinal 2 was never opened.
    expect(tile.parentElement).toHaveTextContent("1");

    // The per-row ✓ must track the SAME reached/unreached split as the tile
    // count above — find each challenge row by its purpose text, then assert
    // the inline checkmark <svg> is present only in the reached row. Each row
    // always renders one <svg> for its icon badge (a sibling wrapper, not a
    // direct child of the row), so querying the row's DIRECT-CHILD svg
    // isolates the conditional checkmark from that always-present icon.
    const reachedPurpose = await screen.findByText("核查一处断言");
    const unreachedPurpose = screen.getByText("找一个反例");
    const reachedRow = reachedPurpose.parentElement!.parentElement as HTMLElement;
    const unreachedRow = unreachedPurpose.parentElement!.parentElement as HTMLElement;
    expect(reachedRow.querySelector(":scope > svg")).not.toBeNull();
    expect(unreachedRow.querySelector(":scope > svg")).toBeNull();
  });

  it("generates the report once when none exists, and renders the DualAxisReport body", async () => {
    (api.getCourseAssessment as any).mockResolvedValue(null);
    (api.generateCourseAssessment as any).mockResolvedValue(assessmentFixture);

    render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);

    expect(await screen.findByText("能力评估")).toBeInTheDocument();
    expect(screen.getByText("按 SOLO 四级 · 来自这门课里你的表现")).toBeInTheDocument();
    expect(screen.getByText("证据与信源意识")).toBeInTheDocument();
    expect(screen.getByText("学生在第 3 轮追问了来源的作者与机构。")).toBeInTheDocument();
    await waitFor(() => expect(api.generateCourseAssessment).toHaveBeenCalledTimes(1));
  });

  it("does not regenerate when a report already exists", async () => {
    (api.getCourseAssessment as any).mockResolvedValue(assessmentFixture);
    render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);
    expect(await screen.findByText("能力评估")).toBeInTheDocument();
    expect(api.generateCourseAssessment).not.toHaveBeenCalled();
  });

  // RL-5: no rank, no aggregate anywhere in the DOM. The DualAxisReport DOES
  // legitimately say "总分" once — inside its axiom disclaimer ("两轴永不合成
  // 总分…"), which asserts the ABSENCE of a combined total, not a score.
  it("renders no rank or aggregate score, and states via the axiom that the two axes are never combined", async () => {
    (api.getCourseAssessment as any).mockResolvedValue(assessmentFixture);
    const { container } = render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);
    await screen.findByText("能力评估");
    expect(container.textContent).not.toMatch(/排名|平均分|\d+\s*\/\s*40/);
    expect(screen.getByText(/两轴永不合成总分/)).toBeInTheDocument();
  });

  // An NA cross-axis level is rendered as 未涉及, never as a bare "NA" or a
  // fabricated "L0" — RL-5's diagnostic-not-graded posture (mirrors the OLD
  // per-dimension NA-rendering contract, now expressed via SoloLevel "NA").
  it("renders a NA level as 未涉及, never as a bare level string", async () => {
    (api.getCourseAssessment as any).mockResolvedValue({
      ...assessmentFixture,
      crossAxis: { ...assessmentFixture.crossAxis, depthLevel: "NA" },
    });
    render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);
    expect(await screen.findByText(/未涉及/)).toBeInTheDocument();
    expect(screen.queryByText("L0")).toBeNull();
  });

  // Generation failure is honest — never a fabricated report.
  it("shows an honest error when generation fails", async () => {
    (api.getCourseAssessment as any).mockResolvedValue(null);
    (api.generateCourseAssessment as any).mockRejectedValue(new Error("boom"));
    render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);
    expect(await screen.findByText("能力评估暂时没能生成，稍后再看看。")).toBeInTheDocument();
  });

  // Task 6: 收集到的工具 renders one pill per completed card, named via
  // CARD_REGISTRY — never a raw card id.
  it("renders one pill per collected card, named via CARD_REGISTRY", async () => {
    (api.getCourseSession as any).mockResolvedValue({
      id: "sess1", courseId: "co1", phase: "done", phaseTitle: "完成", status: "finished",
      messages: [], openCards: [], collectedCards: [{ cardId: "sift_craap" }],
    });
    render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);
    await screen.findByText("能力评估");
    expect(await screen.findByText("收集到的工具")).toBeInTheDocument();
    expect(screen.getByText("SIFT×CRAAP 信息核查")).toBeInTheDocument();
  });

  // A1: 工具收集 counted course.tools_count — the static authored catalogue
  // number (4, same as the course card's "4 个工具") — while the block right
  // below it showed only what was actually collected. A student who skipped
  // the one offered card saw "工具收集 4" directly above an omitted 收集到的
  // 工具 block: the page contradicted itself. This fixture sets tools_count=4
  // but collects only 1 card, so it fails if the tile ever reverts to
  // course.tools_count (which would render "4", not "1").
  it("counts collected tools, not the course's authored tools_count", async () => {
    (api.getCourse as any).mockResolvedValue({ ...course, tools_count: 4 });
    (api.getCourseSession as any).mockResolvedValue({
      id: "sess1", courseId: "co1", phase: "done", phaseTitle: "完成", status: "finished",
      messages: [], openCards: [], collectedCards: [{ cardId: "sift_craap" }],
    });

    render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);

    const tile = await screen.findByText("工具收集");
    expect(tile.parentElement).toHaveTextContent("1");
    expect(tile.parentElement).not.toHaveTextContent("4");

    // Agreement check: the tile's count must equal the number of pills the
    // 收集到的工具 block actually renders below it.
    expect(await screen.findByText("收集到的工具")).toBeInTheDocument();
    expect(screen.getByText("SIFT×CRAAP 信息核查")).toBeInTheDocument();
  });

  // No fabricated empty state — the design has none, and an empty block would
  // wrongly imply the student collected nothing when they may simply not have
  // reached a card yet.
  it("omits the 收集到的工具 block when no cards were collected", async () => {
    render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);
    await screen.findByText("能力评估");
    expect(screen.queryByText("收集到的工具")).toBeNull();
  });

  // commit 7a45183: the gate must check the CARD_REGISTRY-filtered list, not
  // collectedCardIds.length — a session can carry a cardId with no matching
  // spec (e.g. a retired/renamed card), and that must render as empty too,
  // never a card block with zero pills.
  it("omits the 收集到的工具 block when every collected cardId is absent from CARD_REGISTRY", async () => {
    (api.getCourseSession as any).mockResolvedValue({
      id: "sess1", courseId: "co1", phase: "done", phaseTitle: "完成", status: "finished",
      messages: [], openCards: [], collectedCards: [{ cardId: "no-such-card" }],
    });
    render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);
    await screen.findByText("能力评估");
    expect(screen.queryByText("收集到的工具")).toBeNull();
  });
});
