import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { CourseSession, SliceSessionState } from "@mind-imprint/course-contract";
import {
  InMemorySessionAdapter,
  type CourseRuntimeAdapters,
  type RuntimeEventBus,
  type RuntimeSceneGenerator,
  type SceneGenerationInput,
  type SessionAdapter,
} from "@mind-imprint/course-runtime";
import { CoursePlayer } from "../src/course/CoursePlayer";
import { AudioEngineProvider } from "../src/narration/audioEngine";
import { FakeAudioEngine } from "./support/fakeAudioEngine";
import { staticCourseDocument } from "./support/staticCourse";

function makeIdFactory(prefix: string) {
  let n = 0;
  return () => `${prefix}-${++n}`;
}
const clock = () => "2026-08-16T00:00:00.000Z";

/** Fallback generator: echoes the input fallback text, flagged fallbackUsed. */
const fallbackGenerator: RuntimeSceneGenerator = {
  generate: async (input: SceneGenerationInput) => ({
    text: input.fallback.text,
    audioUrl: input.fallback.audioUrl,
    generatedAt: clock(),
    usedSignalTypes: [],
    fallbackUsed: true,
  }),
};

function buildAdapters() {
  const sessionAdapter = new InMemorySessionAdapter({ idFactory: makeIdFactory("session"), clock });
  const adapters: CourseRuntimeAdapters = {
    assetResolver: { resolve: (p) => `/resolved/${p}` },
    sessionAdapter,
    openingGenerator: fallbackGenerator,
    closingGenerator: fallbackGenerator,
  };
  return { sessionAdapter, adapters };
}

/**
 * A SessionAdapter whose `create()` mimics the production API adapter's
 * get-or-create semantics (apps/web's apiSessionAdapter over the server's
 * get-or-create POST): it does NOT mint a fresh session — it always returns
 * (and mutates) the one pre-seeded session, regardless of the input. This is
 * what a real host that never passes a `sessionId` prop actually gets back,
 * so tests here exercise the same "no sessionId → create() must still drive
 * resume" path production hits.
 */
function buildSeededAdapters(seed: CourseSession) {
  let session: CourseSession = structuredClone(seed);
  const setStatusCalls: Array<CourseSession["status"]> = [];

  const sessionAdapter: SessionAdapter = {
    async load(sessionId) {
      return session.id === sessionId ? structuredClone(session) : null;
    },
    async create() {
      // Get-or-create: the seeded session already exists server-side.
      return structuredClone(session);
    },
    async appendEvent(_sessionId, event) {
      session.events.push(structuredClone(event));
    },
    async saveSliceState(_sessionId, sliceId, state) {
      session.sliceStates[sliceId] = structuredClone(state);
    },
    async saveScene(_sessionId, which, result) {
      session[which] = structuredClone(result);
    },
    async setStatus(_sessionId, status) {
      setStatusCalls.push(status);
      session.status = status;
    },
    async setCurrent(_sessionId, current) {
      session.current = current ? structuredClone(current) : undefined;
    },
  };

  const adapters: CourseRuntimeAdapters = {
    assetResolver: { resolve: (p) => `/resolved/${p}` },
    sessionAdapter,
    openingGenerator: fallbackGenerator,
    closingGenerator: fallbackGenerator,
  };
  return { adapters, setStatusCalls, getSession: () => session };
}

describe("CoursePlayer end-to-end", () => {
  it("plays Opening → Slices → Closing driven by the workflow; Closing is visible and the session stays 'closing' until the learner dismisses it, only then firing onComplete (P1-03)", async () => {
    const { sessionAdapter, adapters } = buildAdapters();
    const engine = new FakeAudioEngine();
    let bus: RuntimeEventBus | null = null;
    const onComplete = vi.fn();

    render(
      <AudioEngineProvider value={engine}>
        <CoursePlayer
          document={staticCourseDocument}
          adapters={adapters}
          studentId="student-1"
          idFactory={makeIdFactory("ev")}
          clock={clock}
          onBusReady={(b) => {
            bus = b;
          }}
          onComplete={onComplete}
        />
      </AudioEngineProvider>,
    );

    // Opening renders with the fixed start action + the fallback greeting.
    await screen.findByText("一起开始吧");
    expect(screen.getByText(/欢迎来到这门演示课程/)).toBeInTheDocument();

    // Start → slice one mounts and plays its narration.
    await userEvent.click(screen.getByRole("button", { name: "一起开始吧" }));
    expect(engine.calls.some((c) => c.url === "/resolved/audio/s1.mp3")).toBe(true);

    // slice one: narration.ended → reveal, then student.continue → navigate.
    act(() => engine.fireEnded());
    expect(document.querySelector('[data-block-id="s1-reveal-text"]')).not.toHaveAttribute("hidden");
    act(() => bus!.bindSlice("part-one", "slice-one")("s1-continue", "student.continue"));

    // slice two mounted and playing its narration.
    expect(engine.calls.some((c) => c.url === "/resolved/audio/s2.mp3")).toBe(true);

    // slice two: narration.ended → navigate past the last slice → Closing.
    act(() => engine.fireEnded());

    // Closing renders the prepared summary + takeaways — the learner ACTUALLY
    // sees it (this is the P1-03 fix: it must render before status flips to
    // "completed", not after).
    await screen.findByText("你走完了两个片段，理解了演示流程。");
    expect(screen.getByText("片段按顺序推进")).toBeInTheDocument();
    expect(screen.getByText("讲解结束触发揭示")).toBeInTheDocument();

    // Still "closing", not "completed" — and onComplete has NOT fired — while
    // the Closing scene is up and the learner hasn't dismissed it yet.
    const midSession = await sessionAdapter.load("session-1");
    expect(midSession!.status).toBe("closing");
    expect(midSession!.closing?.fallbackUsed).toBe(true);
    expect(onComplete).not.toHaveBeenCalled();

    // The learner dismisses via the explicit completion control — only THEN
    // does the session become "completed" and onComplete fire.
    await userEvent.click(screen.getByRole("button", { name: "完成课程" }));
    expect(onComplete).toHaveBeenCalledTimes(1);

    const finalSession = await sessionAdapter.load("session-1");
    expect(finalSession!.status).toBe("completed");
  });

  it("resumes a 'closing' (not-yet-dismissed) session returned by get-or-create: renders Closing directly without regenerating, and dismissing it still completes + fires onComplete", async () => {
    const seed: CourseSession = {
      id: "server-session-closing",
      courseId: "static-demo-course",
      courseSchemaVersion: "2.0",
      studentId: "student-1",
      status: "closing",
      opening: { text: "欢迎回来", generatedAt: clock(), usedSignalTypes: [], fallbackUsed: true },
      closing: { text: "已保存的收尾", generatedAt: clock(), usedSignalTypes: [], fallbackUsed: true },
      sliceStates: {},
      events: [],
    };
    const { adapters, setStatusCalls } = buildSeededAdapters(seed);
    const engine = new FakeAudioEngine();
    const onComplete = vi.fn();

    render(
      <AudioEngineProvider value={engine}>
        <CoursePlayer
          document={staticCourseDocument}
          adapters={adapters}
          studentId="student-1"
          idFactory={makeIdFactory("ev")}
          clock={clock}
          onComplete={onComplete}
        />
      </AudioEngineProvider>,
    );

    // Restored straight to the saved Closing narration — never regenerated,
    // never touched status on the way in.
    await screen.findByText("已保存的收尾");
    expect(setStatusCalls).toEqual([]);
    expect(onComplete).not.toHaveBeenCalled();

    await userEvent.click(screen.getByRole("button", { name: "完成课程" }));
    expect(onComplete).toHaveBeenCalledTimes(1);
    expect(setStatusCalls).toEqual(["completed"]);
  });

  it("resumes an existing mid-Slice session: lands on the same Slice + workflow step + block state, not Opening/index 0", async () => {
    const { sessionAdapter, adapters } = buildAdapters();
    const engine = new FakeAudioEngine();

    // Seed a prior session already parked mid-slice-one, at the "reveal" step
    // (reached only after narration.ended — never the initial "intro" step).
    const priorSession = await sessionAdapter.create({ courseId: "static-demo-course", studentId: "student-1" });
    await sessionAdapter.saveScene(priorSession.id, "opening", {
      text: "欢迎回来",
      generatedAt: clock(),
      usedSignalTypes: [],
      fallbackUsed: true,
    });
    await sessionAdapter.setStatus(priorSession.id, "in-progress");
    const revealState: SliceSessionState = {
      status: "in-progress",
      currentWorkflowStepId: "reveal",
      startedAt: clock(),
      elapsedSeconds: 12,
      blockStates: {
        "s1-intro-text": { visible: true, enabled: true, completed: false },
        "s1-reveal-text": { visible: true, enabled: true, completed: false },
        "s1-continue": { visible: true, enabled: true, completed: false },
      },
    };
    await sessionAdapter.saveSliceState(priorSession.id, "slice-one", revealState);
    await sessionAdapter.setCurrent(priorSession.id, { partId: "part-one", sliceId: "slice-one", workflowStepId: "reveal" });

    render(
      <AudioEngineProvider value={engine}>
        <CoursePlayer
          document={staticCourseDocument}
          adapters={adapters}
          studentId="student-1"
          sessionId={priorSession.id}
          idFactory={makeIdFactory("ev")}
          clock={clock}
        />
      </AudioEngineProvider>,
    );

    // Landed directly on slice-one's "reveal" step: the reveal block is already
    // visible without ever firing narration.ended — a fresh mount would start at
    // "intro" (reveal hidden, only after narration.ended does it show).
    await waitFor(() => {
      expect(document.querySelector('[data-block-id="s1-reveal-text"]')).not.toHaveAttribute("hidden");
    });
    // Never showed the Opening scene or slice-two — restored straight to slice-one.
    expect(screen.queryByText("一起开始吧")).toBeNull();
    expect(document.querySelector('[data-block-id="s2-text"]')).toBeNull();
    // "reveal" enterActions don't playNarration — the intro narration never replayed.
    expect(engine.calls.some((c) => c.op === "play" && c.url === "/resolved/audio/s1.mp3")).toBe(false);
  });

  it("resumes an in-progress session returned by get-or-create even with NO client sessionId prop (production host path): restores mid-Slice, never shows Opening, never resets status to 'opening'", async () => {
    const seed: CourseSession = {
      id: "server-session-1",
      courseId: "static-demo-course",
      courseSchemaVersion: "2.0",
      studentId: "student-1",
      status: "in-progress",
      current: { partId: "part-one", sliceId: "slice-one", workflowStepId: "reveal" },
      opening: { text: "欢迎回来", generatedAt: clock(), usedSignalTypes: [], fallbackUsed: true },
      sliceStates: {
        "slice-one": {
          status: "in-progress",
          currentWorkflowStepId: "reveal",
          startedAt: clock(),
          elapsedSeconds: 12,
          blockStates: {
            "s1-intro-text": { visible: true, enabled: true, completed: false },
            "s1-reveal-text": { visible: true, enabled: true, completed: false },
            "s1-continue": { visible: true, enabled: true, completed: false },
          },
        },
      },
      events: [],
    };
    const { adapters, setStatusCalls } = buildSeededAdapters(seed);
    const engine = new FakeAudioEngine();

    render(
      <AudioEngineProvider value={engine}>
        <CoursePlayer
          document={staticCourseDocument}
          adapters={adapters}
          studentId="student-1"
          // No sessionId prop — this is exactly what RuntimeCoursePlayer does
          // in production; resume must still key off the get-or-create result.
          idFactory={makeIdFactory("ev")}
          clock={clock}
        />
      </AudioEngineProvider>,
    );

    // Landed directly on slice-one's "reveal" step, matching the seeded state.
    await waitFor(() => {
      expect(document.querySelector('[data-block-id="s1-reveal-text"]')).not.toHaveAttribute("hidden");
    });
    // Never showed the fresh Opening scene or reset to slice index 0's intro.
    expect(screen.queryByText("一起开始吧")).toBeNull();
    expect(document.querySelector('[data-block-id="s2-text"]')).toBeNull();
    // The server's in-progress status must never be clobbered back to "opening".
    expect(setStatusCalls).not.toContain("opening");
  });

  it("resumes a completed session returned by get-or-create with NO client sessionId prop: renders Closing/summary directly, never resets", async () => {
    const seed: CourseSession = {
      id: "server-session-2",
      courseId: "static-demo-course",
      courseSchemaVersion: "2.0",
      studentId: "student-1",
      status: "completed",
      opening: { text: "欢迎回来", generatedAt: clock(), usedSignalTypes: [], fallbackUsed: true },
      closing: { text: "已保存的收尾", generatedAt: clock(), usedSignalTypes: [], fallbackUsed: true },
      sliceStates: {},
      events: [],
    };
    const { adapters, setStatusCalls } = buildSeededAdapters(seed);
    const engine = new FakeAudioEngine();

    render(
      <AudioEngineProvider value={engine}>
        <CoursePlayer
          document={staticCourseDocument}
          adapters={adapters}
          studentId="student-1"
          idFactory={makeIdFactory("ev")}
          clock={clock}
        />
      </AudioEngineProvider>,
    );

    // Restored straight to the saved Closing narration + the course's summary.
    await screen.findByText("已保存的收尾");
    expect(screen.getByText("你走完了两个片段，理解了演示流程。")).toBeInTheDocument();
    // Never showed Opening or any Slice.
    expect(screen.queryByText("一起开始吧")).toBeNull();
    expect(document.querySelector(".course-slice")).toBeNull();
    // A completed session's status must never be touched, let alone reset.
    expect(setStatusCalls).toEqual([]);
  });

  it("renders the error surface for a structurally-invalid document and never mounts a slice", () => {
    const { adapters } = buildAdapters();
    render(
      <CoursePlayer
        document={{ schemaVersion: "2.0", course: { id: "bad" } }}
        adapters={adapters}
        studentId="student-1"
        idFactory={makeIdFactory("ev")}
        clock={clock}
      />,
    );
    expect(screen.getByRole("alert")).toHaveAttribute("data-course-error", "true");
    expect(document.querySelector(".course-slice")).toBeNull();
    expect(screen.queryByText("一起开始吧")).toBeNull();
  });
});
