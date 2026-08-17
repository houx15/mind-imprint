import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import type { CourseSession, CourseRuntimeEvent, SliceSessionState, RuntimeSceneResult } from "@mind-imprint/course-contract";

import { createCourseSession, saveCourseSession } from "@/api/courseDefinition";
import { makeApiSessionAdapter } from "@/course/apiSessionAdapter";

vi.mock("@/api/courseDefinition", () => ({
  createCourseSession: vi.fn(),
  saveCourseSession: vi.fn(),
}));
const createMock = vi.mocked(createCourseSession);
const saveMock = vi.mocked(saveCourseSession);

const SLUG = "evidence-comparability";

function serverSession(): CourseSession {
  return {
    id: "session-server-1",
    courseId: "evidence-comparability",
    courseSchemaVersion: "2.0",
    studentId: "student-1",
    status: "created",
    sliceStates: {},
    events: [],
  };
}

const event: CourseRuntimeEvent = {
  id: "ev-1",
  sessionId: "session-server-1",
  courseId: "evidence-comparability",
  sourceId: "slice-observe-and-answer",
  type: "student.continue",
  occurredAt: "2026-08-16T00:00:00.000Z",
  payload: null,
};

const sliceState: SliceSessionState = {
  status: "completed",
  elapsedSeconds: 12,
  blockStates: {},
};

const scene: RuntimeSceneResult = {
  text: "welcome",
  generatedAt: "2026-08-16T00:00:00.000Z",
  usedSignalTypes: [],
  fallbackUsed: true,
};

beforeEach(() => {
  createMock.mockReset();
  saveMock.mockReset();
  createMock.mockResolvedValue(serverSession());
  saveMock.mockResolvedValue(undefined);
});

afterEach(() => {
  vi.useRealTimers();
});

describe("makeApiSessionAdapter", () => {
  it("create() returns the server session via POST get-or-create", async () => {
    const adapter = makeApiSessionAdapter(SLUG);
    const session = await adapter.create({ courseId: "evidence-comparability", studentId: "student-1" });
    expect(createMock).toHaveBeenCalledWith(SLUG);
    expect(session.id).toBe("session-server-1");
    expect(session.status).toBe("created");
  });

  it("load() resolves the server session (POST get-or-create is the resume path)", async () => {
    const adapter = makeApiSessionAdapter(SLUG);
    const loaded = await adapter.load("session-server-1");
    expect(createMock).toHaveBeenCalledWith(SLUG);
    expect(loaded?.id).toBe("session-server-1");
  });

  it("a mutating call then flush PUTs the updated blob", async () => {
    const adapter = makeApiSessionAdapter(SLUG, { debounceMs: 400 });
    const created = await adapter.create({ courseId: "evidence-comparability", studentId: "student-1" });
    await adapter.appendEvent(created.id, event);
    expect(saveMock).not.toHaveBeenCalled(); // debounced, not yet flushed

    await adapter.flush();
    expect(saveMock).toHaveBeenCalledTimes(1);
    const [slug, saved] = saveMock.mock.calls[0]!;
    expect(slug).toBe(SLUG);
    expect(saved.events).toHaveLength(1);
    expect(saved.events[0]!.id).toBe("ev-1");
  });

  it("debounces several mutations into ONE coalesced snapshot", async () => {
    vi.useFakeTimers();
    const adapter = makeApiSessionAdapter(SLUG, { debounceMs: 400 });
    const created = await adapter.create({ courseId: "evidence-comparability", studentId: "student-1" });

    await adapter.appendEvent(created.id, event);
    await adapter.saveSliceState(created.id, "slice-observe-and-answer", sliceState);
    await adapter.saveScene(created.id, "opening", scene);
    expect(saveMock).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(400);
    expect(saveMock).toHaveBeenCalledTimes(1);
    const [, saved] = saveMock.mock.calls[0]!;
    expect(saved.events).toHaveLength(1);
    expect(saved.sliceStates["slice-observe-and-answer"]).toEqual(sliceState);
    expect(saved.opening).toEqual(scene);
  });

  it("setStatus('completed') flushes immediately (student may leave)", async () => {
    const adapter = makeApiSessionAdapter(SLUG);
    const created = await adapter.create({ courseId: "evidence-comparability", studentId: "student-1" });
    await adapter.setStatus(created.id, "completed");
    expect(saveMock).toHaveBeenCalledTimes(1);
    const [, saved] = saveMock.mock.calls[0]!;
    expect(saved.status).toBe("completed");
  });

  // D5 / P2-08: CoursePlayer calls this to stamp the content-hash it just
  // detected (new session, or after a stale-hash reset) — the PUT must carry
  // it just like every other single-field setter.
  it("setDefinitionHash() updates the held session's `courseDefinitionHash` and schedules a snapshot", async () => {
    const adapter = makeApiSessionAdapter(SLUG, { debounceMs: 400 });
    const created = await adapter.create({ courseId: "evidence-comparability", studentId: "student-1" });
    await adapter.setDefinitionHash(created.id, "def-hash-abc");
    await adapter.flush();
    expect(saveMock).toHaveBeenCalledTimes(1);
    const [, saved] = saveMock.mock.calls[0]!;
    expect(saved.courseDefinitionHash).toBe("def-hash-abc");
  });

  // D5 / P2-08 durability regression: CoursePlayer's stale-hash reset must
  // reach the SERVER, not just the renderer's in-memory cache — otherwise a
  // later resume, once the stamped hash matches, resurrects the discarded
  // slice/step. resetProgress() must actually clear the held session's
  // `current`/`sliceStates` so the next PUT carries the cleared state.
  it("resetProgress() clears the held session's `current` and `sliceStates`, and the PUT reflects it", async () => {
    createMock.mockResolvedValueOnce({
      ...serverSession(),
      status: "in-progress",
      current: { partId: "part-1", sliceId: "slice-observe-and-answer", workflowStepId: "step-1" },
      sliceStates: { "slice-observe-and-answer": sliceState },
    });
    const adapter = makeApiSessionAdapter(SLUG, { debounceMs: 400 });
    const created = await adapter.create({ courseId: "evidence-comparability", studentId: "student-1" });
    expect(created.current).not.toBeUndefined();
    expect(created.sliceStates).not.toEqual({});

    await adapter.resetProgress(created.id);
    await adapter.flush();

    expect(saveMock).toHaveBeenCalledTimes(1);
    const [, saved] = saveMock.mock.calls[0]!;
    expect(saved.current).toBeUndefined();
    expect(saved.sliceStates).toEqual({});
  });

  it("setCurrent() updates the held session's `current` and schedules a snapshot", async () => {
    const adapter = makeApiSessionAdapter(SLUG, { debounceMs: 400 });
    const created = await adapter.create({ courseId: "evidence-comparability", studentId: "student-1" });
    await adapter.setCurrent(created.id, { partId: "part-1", sliceId: "slice-observe-and-answer", workflowStepId: "step-1" });
    await adapter.flush();
    expect(saveMock).toHaveBeenCalledTimes(1);
    const [, saved] = saveMock.mock.calls[0]!;
    expect(saved.current).toEqual({ partId: "part-1", sliceId: "slice-observe-and-answer", workflowStepId: "step-1" });
  });

  // P2-07: a rejection must not be mistaken for a successful save — the
  // snapshot stays dirty and a later flush re-sends the SAME data.
  it("a rejected save keeps the snapshot dirty; a subsequent flush re-sends it", async () => {
    const adapter = makeApiSessionAdapter(SLUG);
    const created = await adapter.create({ courseId: "evidence-comparability", studentId: "student-1" });
    await adapter.appendEvent(created.id, event);

    saveMock.mockRejectedValueOnce(new Error("network down"));
    await expect(adapter.flush()).rejects.toThrow("network down");
    expect(adapter.getSaveStatus()).toBe("error");
    expect(saveMock).toHaveBeenCalledTimes(1);

    // A second flush re-sends the still-dirty snapshot — nothing was dropped.
    await adapter.flush();
    expect(saveMock).toHaveBeenCalledTimes(2);
    const [, secondSave] = saveMock.mock.calls[1]!;
    expect(secondSave.events).toHaveLength(1);
    expect(secondSave.events[0]!.id).toBe("ev-1");
    expect(adapter.getSaveStatus()).toBe("saved");
  });

  it("the debounced auto-save path does not throw an unhandled rejection on failure", async () => {
    vi.useFakeTimers();
    try {
      saveMock.mockRejectedValueOnce(new Error("boom"));
      const adapter = makeApiSessionAdapter(SLUG, { debounceMs: 50 });
      const created = await adapter.create({ courseId: "evidence-comparability", studentId: "student-1" });
      await adapter.appendEvent(created.id, event);

      // Advancing past the debounce window fires the auto-save; it rejects,
      // but must be caught internally (no bare `void flush()`).
      await vi.advanceTimersByTimeAsync(50);
      expect(saveMock).toHaveBeenCalledTimes(1);
      expect(adapter.getSaveStatus()).toBe("error");

      // The retry (bounded backoff) is armed and eventually re-sends.
      saveMock.mockResolvedValueOnce(undefined);
      await vi.advanceTimersByTimeAsync(1_000);
      expect(saveMock.mock.calls.length).toBeGreaterThanOrEqual(2);
      expect(adapter.getSaveStatus()).toBe("saved");
    } finally {
      vi.useRealTimers();
    }
  });
});
