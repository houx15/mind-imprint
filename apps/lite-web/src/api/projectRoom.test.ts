import { describe, expect, it } from "vitest";
import {
  PLAN_RESOLUTIONS,
  PLAN_RESOLUTION_LABELS,
  SESSION_KINDS,
  SESSION_KIND_LABELS,
  SESSION_REQUIRED_FIELD,
  SESSION_WRITEBACK_PROMPT,
  STEP_STATUSES,
  STEP_STATUS_LABELS,
  sessionTrail,
  type Session,
} from "./projectRoom";

function s(id: string, parentId: string | null, depth: number): Session {
  return {
    id, kind: "free", parentId, depth, anchorKind: "free", anchorRef: "",
    question: "", takeaway: "", closedAt: null, createdAt: "2026-09-01T00:00:00Z",
  };
}

describe("sessionTrail", () => {
  it("is empty on the main thread", () => {
    expect(sessionTrail([s("a", null, 0)], null)).toEqual([]);
  });

  it("walks from the main thread down to the session, oldest first", () => {
    const all = [s("a", null, 0), s("b", "a", 1), s("c", "b", 2)];
    expect(sessionTrail(all, "c").map((x) => x.id)).toEqual(["a", "b", "c"]);
  });

  it("returns what it can when the chain is broken, rather than throwing", () => {
    // 'b' names a parent that is not in the list — a partial trail beats a
    // crashed room.
    const all = [s("b", "missing", 1)];
    expect(() => sessionTrail(all, "b")).not.toThrow();
    expect(sessionTrail(all, "b").map((x) => x.id)).toEqual(["b"]);
  });

  it("terminates on a cycle instead of hanging", () => {
    const all = [s("a", "b", 0), s("b", "a", 1)];
    expect(() => sessionTrail(all, "a")).not.toThrow();
    expect(sessionTrail(all, "a").length).toBeLessThanOrEqual(8);
  });

  it("returns nothing for an id it has never seen", () => {
    expect(sessionTrail([s("a", null, 0)], "zzz")).toEqual([]);
  });
});

// These sweeps guard the same invariant from the client side: a value the
// server can legitimately send, with no label here, renders as blank chrome.
describe("vocabularies are complete", () => {
  it("can write back the server-created review and keeping sessions", () => {
    expect(SESSION_KINDS).toContain("review");
    expect(SESSION_KINDS).toContain("keeping");
    expect(SESSION_REQUIRED_FIELD.keeping).toBe("reading");
    expect(SESSION_REQUIRED_FIELD.review).toBeNull();
  });
  it("labels every session kind and gives each a write-back prompt", () => {
    for (const k of SESSION_KINDS) {
      expect(SESSION_KIND_LABELS[k]).toBeTruthy();
      expect(SESSION_WRITEBACK_PROMPT[k]).toBeTruthy();
      // null is meaningful for `free` (the takeaway IS the contract), so this
      // checks the key exists rather than that it is truthy.
      expect(k in SESSION_REQUIRED_FIELD).toBe(true);
    }
  });

  it("labels every step status", () => {
    for (const st of STEP_STATUSES) expect(STEP_STATUS_LABELS[st]).toBeTruthy();
    expect(Object.keys(STEP_STATUS_LABELS).sort()).toEqual([...STEP_STATUSES].sort());
  });

  it("labels every plan resolution, keeping 保留原计划 among them", () => {
    for (const r of PLAN_RESOLUTIONS) expect(PLAN_RESOLUTION_LABELS[r]).toBeTruthy();
    expect(PLAN_RESOLUTIONS).toContain("kept");
  });
});
