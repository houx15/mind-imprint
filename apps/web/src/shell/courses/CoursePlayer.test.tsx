import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Course, CourseSession } from "@mind-imprint/contracts";

vi.mock("../../api", async (orig) => {
  const real = await orig<typeof import("../../api")>();
  return {
    ...real,
    api: {
      ...real.api,
      getCourse: vi.fn(),
      getCourseProgress: vi.fn(),
      saveCourseProgress: vi.fn(),
      renderCourseStep: vi.fn(),
      startCourseSession: vi.fn(),
      getCourseSession: vi.fn(),
      courseAsk: vi.fn(),
      courseAdvance: vi.fn(),
      submitCourseCard: vi.fn(),
      skipCourseCard: vi.fn(),
    },
  };
});

import { api } from "../../api";
import { CoursePlayer } from "./CoursePlayer";

const course: Course = {
  id: "co1", branch: "批判性思维", title: "一条网络信息，该不该信", blurb: "…", tasks_count: 3, tools_count: 4, time_label: "约 40 分钟", step_count: 2,
  steps: [
    { id: "s0", course_id: "co1", ordinal: 0, kind: "teaching", purpose: "", assets: [], challenge_type: null, authored_content: {} },
    { id: "s1", course_id: "co1", ordinal: 1, kind: "teaching", purpose: "", assets: [], challenge_type: null, authored_content: {} },
  ],
};

// The real seeded skill's "demonstrate" phase covers ordinals [0, 1] — this
// fixture's two steps sit entirely inside it, so a next-click from ordinal 0
// to 1 never crosses the phase boundary; a next-click from ordinal 1 always
// does (matches packages/contracts/skills/info-literacy-course.json).
const demonstrateSession: CourseSession = {
  id: "sess1", courseId: "co1", phase: "demonstrate", phaseTitle: "演示", status: "active", messages: [], openCards: [], collectedCards: [],
};

// Every real courseAsk/courseAdvance stream ends with a `done` frame
// unconditionally — the server emits it at the end of EVERY turn, including
// the error path (apps/api/internal/api/course_session.go's runCourseTurn
// calls em.Done() unconditionally). A mock stream that omits it encodes a
// shape the backend cannot produce, so `done` is always appended here to
// keep every mock faithful to the real client's contract
// (apps/web/src/api/courseSession.ts's runCourseTurn always yields it last).
async function* gen(events: unknown[]) {
  for (const e of events) yield e;
  yield { type: "done" };
}

describe("CoursePlayer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.getCourse as any).mockResolvedValue(course);
    (api.getCourseProgress as any).mockResolvedValue({ course_id: "co1", current_ordinal: 0, completed_ordinals: [], updated_at: "" });
    (api.saveCourseProgress as any).mockResolvedValue({ course_id: "co1", current_ordinal: 1, completed_ordinals: [0], updated_at: "" });
    (api.renderCourseStep as any).mockImplementation(async (_c: string, ord: number) => ({
      ordinal: ord, kind: "teaching", template: "teaching", source: "generated",
      content: { title: `第 ${ord} 步`, subtitle: "导语", body: ["正文。"], foreground_asset_id: null },
    }));
    (api.startCourseSession as any).mockResolvedValue(demonstrateSession);
    (api.getCourseSession as any).mockResolvedValue(demonstrateSession);
    (api.courseAsk as any).mockImplementation(() => gen([]));
    (api.courseAdvance as any).mockImplementation(() => gen([]));
    (api.submitCourseCard as any).mockResolvedValue(undefined);
    (api.skipCourseCard as any).mockResolvedValue(undefined);
  });

  it("renders the first step and advances to the next on 下一步", async () => {
    render(<CoursePlayer courseId="co1" onExit={vi.fn()} onFinish={vi.fn()} />);
    expect(await screen.findByText("第 0 步")).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("下一步"));
    expect(await screen.findByText("第 1 步")).toBeInTheDocument();
    // Whole-branch C1+C3 regression, sharpened by Minor 4: the write payload
    // carries ONLY current_ordinal — completed_ordinals was dropped from the
    // write-side type entirely (courses.ts/index.ts), since the server (not
    // this call) is the sole writer of the steps_viewed floor's real input
    // (course_render.go's RecordCourseStepViewed). Asserting the exact
    // payload here (not just that the call fired) is what would have caught
    // a future reintroduction of a client-writable completed_ordinals.
    await waitFor(() => expect(api.saveCourseProgress).toHaveBeenCalledWith("co1", { current_ordinal: 1 }));
  });

  it("exits via the back control", async () => {
    const onExit = vi.fn();
    render(<CoursePlayer courseId="co1" onExit={onExit} onFinish={vi.fn()} />);
    await screen.findByText("第 0 步");
    fireEvent.click(screen.getByText("课程"));
    expect(onExit).toHaveBeenCalled();
  });

  it("resumes at the saved ordinal without rendering step 0 first", async () => {
    (api.getCourseProgress as any).mockResolvedValue({ course_id: "co1", current_ordinal: 1, completed_ordinals: [0], updated_at: "" });
    render(<CoursePlayer courseId="co1" onExit={vi.fn()} onFinish={vi.fn()} />);
    expect(await screen.findByText("第 1 步")).toBeInTheDocument();
    expect(api.renderCourseStep).toHaveBeenCalledWith("co1", 1);
    expect(api.renderCourseStep).not.toHaveBeenCalledWith("co1", 0);
  });

  it("renders the phase title as the stage label", async () => {
    render(<CoursePlayer courseId="co1" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByText("第 0 步");
    // header pill + the small label above the page title — both read the
    // session's phaseTitle ("演示"), never a hardcoded phase id.
    expect(screen.getAllByText("演示").length).toBe(2);
  });

  it("pages inside a phase without calling courseAdvance", async () => {
    render(<CoursePlayer courseId="co1" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByText("第 0 步");
    fireEvent.click(screen.getByLabelText("下一步")); // 0 -> 1, both inside "demonstrate"
    await screen.findByText("第 1 步");
    expect(api.courseAdvance).not.toHaveBeenCalled();
  });

  it("asks the coach when next would cross a phase boundary", async () => {
    (api.courseAdvance as any).mockImplementation(() => gen([{ type: "phase", to: "guided" }]));
    render(<CoursePlayer courseId="co1" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByText("第 0 步");
    fireEvent.click(screen.getByLabelText("下一步")); // 0 -> 1, local
    await screen.findByText("第 1 步");
    fireEvent.click(screen.getByLabelText("下一步")); // 1 -> boundary, asks the coach
    await waitFor(() => expect(api.courseAdvance).toHaveBeenCalledTimes(1));
  });

  it("auto-expands the panel when the coach refuses to advance", async () => {
    (api.courseAdvance as any).mockImplementation(() => gen([{ type: "reply", body: "先把第二步也看完，再往下走。" }]));
    render(<CoursePlayer courseId="co1" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByText("第 0 步");
    // collapse the panel first so the auto-expand-on-refusal behavior is
    // actually observable (the panel starts expanded by default).
    fireEvent.click(screen.getByLabelText("收起问印记"));
    expect(screen.queryByPlaceholderText("输入你的问题……")).not.toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("下一步")); // 0 -> 1, local (panel stays collapsed)
    await screen.findByText("第 1 步");
    fireEvent.click(screen.getByLabelText("下一步")); // 1 -> boundary, refused

    expect(await screen.findByText("先把第二步也看完，再往下走。")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("输入你的问题……")).toBeInTheDocument(); // re-expanded
    expect(screen.getByText("第 1 步")).toBeInTheDocument(); // the page has not moved
  });

  it("moves the phase when the coach advances", async () => {
    (api.courseAdvance as any).mockImplementation(() => gen([{ type: "phase", to: "guided" }]));
    (api.getCourseSession as any).mockResolvedValue({ id: "sess1", courseId: "co1", phase: "guided", phaseTitle: "引导", status: "active", messages: [] });
    render(<CoursePlayer courseId="co1" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByText("第 0 步");
    fireEvent.click(screen.getByLabelText("下一步")); // 0 -> 1, local
    await screen.findByText("第 1 步");
    fireEvent.click(screen.getByLabelText("下一步")); // 1 -> boundary, coach advances

    await waitFor(() => expect(screen.getAllByText("引导").length).toBeGreaterThan(0));
  });

  it("finishes the course when a courseAdvance stream settles into a finished session", async () => {
    // The terminal never arrives as a frame — the coach's reply is a plain
    // `reply` frame (the phase does not move: reflect has no successor). The
    // player learns completion by refetching the session after the stream
    // settles and finding status: "finished" (Critical-3 — the backend mints
    // the terminal; the client learns it from the session, not a frame).
    (api.courseAdvance as any).mockImplementation(() => gen([{ type: "reply", body: "这节课到这里就走完了。" }]));
    (api.getCourseSession as any).mockResolvedValue({ id: "sess1", courseId: "co1", phase: "reflect", phaseTitle: "回看", status: "finished", messages: [] });
    const onFinish = vi.fn();
    render(<CoursePlayer courseId="co1" onExit={vi.fn()} onFinish={onFinish} />);
    await screen.findByText("第 0 步");
    fireEvent.click(screen.getByLabelText("下一步")); // 0 -> 1, local
    await screen.findByText("第 1 步");
    fireEvent.click(screen.getByLabelText("下一步")); // 1 -> boundary, coach settles the terminal

    await waitFor(() => expect(onFinish).toHaveBeenCalled());
  });

  it("degrades to local paging with no courseAdvance calls when the session fails to start", async () => {
    // Important-5: a failed startCourseSession must not brick the page — the
    // content layer (course_step pages) survives the runtime layer being
    // down, exactly like the pre-Slice-12 player, and handleNext must never
    // fire courseAdvance against a session that does not exist.
    (api.startCourseSession as any).mockRejectedValue(new Error("session runtime unavailable"));
    render(<CoursePlayer courseId="co1" onExit={vi.fn()} onFinish={vi.fn()} />);
    expect(await screen.findByText("第 0 步")).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("下一步"));
    expect(await screen.findByText("第 1 步")).toBeInTheDocument();

    expect(api.courseAdvance).not.toHaveBeenCalled();
  });

  it("requires 接受 before the card sheet mounts", async () => {
    (api.courseAsk as any).mockImplementation(() => gen([{ type: "card", cardInstanceId: "ci1", cardId: "craap", materialId: "m1" }]));
    render(<CoursePlayer courseId="co1" onExit={vi.fn()} onFinish={vi.fn()} />);
    await screen.findByText("第 0 步");

    await userEvent.type(screen.getByPlaceholderText("输入你的问题……"), "这张卡要我做什么？");
    await userEvent.click(screen.getByLabelText("发送"));

    expect(await screen.findByText("接受")).toBeInTheDocument();
    expect(screen.queryByText("跳过这张卡")).not.toBeInTheDocument();
    // Minor 3 (whole-branch): the live `card` frame carries no reply text —
    // the assistant turn that carries the offer must not also render an
    // empty bubble above the card preview. (The student's own typed message
    // legitimately renders a bubble, so this checks every rendered bubble
    // has text, rather than asserting none exist at all.)
    for (const bubble of screen.queryAllByTestId("ask-bubble")) {
      expect(bubble.textContent).not.toBe("");
    }

    await userEvent.click(screen.getByText("接受"));
    expect(await screen.findByText("跳过这张卡")).toBeInTheDocument();
  });

  // Whole-branch Important-2: a step-less phase (guided/reflect) has no
  // course_step render to page through, so 上一步 must not render there even
  // when ordinal > 0 (the resumed position from a prior step-ful phase) —
  // before the fix it rendered and visibly did nothing but change the
  // counter.
  it("hides 上一步 in a step-less phase even when ordinal > 0", async () => {
    (api.getCourseProgress as any).mockResolvedValue({ course_id: "co1", current_ordinal: 1, completed_ordinals: [0], updated_at: "" });
    const guidedSession: CourseSession = {
      id: "sess1", courseId: "co1", phase: "guided", phaseTitle: "引导", status: "active", messages: [], openCards: [], collectedCards: [],
    };
    (api.startCourseSession as any).mockResolvedValue(guidedSession);
    (api.getCourseSession as any).mockResolvedValue(guidedSession);

    render(<CoursePlayer courseId="co1" onExit={vi.fn()} onFinish={vi.fn()} />);
    // guided's authored page block renders (steps: [] in the skill config).
    expect(await screen.findByText("现在，对这条说法做一次溯源")).toBeInTheDocument();

    expect(screen.queryByLabelText("上一步")).not.toBeInTheDocument();
    expect(screen.getByLabelText("下一步")).toBeInTheDocument();
  });

  // Whole-branch Critical-2: a card offer that already exists in the DB
  // (status proposed/active, carried on the session as `openCards`) must be
  // restored into the ask panel on load — this is exactly what a page reload
  // during `guided` needs, since before the fix the offer lived only in
  // React state and a reload erased it, dead-ending card_dispositioned
  // forever. No turn is driven to produce it — it comes purely from session
  // load.
  it("rehydrates an open card offer from the session on load", async () => {
    const guidedSessionWithOffer: CourseSession = {
      id: "sess1", courseId: "co1", phase: "guided", phaseTitle: "引导", status: "active", messages: [],
      openCards: [{ cardInstanceId: "ci1", cardId: "craap", materialId: "m1" }], collectedCards: [],
    };
    (api.startCourseSession as any).mockResolvedValue(guidedSessionWithOffer);
    (api.getCourseSession as any).mockResolvedValue(guidedSessionWithOffer);

    render(<CoursePlayer courseId="co1" onExit={vi.fn()} onFinish={vi.fn()} />);
    expect(await screen.findByText("接受")).toBeInTheDocument();
    expect(api.courseAsk).not.toHaveBeenCalled();
    expect(api.courseAdvance).not.toHaveBeenCalled();
    // Minor 3 (whole-branch): the rehydrated offer carries text: "" — it
    // must not render an empty bubble above the card preview.
    expect(screen.queryByTestId("ask-bubble")).not.toBeInTheDocument();

    // And it is still a confirm-to-open offer (铁律 2): accepting mounts the
    // card sheet, exactly like a freshly-surfaced offer would.
    await userEvent.click(screen.getByText("接受"));
    expect(await screen.findByText("跳过这张卡")).toBeInTheDocument();
  });
});
