import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  InMemorySessionAdapter,
  type CourseRuntimeAdapters,
  type RuntimeEventBus,
  type RuntimeSceneGenerator,
  type SceneGenerationInput,
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

describe("CoursePlayer end-to-end", () => {
  it("plays Opening → Slices → Closing driven by the workflow, persisting the session", async () => {
    const { sessionAdapter, adapters } = buildAdapters();
    const engine = new FakeAudioEngine();
    let bus: RuntimeEventBus | null = null;

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

    // Closing renders the prepared summary + takeaways.
    await screen.findByText("你走完了两个片段，理解了演示流程。");
    expect(screen.getByText("片段按顺序推进")).toBeInTheDocument();
    expect(screen.getByText("讲解结束触发揭示")).toBeInTheDocument();

    // Session ends completed.
    const sessions = await Promise.all(
      // there is exactly one session; find it by loading the only created id.
      [await sessionAdapter.load("session-1")],
    );
    expect(sessions[0]!.status).toBe("completed");
    expect(sessions[0]!.closing?.fallbackUsed).toBe(true);
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
