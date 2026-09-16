import { describe, expect, it } from "vitest";
import { normalizeNavigate, type WorkspaceNavigate } from "../../api/teacherWorkspace";
import {
  assignmentCardRows,
  classSnapshotView,
  navigateOf,
  navigateRoute,
  NAVIGATE_CARD_KIND,
  withNavigateCard,
} from "./homeLogic";

const nav = (over: Partial<WorkspaceNavigate>): WorkspaceNavigate => ({ view: "classWeekly", classId: "c1", label: "本周报告", ...over });

describe("navigateRoute", () => {
  it.each([
    [nav({}), { view: "classWeekly", classId: "c1" }],
    [nav({ view: "student", userId: "u1", label: "林知遥的学习页" }), { view: "student", classId: "c1", userId: "u1" }],
    [nav({ view: "assignmentNew", label: "布置作业" }), { view: "assignmentNew", classId: "c1" }],
    [nav({ view: "assignment", assignmentId: "a1", label: "《气候》" }), { view: "assignment", assignmentId: "a1" }],
    [nav({ view: "parentReports", label: "家长报告" }), { view: "parentReports" }],
  ])("maps %o", (input, route) => {
    expect(navigateRoute(input, "c1")).toEqual({ route, label: input.label, classId: "c1" });
  });

  it.each([
    ["no offer", undefined],
    ["an unknown view", nav({ view: "grading" })],
    ["student without userId", nav({ view: "student" })],
    ["assignment without assignmentId", nav({ view: "assignment" })],
    ["an empty label", nav({ label: "  " })],
    ["another class", nav({ classId: "c2" })],
  ])("drops %s", (_, input) => {
    expect(navigateRoute(input, "c1")).toBeNull();
  });

  it("drops a wire offer whose ids are not strings", () => {
    const raw = normalizeNavigate({ view: "student", classId: "c1", label: "x", userId: 7 as unknown as string });
    expect(navigateRoute(raw, "c1")).toBeNull();
  });
});

describe("navigate card", () => {
  it("rides along with the tool cards and is found again", () => {
    const tool = { kind: "students", rows: [] };
    const offer = nav({});
    const cards = withNavigateCard([tool], offer);
    expect(cards).toEqual([tool, { kind: NAVIGATE_CARD_KIND, rows: offer }]);
    expect(navigateOf(cards)).toBe(offer);
  });

  it("adds nothing without an offer", () => {
    const cards = [{ kind: "students", rows: [] }];
    expect(withNavigateCard(cards, undefined)).toBe(cards);
    expect(navigateOf(cards)).toBeUndefined();
  });
});

describe("classSnapshotView", () => {
  const good = {
    classSize: 12,
    activeStudents: 9,
    finished: 4,
    assignmentRate: -1,
    praise: [{ userId: "u1", name: "林知遥", kind: "praise", code: "c", label: "坚持", evidence: "5 天" }, { name: "" }, "x"],
    watch: null,
  };

  it("reads the single-object rows and skips nameless students", () => {
    const v = classSnapshotView(good);
    expect(v?.classSize).toBe(12);
    expect(v?.assignmentRate).toBe(-1);
    expect(v?.praise.map((p) => p.name)).toEqual(["林知遥"]);
    expect(v?.watch).toEqual([]);
  });

  it("rejects an array or a missing count", () => {
    expect(classSnapshotView([good])).toBeNull();
    expect(classSnapshotView({ ...good, finished: undefined })).toBeNull();
    expect(classSnapshotView(null)).toBeNull();
  });
});

describe("assignmentCardRows", () => {
  it("maps kind and status keys to labels and never shows a raw key", () => {
    const rows = assignmentCardRows([
      {
        id: "a1",
        title: "气候",
        kind: "reading",
        dueAt: "2026-09-20T10:00:00Z",
        counts: { overdue: 2, not_started: 3, done: 0, mystery: 5 },
      },
      { id: "a2", title: "新类型", kind: "podcast", counts: null },
      { id: "", title: "无 id" },
      "junk",
    ]);
    expect(rows).toEqual([
      {
        id: "a1",
        title: "气候",
        kindLabel: "阅读",
        dueAt: "2026-09-20T10:00:00Z",
        counts: [
          { status: "not_started", label: "未开始", n: 3 },
          { status: "overdue", label: "已逾期", n: 2 },
        ],
      },
      { id: "a2", title: "新类型", kindLabel: "", dueAt: "", counts: [] },
    ]);
  });

  it("returns nothing for a non-array", () => {
    expect(assignmentCardRows({ id: "a1" })).toEqual([]);
  });
});
