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
});
