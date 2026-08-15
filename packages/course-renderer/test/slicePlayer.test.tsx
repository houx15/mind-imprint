import { act, render } from "@testing-library/react";
import { RuntimeEventBus, InMemorySessionAdapter, type CourseRuntimeAdapters } from "@mind-imprint/course-runtime";
import { SlicePlayer } from "../src/slice/SlicePlayer";
import { AudioEngineProvider } from "../src/narration/audioEngine";
import { FakeAudioEngine } from "./support/fakeAudioEngine";
import { sliceOne, STATIC_PART_ID } from "./support/staticCourse";

function makeIdFactory() {
  let n = 0;
  return () => `ev-${++n}`;
}
const clock = () => "2026-08-16T00:00:00.000Z";

function fallbackGenerator() {
  return {
    generate: async () => ({ text: "", generatedAt: clock(), usedSignalTypes: [], fallbackUsed: true }),
  };
}

async function setup() {
  const sessionAdapter = new InMemorySessionAdapter({ idFactory: makeIdFactory(), clock });
  const session = await sessionAdapter.create({ courseId: "static-demo-course", studentId: "student-1" });
  const adapters: CourseRuntimeAdapters = {
    assetResolver: { resolve: (p) => `/resolved/${p}` },
    sessionAdapter,
    openingGenerator: fallbackGenerator(),
    closingGenerator: fallbackGenerator(),
  };
  const bus = new RuntimeEventBus({
    courseId: "static-demo-course",
    sessionId: session.id,
    idFactory: makeIdFactory(),
    clock,
  });
  return { sessionAdapter, session, adapters, bus };
}

describe("SlicePlayer", () => {
  it("drives a slice from intro narration through reveal to completion + navigation", async () => {
    const { sessionAdapter, session, adapters, bus } = await setup();
    const engine = new FakeAudioEngine();
    const onSliceComplete = vi.fn();
    const onNavigateNext = vi.fn();

    let container!: HTMLElement;
    await act(async () => {
      const r = render(
        <AudioEngineProvider value={engine}>
          <SlicePlayer
            slice={sliceOne}
            partId={STATIC_PART_ID}
            sessionId={session.id}
            adapters={adapters}
            bus={bus}
            onSliceComplete={onSliceComplete}
            onNavigateNext={onNavigateNext}
          />
        </AudioEngineProvider>,
      );
      container = r.container;
    });

    // intro step played the narration through the engine
    expect(engine.calls.some((c) => c.op === "play" && c.url === "/resolved/audio/s1.mp3")).toBe(true);

    // the reveal block starts hidden
    const revealWrapper = () => container.querySelector('[data-block-id="s1-reveal-text"]') as HTMLElement;
    expect(revealWrapper()).toHaveAttribute("hidden");

    // narration.ended → reveal step shows the hidden block + enables continue
    act(() => engine.fireEnded());
    expect(revealWrapper()).not.toHaveAttribute("hidden");
    expect(onSliceComplete).not.toHaveBeenCalled();

    // fire the continue control through the bus → done step completes + navigates
    const emit = bus.bindSlice(STATIC_PART_ID, sliceOne.id);
    act(() => emit("s1-continue", "student.continue"));

    expect(onSliceComplete).toHaveBeenCalledTimes(1);
    expect(onNavigateNext).toHaveBeenCalledTimes(1);

    // persisted slice state reflects completion + the revealed block
    const persisted = await sessionAdapter.load(session.id);
    const sliceState = persisted!.sliceStates[sliceOne.id]!;
    expect(sliceState.status).toBe("completed");
    expect(sliceState.blockStates["s1-reveal-text"]!.visible).toBe(true);
    expect(sliceState.blockStates["s1-continue"]!.enabled).toBe(true);
  });

  it("ignores events for a non-active slice (bus scoping)", async () => {
    const { adapters, bus, session } = await setup();
    const engine = new FakeAudioEngine();
    const onSliceComplete = vi.fn();
    const onNavigateNext = vi.fn();

    await act(async () => {
      render(
        <AudioEngineProvider value={engine}>
          <SlicePlayer
            slice={sliceOne}
            partId={STATIC_PART_ID}
            sessionId={session.id}
            adapters={adapters}
            bus={bus}
            onSliceComplete={onSliceComplete}
            onNavigateNext={onNavigateNext}
          />
        </AudioEngineProvider>,
      );
    });

    // an emitter bound to a DIFFERENT slice is dropped by the bus
    const strayEmit = bus.bindSlice(STATIC_PART_ID, "some-other-slice");
    act(() => strayEmit("s1-continue", "student.continue"));
    expect(onSliceComplete).not.toHaveBeenCalled();
  });
});
