import { describe, expect, it, vi } from "vitest";

vi.mock("./client", () => ({
  apiFetch: vi.fn(async () => ({
    roster: [{ id: "u1", displayName: "林同学", minutesThisWeek: -1 }],
  })),
}));

import { getRoster, normalizeItemDetail, normalizeRosterRow, normalizeStudentPage } from "./teacher";

describe("getRoster", () => {
  it("fills missing numeric fields with 0 and keeps -1", async () => {
    const [row] = await getRoster("c1");
    if (!row) throw new Error("expected a roster row");
    expect(row.minutesThisWeek).toBe(-1);
    expect(row.turns).toBe(0);
    expect(row.lastActiveAt).toBeNull();
  });

  it("defaults overdueAssignments to 0 when the field is missing", async () => {
    const [row] = await getRoster("c1");
    if (!row) throw new Error("expected a roster row");
    expect(row.overdueAssignments).toBe(0);
  });
});

describe("normalizeRosterRow", () => {
  it("keeps a real overdueAssignments count", () => {
    expect(normalizeRosterRow({ id: "u1", overdueAssignments: 3 }).overdueAssignments).toBe(3);
  });
});

describe("normalizeStudentPage", () => {
  it("defaults assignments to [] when the field is missing", () => {
    const page = normalizeStudentPage({ student: { id: "u1" } });
    expect(page.assignments).toEqual([]);
  });

  it("normalizes each assignment row", () => {
    const page = normalizeStudentPage({
      student: { id: "u1" },
      assignments: [
        { id: "a1", kind: "writing", title: "写一篇议论文", dueAt: "2026-09-20T14:00:00Z", status: "in_progress", statusLabel: "进行中", atomId: "atom1" },
      ],
    });
    expect(page.assignments).toEqual([
      { id: "a1", kind: "writing", title: "写一篇议论文", dueAt: "2026-09-20T14:00:00Z", status: "in_progress", statusLabel: "进行中", atomId: "atom1" },
    ]);
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

  // ItemPage labels `idea` 驱动问题（作业） on this flag. Only a real `true`
  // may do that; anything else would label her own sentence as the teacher's.
  it("reads project.assigned as a strict boolean", () => {
    const base = {
      item: { atomId: "a1", kind: "project", title: "t", status: "active", level: null, minutes: 0, turns: 0, createdAt: "", lastActiveAt: "", finishedAt: null },
      reading: null,
      writing: null,
      report: null,
      reportError: null,
    };
    expect(normalizeItemDetail({ ...base, project: { idea: "q", assigned: true } }).project?.assigned).toBe(true);
    expect(normalizeItemDetail({ ...base, project: { idea: "q" } }).project?.assigned).toBe(false);
    expect(normalizeItemDetail({ ...base, project: { idea: "q", assigned: "true" } }).project?.assigned).toBe(false);
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
