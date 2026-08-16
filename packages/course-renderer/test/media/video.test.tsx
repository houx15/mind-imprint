import { act, render } from "@testing-library/react";
import type { BlockSessionState } from "@mind-imprint/course-contract";
import type { SliceEmitter } from "@mind-imprint/course-runtime";
import { VideoRenderer } from "../../src/blocks/media/VideoRenderer";
import type { VideoBlock } from "../../src/blocks/types";
import { VideoEngineProvider } from "../../src/media/videoEngine";
import { MediaHandleRegistry, MediaHandleRegistryProvider } from "../../src/media/mediaRegistry";
import { FakeVideoEngine } from "../support/fakeVideoEngine";

const assetResolver = { resolve: (p: string) => `/resolved/${p}` };

interface Recorded {
  sourceId: string;
  type: string;
  payload: unknown;
}

const baseState: BlockSessionState = { visible: true, enabled: true, completed: false };

function renderVideo(block: VideoBlock) {
  const events: Recorded[] = [];
  const emit: SliceEmitter = (sourceId, type, payload) => events.push({ sourceId, type: String(type), payload });
  const engine = new FakeVideoEngine();
  const registry = new MediaHandleRegistry();

  const utils = render(
    <MediaHandleRegistryProvider value={registry}>
      <VideoEngineProvider value={engine.factory}>
        <VideoRenderer block={block} assetResolver={assetResolver} state={baseState} visible enabled emit={emit} />
      </VideoEngineProvider>
    </MediaHandleRegistryProvider>,
  );
  return { ...utils, events, engine, registry };
}

const endedRuleBlock: VideoBlock = {
  id: "case-video",
  type: "video",
  source: "assets/videos/case.mp4",
  poster: "assets/images/case-poster.jpg",
  captions: "assets/captions/case.en.vtt",
  durationSeconds: 90,
  completion: { rule: "video-ended" },
};

const noCompletionBlock: VideoBlock = {
  id: "plain-video",
  type: "video",
  source: "assets/videos/plain.mp4",
};

const typeNames = (events: Recorded[]) => events.map((e) => e.type);

describe("VideoRenderer", () => {
  it("registers a media handle whose play drives the engine and emits video.started", () => {
    const { engine, registry, events } = renderVideo(endedRuleBlock);
    const handle = registry.get("case-video");
    expect(handle).toBeDefined();
    act(() => handle!.play());
    expect(engine.calls).toContain("play");
    expect(typeNames(events)).toContain("video.started");
  });

  it("handle pause drives the engine and emits video.paused with the current position", () => {
    const { engine, registry, events } = renderVideo(endedRuleBlock);
    engine.advanceTo(37);
    act(() => registry.get("case-video")!.pause());
    expect(engine.calls).toContain("pause");
    expect(typeNames(events)).toContain("video.paused");
    expect(events.find((e) => e.type === "video.paused")).toMatchObject({ payload: { positionSeconds: 37 } });
  });

  it("engine ended with completion video-ended emits video.ended (with position) then block.completed", () => {
    const { engine, events } = renderVideo(endedRuleBlock);
    engine.advanceTo(90);
    act(() => engine.fireEnded());
    expect(typeNames(events)).toEqual(["video.ended", "block.completed"]);
    expect(events[0]).toMatchObject({ sourceId: "case-video", payload: { positionSeconds: 90 } });
  });

  it("with no completion rule, ended emits video.ended but NOT block.completed", () => {
    const { engine, events } = renderVideo(noCompletionBlock);
    act(() => engine.fireEnded());
    expect(typeNames(events)).toEqual(["video.ended"]);
    expect(typeNames(events)).not.toContain("block.completed");
  });

  it("renders a <video> with the resolved source, poster, and a caption track", () => {
    const { container } = renderVideo(endedRuleBlock);
    const video = container.querySelector("video")!;
    expect(video).toBeInTheDocument();
    expect(video).toHaveAttribute("src", "/resolved/assets/videos/case.mp4");
    expect(video).toHaveAttribute("poster", "/resolved/assets/images/case-poster.jpg");
    const track = container.querySelector("track");
    expect(track).toHaveAttribute("src", "/resolved/assets/captions/case.en.vtt");
    expect(track).toHaveAttribute("kind", "captions");
  });

  it("block.completed is emitted at most once even if ended fires twice", () => {
    const { engine, events } = renderVideo(endedRuleBlock);
    act(() => engine.fireEnded());
    act(() => engine.fireEnded());
    expect(events.filter((e) => e.type === "block.completed")).toHaveLength(1);
  });
});
