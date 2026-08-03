import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { CoursePlayerPayload } from "@mind-imprint/contracts";

vi.mock("@/api", async (orig) => {
  const real = await orig<typeof import("@/api")>();
  return {
    ...real,
    api: {
      ...real.api,
      getCourse: vi.fn(),
      getCourseProgress: vi.fn(),
      saveCourseProgress: vi.fn(),
      answerCourseQuiz: vi.fn(),
      courseAsk: vi.fn(),
    },
  };
});

import { api } from "@/api";
import { CoursePlayer } from "@/shell/courses/CoursePlayer";

const payload: CoursePlayerPayload = {
  slug: "info-literacy",
  title: "一条网络信息，该不该信",
  branch: "批判性思维",
  cardIds: ["craap"],
  structure: {
    id: "co1",
    title: "一条网络信息，该不该信",
    course_goal: "",
    teaching_thread: "",
    steps: [
      { id: "s0", title: "第一步", materials: [] },
      { id: "s1", title: "第二步", materials: [] },
    ],
    asset_library: [],
  },
  renderCache: {
    version: "1",
    courseId: "co1",
    courseTitle: "一条网络信息，该不该信",
    steps: [
      { stepId: "s0", content: { title: "第 0 步", subtitle: "", segments: [{ kind: "teaching", flow_block_id: "", text: "第 0 步的正文。", asset_ids: [], items: [] }], interactions: [], board: [] } },
      { stepId: "s1", content: { title: "第 1 步", subtitle: "", segments: [{ kind: "teaching", flow_block_id: "", text: "第 1 步的正文。", asset_ids: [], items: [] }], interactions: [], board: [] } },
    ],
  },
};

// Every real courseAsk stream ends with a `done` frame unconditionally (see
// apps/web/src/api/courses.ts's courseAsk) — a mock stream that omits it
// encodes a shape the real client never produces.
async function* gen(events: unknown[]) {
  for (const e of events) yield e;
}

describe("CoursePlayer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.getCourse as any).mockResolvedValue(payload);
    (api.getCourseProgress as any).mockRejectedValue(new Error("no progress yet"));
    (api.saveCourseProgress as any).mockResolvedValue({ course_slug: "info-literacy", current_ordinal: 1, completed_ordinals: [0], started_at: null, completed_at: null, updated_at: "" });
    (api.answerCourseQuiz as any).mockResolvedValue(undefined);
    (api.courseAsk as any).mockImplementation(() => gen([]));
  });

  it("renders the first step's content via SegmentTimeline", async () => {
    render(<CoursePlayer courseId="info-literacy" onExit={vi.fn()} onFinish={vi.fn()} />);
    expect(await screen.findByTestId("segment-timeline")).toBeInTheDocument();
    expect(await screen.findByText("第 0 步的正文。")).toBeInTheDocument();
  });

  it("exits via the back control", async () => {
    const onExit = vi.fn();
    render(<CoursePlayer courseId="info-literacy" onExit={onExit} onFinish={vi.fn()} />);
    await screen.findByText("第 0 步的正文。");
    fireEvent.click(screen.getByText("课程"));
    expect(onExit).toHaveBeenCalled();
  });

  it("advances to the next step on 下一步, saving current_ordinal", async () => {
    render(<CoursePlayer courseId="info-literacy" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByText("第 0 步的正文。");
    // Step 0 has one segment and no quiz, so it is done immediately → 下一步 enabled.
    fireEvent.click(screen.getByLabelText("下一步"));
    expect(await screen.findByText("第 1 步的正文。")).toBeInTheDocument();
    await waitFor(() => expect(api.saveCourseProgress).toHaveBeenCalledWith("info-literacy", expect.objectContaining({ current_ordinal: 1 })));
  });

  it("resumes at the saved ordinal without rendering step 0 first", async () => {
    (api.getCourseProgress as any).mockResolvedValue({ course_slug: "info-literacy", current_ordinal: 1, completed_ordinals: [0], started_at: null, completed_at: null, updated_at: "" });
    render(<CoursePlayer courseId="info-literacy" onExit={vi.fn()} onFinish={vi.fn()} />);
    expect(await screen.findByText("第 1 步的正文。")).toBeInTheDocument();
    expect(screen.queryByText("第 0 步的正文。")).not.toBeInTheDocument();
  });

  it("clamps a resumed ordinal past the last step", async () => {
    (api.getCourseProgress as any).mockResolvedValue({ course_slug: "info-literacy", current_ordinal: 99, completed_ordinals: [0, 1], started_at: null, completed_at: null, updated_at: "" });
    render(<CoursePlayer courseId="info-literacy" onExit={vi.fn()} onFinish={vi.fn()} />);
    expect(await screen.findByText("第 1 步的正文。")).toBeInTheDocument();
  });

  it("pages backward via 上一步, saving the resume position", async () => {
    render(<CoursePlayer courseId="info-literacy" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByText("第 0 步的正文。");
    fireEvent.click(screen.getByLabelText("下一步"));
    await screen.findByText("第 1 步的正文。");
    (api.saveCourseProgress as any).mockClear();
    fireEvent.click(screen.getByLabelText("上一步"));
    expect(await screen.findByText("第 0 步的正文。")).toBeInTheDocument();
    // 上一步 now persists the resume position (and flushes active time).
    await waitFor(() => expect(api.saveCourseProgress).toHaveBeenCalledWith("info-literacy", expect.objectContaining({ current_ordinal: 0 })));
  });

  it("calls onFinish from the last step's 完成课程 control once the step is done", async () => {
    const onFinish = vi.fn();
    render(<CoursePlayer courseId="info-literacy" onExit={vi.fn()} onFinish={onFinish} />);
    await screen.findByText("第 0 步的正文。");
    fireEvent.click(screen.getByLabelText("下一步"));
    await screen.findByText("第 1 步的正文。");
    expect(screen.queryByLabelText("下一步")).not.toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("完成课程"));
    expect(onFinish).toHaveBeenCalled();
  });

  it("gates 下一步 until the step is fully revealed AND its quiz answered", async () => {
    const gated: CoursePlayerPayload = {
      ...payload,
      renderCache: {
        ...payload.renderCache,
        steps: [
          {
            stepId: "s0",
            content: {
              title: "第 0 步",
              subtitle: "",
              segments: [
                { kind: "teaching", flow_block_id: "", text: "先读这段。", asset_ids: [], items: [] },
                { kind: "teaching", flow_block_id: "q1", text: "现在回答问题。", asset_ids: [], items: [] },
              ],
              interactions: [
                { id: "q1", type: "single_choice", prompt: "选哪个？", options: [{ id: "a", text: "选项 A" }, { id: "b", text: "选项 B" }], correct_answer: ["b"], explanation: "", remediation_questions: [] },
              ],
              board: [],
            },
          },
          payload.renderCache.steps[1]!,
        ],
      },
    };
    (api.getCourse as any).mockResolvedValue(gated);
    render(<CoursePlayer courseId="info-literacy" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByText("先读这段。");

    const pane = screen.getByTestId("segment-timeline");
    // Not fully revealed yet → 下一步 disabled.
    expect(screen.getByLabelText("下一步")).toBeDisabled();

    // Reveal the second segment, then the quiz.
    fireEvent.click(pane);
    await screen.findByText("现在回答问题。");
    fireEvent.click(pane);
    await screen.findByText("选哪个？");

    // Everything revealed but the quiz is unanswered → still disabled.
    expect(screen.getByLabelText("下一步")).toBeDisabled();

    // Answer (any option) and submit → gate clears.
    fireEvent.click(screen.getByText("选项 A"));
    fireEvent.click(screen.getByRole("button", { name: "提交" }));
    await waitFor(() => expect(screen.getByLabelText("下一步")).not.toBeDisabled());
    expect(api.answerCourseQuiz).toHaveBeenCalled();
  });

  it("sends a question through the AI bar and streams the reply", async () => {
    (api.courseAsk as any).mockImplementation(() => gen([{ type: "reply", body: "先看看这条信息的来源。" }]));
    render(<CoursePlayer courseId="info-literacy" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByText("第 0 步的正文。");

    await userEvent.type(screen.getByPlaceholderText("输入你的问题……"), "这段话可信吗？");
    await userEvent.click(screen.getByLabelText("发送"));

    expect(await screen.findByText("先看看这条信息的来源。")).toBeInTheDocument();
    expect(api.courseAsk).toHaveBeenCalledWith("info-literacy", "这段话可信吗？", 0);
  });

  it("surfaces a courseAsk error frame as the assistant's reply", async () => {
    (api.courseAsk as any).mockImplementation(() => gen([{ type: "error", message: "出错了，请重试" }]));
    render(<CoursePlayer courseId="info-literacy" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByText("第 0 步的正文。");

    await userEvent.type(screen.getByPlaceholderText("输入你的问题……"), "这段话可信吗？");
    await userEvent.click(screen.getByLabelText("发送"));

    expect(await screen.findByText("出错了，请重试")).toBeInTheDocument();
  });
});
