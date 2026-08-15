import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { BlockSessionState, VideoInteractionDocument } from "@mind-imprint/course-contract";
import type { SliceEmitter } from "@mind-imprint/course-runtime";
import { VideoRenderer } from "../../src/blocks/media/VideoRenderer";
import type { VideoBlock } from "../../src/blocks/types";
import { VideoEngineProvider } from "../../src/media/videoEngine";
import { MediaHandleRegistry, MediaHandleRegistryProvider } from "../../src/media/mediaRegistry";
import { InteractionLoaderProvider } from "../../src/blocks/media/VideoInteractionController";
import { FakeVideoEngine } from "../support/fakeVideoEngine";

const assetResolver = { resolve: (p: string) => `/resolved/${p}` };
const baseState: BlockSessionState = { visible: true, enabled: true, completed: false };

interface Recorded {
  sourceId: string;
  type: string;
  payload: unknown;
}

const gatedBlock: VideoBlock = {
  id: "case-video",
  type: "video",
  source: "assets/videos/case.mp4",
  durationSeconds: 195,
  interaction: { source: "interactions/video/case-video.json" },
  completion: { rule: "video-ended-and-interactions-completed" },
};

const doc: VideoInteractionDocument = {
  schemaVersion: "1.1",
  video: {
    blockId: "case-video",
    source: "assets/videos/case.mp4",
    durationSeconds: 195,
    cues: [
      {
        id: "prediction-check",
        atSeconds: 42,
        pauseVideo: true,
        required: true,
        prompt: "What do you predict will happen next?",
        activity: {
          type: "singleChoice",
          options: [
            { id: "same-basis", label: "The comparison basis will stay the same" },
            { id: "new-basis", label: "The comparison basis will change" },
          ],
          assessment: { mode: "survey" },
          completion: { rule: "submit-any" },
        },
      },
    ],
  },
};

function renderGated() {
  const events: Recorded[] = [];
  const emit: SliceEmitter = (sourceId, type, payload) => events.push({ sourceId, type: String(type), payload });
  const engine = new FakeVideoEngine();
  const registry = new MediaHandleRegistry();
  const utils = render(
    <InteractionLoaderProvider value={() => doc}>
      <MediaHandleRegistryProvider value={registry}>
        <VideoEngineProvider value={engine.factory}>
          <VideoRenderer block={gatedBlock} assetResolver={assetResolver} state={baseState} visible enabled emit={emit} />
        </VideoEngineProvider>
      </MediaHandleRegistryProvider>
    </InteractionLoaderProvider>,
  );
  return { ...utils, events, engine, registry };
}

const typeNames = (events: Recorded[]) => events.map((e) => e.type);

describe("VideoInteractionController (cue timeline, §14)", () => {
  it("at the cue time it pauses, emits video.interaction.shown, and renders the cue activity", () => {
    const { engine, events } = renderGated();
    act(() => engine.advanceTo(42));
    expect(engine.calls).toContain("pause");
    expect(typeNames(events)).toContain("video.interaction.shown");
    const shown = events.find((e) => e.type === "video.interaction.shown");
    expect(shown).toMatchObject({ payload: { interactionId: "prediction-check" } });
    expect(screen.getByRole("radiogroup")).toBeInTheDocument();
  });

  it("completing the cue emits video.interaction.completed and resumes the video", async () => {
    const user = userEvent.setup();
    const { engine, events } = renderGated();
    act(() => engine.advanceTo(42));
    const group = screen.getByRole("radiogroup");
    await user.click(within(group).getByRole("radio", { name: "The comparison basis will change" }));
    await user.click(screen.getByRole("button", { name: "提交" }));
    expect(typeNames(events)).toContain("video.interaction.completed");
    // resumed: a play call comes after the pause
    expect(engine.calls).toContain("play");
    expect(engine.calls.lastIndexOf("play")).toBeGreaterThan(engine.calls.indexOf("pause"));
  });

  it("after the required cue completes, ended emits block.completed", async () => {
    const user = userEvent.setup();
    const { engine, events } = renderGated();
    act(() => engine.advanceTo(42));
    await user.click(screen.getByRole("radio", { name: "The comparison basis will change" }));
    await user.click(screen.getByRole("button", { name: "提交" }));
    act(() => engine.fireEnded());
    expect(typeNames(events)).toContain("block.completed");
  });

  it("ended with the required cue NOT completed does NOT emit block.completed", () => {
    const { engine, events } = renderGated();
    act(() => engine.fireEnded());
    expect(typeNames(events)).toContain("video.ended");
    expect(typeNames(events)).not.toContain("block.completed");
  });

  it("a cue fires only once (idempotent across repeated time updates)", () => {
    const { engine, events } = renderGated();
    act(() => engine.advanceTo(42));
    act(() => engine.advanceTo(43));
    act(() => engine.advanceTo(44));
    expect(events.filter((e) => e.type === "video.interaction.shown")).toHaveLength(1);
  });
});
