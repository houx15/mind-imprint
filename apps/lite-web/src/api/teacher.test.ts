import { describe, expect, it, vi } from "vitest";

vi.mock("./client", () => ({
  apiFetch: vi.fn(async () => ({
    roster: [{ id: "u1", displayName: "林同学", minutesThisWeek: -1 }],
  })),
}));

import { getRoster, normalizeItemDetail } from "./teacher";

describe("getRoster", () => {
  it("fills missing numeric fields with 0 and keeps -1", async () => {
    const [row] = await getRoster("c1");
    if (!row) throw new Error("expected a roster row");
    expect(row.minutesThisWeek).toBe(-1);
    expect(row.turns).toBe(0);
    expect(row.lastActiveAt).toBeNull();
  });
});

describe("normalizeItemDetail", () => {
  // C1: `atom_report.go`'s report fields are `json:",omitempty"`, so a
  // phase-1 report (`prosePending: true`) — the common case right after an
  // item finishes — arrives with `moments`/`stats` as the JSON literal
  // `null`, not `[]`. A project with no live plan version yet arrives with
  // `steps: null` the same way. Neither should ever throw or leave a raw
  // `null` where `ItemPage` expects an array.
  it("turns a null report and null project.steps into arrays, not a throw", () => {
    const raw = {
      item: { atomId: "a1", kind: "writing", title: "t", status: "active", level: null, minutes: 0, turns: 0, createdAt: "", lastActiveAt: "", finishedAt: null },
      reading: null,
      writing: null,
      project: {
        idea: "idea", status: "running", stepsDone: 0, stepsTotal: 0,
        steps: null, tools: null, artifacts: null, keeps: null, courses: null, siteToken: null,
      },
      report: { stats: null, moments: null, keep: null, prosePending: true },
      reportError: null,
    };

    const detail = normalizeItemDetail(raw);

    expect(detail.report).not.toBeNull();
    expect(detail.report?.stats).toEqual([]);
    expect(detail.report?.moments).toEqual([]);
    expect(detail.report?.keep).toBeNull();
    expect(detail.report?.prosePending).toBe(true);

    expect(detail.project?.steps).toEqual([]);
    expect(detail.project?.tools).toEqual([]);
    expect(detail.project?.artifacts).toEqual([]);
    expect(detail.project?.keeps).toEqual([]);
    expect(detail.project?.courses).toEqual([]);
  });

  it("defaults a null reading/writing sub-object to null, and null arrays inside it to []", () => {
    const raw = {
      item: { atomId: "a1", kind: "reading", title: "t", status: "active", level: 1, minutes: 0, turns: 0, createdAt: "", lastActiveAt: "", finishedAt: null },
      reading: { source: null, highlights: null, takeaway: null, lenses: [{ title: "CRAAP", fields: null }] },
      writing: null,
      project: null,
      report: null,
      reportError: null,
    };

    const detail = normalizeItemDetail(raw);

    expect(detail.reading?.highlights).toEqual([]);
    expect(detail.reading?.lenses).toEqual([{ title: "CRAAP", fields: {} }]);
    expect(detail.writing).toBeNull();
    expect(detail.project).toBeNull();
    expect(detail.report).toBeNull();
  });
});
