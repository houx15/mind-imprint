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
      resolveUrl: vi.fn(),
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
  audioKeys: {},
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
    window.localStorage.clear();
    (api.getCourse as any).mockResolvedValue(payload);
    (api.getCourseProgress as any).mockRejectedValue(new Error("no progress yet"));
    (api.saveCourseProgress as any).mockResolvedValue({ course_slug: "info-literacy", current_ordinal: 1, completed_ordinals: [0], started_at: null, completed_at: null, updated_at: "" });
    (api.answerCourseQuiz as any).mockResolvedValue(undefined);
    (api.courseAsk as any).mockImplementation(() => gen([]));
    (api.resolveUrl as any).mockResolvedValue("https://cdn.example.com/audio.mp3");
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

describe("CoursePlayer narration playback", () => {
  // jsdom implements neither play/pause/load nor the paused/ended state
  // transitions a real <audio> element would drive — spy on the prototype so
  // the controller's calls are observable without crashing (real HTMLMedia
  // methods throw "not implemented" in jsdom).
  let lastPlaySrc: string | undefined;
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    lastPlaySrc = undefined;
    vi.spyOn(HTMLMediaElement.prototype, "play").mockImplementation(function (this: HTMLMediaElement) {
      lastPlaySrc = this.src;
      return Promise.resolve();
    });
    vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {});
    vi.spyOn(HTMLMediaElement.prototype, "load").mockImplementation(() => {});
    (api.getCourseProgress as any).mockRejectedValue(new Error("no progress yet"));
    (api.saveCourseProgress as any).mockResolvedValue({ course_slug: "info-literacy", current_ordinal: 1, completed_ordinals: [0], started_at: null, completed_at: null, updated_at: "" });
    (api.answerCourseQuiz as any).mockResolvedValue(undefined);
    (api.courseAsk as any).mockImplementation(() => gen([]));
  });

  const payloadWithAudio: CoursePlayerPayload = {
    ...payload,
    audioKeys: {
      "s0#0": "courses/audio/x/s0_0_abcd1234.mp3",
      "s1#0": "courses/audio/x/s1_0_efgh5678.mp3",
    },
  };

  it("resolves and plays the current piece's narration on the first gesture (▶)", async () => {
    (api.getCourse as any).mockResolvedValue(payloadWithAudio);
    (api.resolveUrl as any).mockResolvedValue("https://cdn.example.com/piece0.mp3");
    render(<CoursePlayer courseId="info-literacy" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByText("第 0 步的正文。");

    fireEvent.click(screen.getByLabelText("播放"));

    await waitFor(() => expect(api.resolveUrl).toHaveBeenCalledWith("courses/audio/x/s0_0_abcd1234.mp3"));
    await waitFor(() => expect(HTMLMediaElement.prototype.play).toHaveBeenCalled());
    expect(lastPlaySrc).toContain("https://cdn.example.com/piece0.mp3");
  });

  it("does not resolve or play before any gesture (mount's initial reveal is not a gesture)", async () => {
    (api.getCourse as any).mockResolvedValue(payloadWithAudio);
    render(<CoursePlayer courseId="info-literacy" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByText("第 0 步的正文。");

    // Give any stray microtask a chance to run, then confirm nothing fired.
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(api.resolveUrl).not.toHaveBeenCalled();
    expect(HTMLMediaElement.prototype.play).not.toHaveBeenCalled();
  });

  it("mute stops further resolve/play and persists to localStorage", async () => {
    (api.getCourse as any).mockResolvedValue(payloadWithAudio);
    (api.resolveUrl as any).mockResolvedValue("https://cdn.example.com/piece0.mp3");
    render(<CoursePlayer courseId="info-literacy" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByText("第 0 步的正文。");

    fireEvent.click(screen.getByLabelText("播放"));
    await waitFor(() => expect(api.resolveUrl).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByLabelText("静音"));
    expect(window.localStorage.getItem("course-audio-muted")).toBe("1");
    expect(HTMLMediaElement.prototype.pause).toHaveBeenCalled();

    // Pressing ▶ again while muted must not resolve/play again.
    fireEvent.click(screen.getByLabelText("播放"));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(api.resolveUrl).toHaveBeenCalledTimes(1);
  });

  it("discards a stale resolveUrl response superseded by a later piece (race protection)", async () => {
    const twoSegPayload: CoursePlayerPayload = {
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
                { kind: "teaching", flow_block_id: "", text: "第一段。", asset_ids: [], items: [] },
                { kind: "teaching", flow_block_id: "", text: "第二段。", asset_ids: [], items: [] },
              ],
              interactions: [],
              board: [],
            },
          },
          payload.renderCache.steps[1]!,
        ],
      },
      audioKeys: {
        "s0#0": "courses/audio/x/s0_0_first.mp3",
        "s0#1": "courses/audio/x/s0_1_second.mp3",
      },
    };
    (api.getCourse as any).mockResolvedValue(twoSegPayload);

    // Keyed (not positional) so the piece-0 prefetch that fires unprompted at
    // mount (background-priming piece 1, no gesture needed) can't shift which
    // call gets which response — only piece 0's resolve is held pending.
    let resolveFirst!: (url: string) => void;
    const firstPending = new Promise<string>((resolve) => {
      resolveFirst = resolve;
    });
    (api.resolveUrl as any).mockImplementation((key: string) => {
      if (key === "courses/audio/x/s0_0_first.mp3") return firstPending;
      if (key === "courses/audio/x/s0_1_second.mp3") return Promise.resolve("https://cdn.example.com/second.mp3");
      return Promise.resolve("https://cdn.example.com/unused.mp3");
    });

    render(<CoursePlayer courseId="info-literacy" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByText("第一段。");

    // First gesture: play piece 0 — its resolve stays pending.
    fireEvent.click(screen.getByLabelText("播放"));
    await waitFor(() => expect(api.resolveUrl).toHaveBeenCalledWith("courses/audio/x/s0_0_first.mp3"));
    expect(HTMLMediaElement.prototype.play).not.toHaveBeenCalled();

    // Reveal piece 1 before the first resolve returns — this must supersede it.
    fireEvent.click(screen.getByTestId("segment-timeline"));
    await screen.findByText("第二段。");
    await waitFor(() => expect(lastPlaySrc).toContain("second.mp3"));
    expect(HTMLMediaElement.prototype.play).toHaveBeenCalledTimes(1);

    // Now the stale first resolve finally returns — it must be discarded, not played.
    resolveFirst("https://cdn.example.com/first.mp3");
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(lastPlaySrc).toContain("second.mp3");
    expect(HTMLMediaElement.prototype.play).toHaveBeenCalledTimes(1);
  });
});
