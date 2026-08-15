import { act, render, screen } from "@testing-library/react";
import { NarrationController, NarrationPlayer } from "../src/narration/NarrationPlayer";
import { FakeAudioEngine } from "./support/fakeAudioEngine";
import type { NarrationDefinition } from "@mind-imprint/course-contract";
import type { SliceEmitter } from "@mind-imprint/course-runtime";

const narration = (id: string): NarrationDefinition => ({ id, text: `transcript for ${id}`, audio: `audio/${id}.mp3` });

describe("NarrationController + NarrationPlayer", () => {
  it("emits narration.ended with the narration id when the engine reports ended", () => {
    const engine = new FakeAudioEngine();
    const controller = new NarrationController(engine);
    const events: Array<{ sourceId: string; type: string }> = [];
    const emit: SliceEmitter = (sourceId, type) => events.push({ sourceId, type: String(type) });

    controller.play(narration("intro"), "/resolved/audio/intro.mp3", emit);
    engine.fireEnded();

    expect(events).toEqual([{ sourceId: "intro", type: "narration.ended" }]);
  });

  it("stops the current track before starting a second (single-track)", () => {
    const engine = new FakeAudioEngine();
    const controller = new NarrationController(engine);
    const emit: SliceEmitter = () => {};

    controller.play(narration("one"), "/resolved/audio/one.mp3", emit);
    controller.play(narration("two"), "/resolved/audio/two.mp3", emit);

    const ops = engine.calls.map((c) => c.op);
    // first play, then a stop, then the second play — stop precedes the 2nd play.
    expect(ops).toEqual(["play", "stop", "play"]);
    expect(engine.calls[0]).toMatchObject({ url: "/resolved/audio/one.mp3" });
    expect(engine.calls[2]).toMatchObject({ url: "/resolved/audio/two.mp3" });
  });

  it("firing ended after switching tracks does not re-emit for the stale narration", () => {
    const engine = new FakeAudioEngine();
    const controller = new NarrationController(engine);
    const events: Array<{ sourceId: string }> = [];
    const emit: SliceEmitter = (sourceId) => events.push({ sourceId });

    controller.play(narration("one"), "/one.mp3", emit);
    controller.play(narration("two"), "/two.mp3", emit);
    engine.fireEnded();

    expect(events).toEqual([{ sourceId: "two" }]);
  });

  it("renders the active narration transcript", () => {
    const engine = new FakeAudioEngine();
    const controller = new NarrationController(engine);
    const emit: SliceEmitter = () => {};
    render(<NarrationPlayer controller={controller} emit={emit} />);

    // nothing before a track starts
    expect(screen.queryByLabelText("讲解")).toBeNull();

    act(() => controller.play(narration("intro"), "/resolved/audio/intro.mp3", emit));
    expect(screen.getByText("transcript for intro")).toBeInTheDocument();
  });
});
