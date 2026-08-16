import { describe, it, expect, vi, beforeEach } from "vitest";
import { InteractionLoadError } from "@mind-imprint/course-renderer";
import { makeInteractionLoader } from "@/course/interactionLoader";

const SOURCE = "interactions/video/case.json";
const SIGNED = "https://cdn/interactions/video/case.json?auth_key=x";

const validDoc = {
  schemaVersion: "1.1",
  video: {
    blockId: "b-video",
    source: "assets/videos/case.mp4",
    durationSeconds: 120,
    cues: [
      {
        id: "cue-1",
        atSeconds: 30,
        pauseVideo: true,
        required: true,
        prompt: "Which claim is stronger?",
        activity: {
          type: "singleChoice",
          options: [
            { id: "a", label: "A" },
            { id: "b", label: "B" },
          ],
          assessment: { mode: "graded", correctOptionId: "a" },
          completion: { rule: "submit-correct" },
        },
      },
    ],
  },
};

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
});

const assets = () => ({ [SOURCE]: SIGNED });

describe("makeInteractionLoader", () => {
  it("fetches, parses, and returns a valid VideoInteractionDocument", async () => {
    fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => validDoc });
    const load = makeInteractionLoader(assets);
    const doc = await load(SOURCE);
    expect(doc.schemaVersion).toBe("1.1");
    expect(doc.video.cues[0]!.id).toBe("cue-1");
    expect(fetchMock).toHaveBeenCalledWith(SIGNED);
  });

  it("caches by source — a second call does NOT refetch", async () => {
    fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => validDoc });
    const load = makeInteractionLoader(assets);
    await load(SOURCE);
    await load(SOURCE);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("rejects not-found when the source has no signed URL", async () => {
    const load = makeInteractionLoader(() => ({}));
    await expect(load(SOURCE)).rejects.toMatchObject({ name: "InteractionLoadError", kind: "not-found" });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("rejects not-found on a non-2xx response (e.g. expired 403 / 404)", async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 404, json: async () => ({}) });
    const load = makeInteractionLoader(assets);
    await expect(load(SOURCE)).rejects.toMatchObject({ kind: "not-found" });
  });

  it("rejects not-found on a network/fetch failure", async () => {
    fetchMock.mockRejectedValue(new Error("network down"));
    const load = makeInteractionLoader(assets);
    await expect(load(SOURCE)).rejects.toBeInstanceOf(InteractionLoadError);
    await expect(load(SOURCE)).rejects.toMatchObject({ kind: "not-found" });
  });

  it("rejects malformed on non-JSON", async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => {
        throw new Error("not json");
      },
    });
    const load = makeInteractionLoader(assets);
    await expect(load(SOURCE)).rejects.toMatchObject({ kind: "malformed" });
  });

  it("rejects malformed on a schema mismatch", async () => {
    fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => ({ schemaVersion: "2.0", nope: true }) });
    const load = makeInteractionLoader(assets);
    await expect(load(SOURCE)).rejects.toMatchObject({ kind: "malformed" });
  });
});
