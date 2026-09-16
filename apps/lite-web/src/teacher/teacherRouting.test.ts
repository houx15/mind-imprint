import { describe, expect, it } from "vitest";
import {
  isTeacherPath,
  landingRoute,
  parseTeacherRoute,
  resolveTeacherRoute,
  teacherRoutePath,
  type TeacherRoute,
} from "./teacherRouting";

const cases: [string, TeacherRoute][] = [
  ["/classes", { view: "classes" }],
  ["/classes/c1", { view: "class", classId: "c1" }],
  ["/classes/c1/weekly", { view: "classWeekly", classId: "c1" }],
  ["/classes/c1/chat", { view: "classChat", classId: "c1" }],
  ["/classes/c1/students/u1", { view: "student", classId: "c1", userId: "u1" }],
  ["/classes/c1/students/u1/items/a1", { view: "item", classId: "c1", userId: "u1", atomId: "a1" }],
  ["/settings", { view: "settings" }],
  ["/overview", { view: "overview" }],
  ["/teachers", { view: "teachers" }],
  ["/import", { view: "import" }],
  ["/assignments", { view: "assignments" }],
  ["/assignments/new", { view: "assignmentNew" }],
  ["/assignments/a1", { view: "assignment", assignmentId: "a1" }],
  ["/parent-reports", { view: "parentReports" }],
  ["/parent-reports/r1", { view: "parentReport", reportId: "r1" }],
];

describe("teacher routing", () => {
  it.each(cases)("parses %s", (path, route) => expect(parseTeacherRoute(path)).toEqual(route));
  it.each(cases)("round-trips %s", (path, route) => expect(teacherRoutePath(route)).toBe(path));
  it("unknown paths land on the class list", () => {
    expect(parseTeacherRoute("/")).toEqual({ view: "classes" });
    expect(parseTeacherRoute("/readings/abc")).toEqual({ view: "classes" });
    expect(parseTeacherRoute("/classes/c1/students")).toEqual({ view: "class", classId: "c1" });
  });

  // `weekly` is a reserved segment after the class id, never a student id.
  it("keeps the class weekly page apart from student routes", () => {
    expect(parseTeacherRoute("/classes/c1/weekly")).toEqual({ view: "classWeekly", classId: "c1" });
    expect(parseTeacherRoute("/classes/c1/weekly/")).toEqual({ view: "classWeekly", classId: "c1" });
    expect(parseTeacherRoute("/classes/c1/students/u1")).toEqual({ view: "student", classId: "c1", userId: "u1" });
    expect(parseTeacherRoute("/classes/c1/students/weekly")).toEqual({ view: "student", classId: "c1", userId: "weekly" });
  });

  it("resolves the class weekly page for a teacher", () => {
    expect(resolveTeacherRoute("/classes/c1/weekly", "teacher")).toEqual({ view: "classWeekly", classId: "c1" });
  });

  it("round-trips the grading view", () => {
    expect(parseTeacherRoute("/gradings/g-1")).toEqual({ view: "grading", gradingId: "g-1" });
    expect(teacherRoutePath({ view: "grading", gradingId: "g-1" })).toBe("/gradings/g-1");
    expect(isTeacherPath("/gradings/g-1")).toBe(true);
    expect(parseTeacherRoute("/gradings")).toEqual({ view: "classes" });
  });

  // 返回 from the grading view carries a hint back to the 批改 tab
  // (`?tab=grading`) instead of always landing on the default 学生 tab.
  it("carries the 批改-tab hint on the assignment route, and ignores it when absent", () => {
    expect(parseTeacherRoute("/assignments/a1?tab=grading")).toEqual({ view: "assignment", assignmentId: "a1", tab: "grading" });
    expect(teacherRoutePath({ view: "assignment", assignmentId: "a1", tab: "grading" })).toBe("/assignments/a1?tab=grading");
    expect(parseTeacherRoute("/assignments/a1")).toEqual({ view: "assignment", assignmentId: "a1" });
    expect(teacherRoutePath({ view: "assignment", assignmentId: "a1" })).toBe("/assignments/a1");
    // An unrecognised query value is not the grading hint.
    expect(parseTeacherRoute("/assignments/a1?tab=students")).toEqual({ view: "assignment", assignmentId: "a1" });
    expect(isTeacherPath("/assignments/a1?tab=grading")).toBe(true);
  });
});

describe("landingRoute", () => {
  it("admin lands on overview", () => expect(landingRoute("admin")).toEqual({ view: "overview" }));
  it("teacher lands on classes", () => expect(landingRoute("teacher")).toEqual({ view: "classes" }));
});

describe("resolveTeacherRoute", () => {
  it("sends a teacher away from admin-only views to classes", () => {
    expect(resolveTeacherRoute("/overview", "teacher")).toEqual({ view: "classes" });
    expect(resolveTeacherRoute("/teachers", "teacher")).toEqual({ view: "classes" });
    expect(resolveTeacherRoute("/import", "teacher")).toEqual({ view: "classes" });
  });

  it("keeps an admin on admin-only views", () => {
    expect(resolveTeacherRoute("/overview", "admin")).toEqual({ view: "overview" });
    expect(resolveTeacherRoute("/teachers", "admin")).toEqual({ view: "teachers" });
    expect(resolveTeacherRoute("/import", "admin")).toEqual({ view: "import" });
  });

  it("lands each role on its own tab at the root path", () => {
    expect(resolveTeacherRoute("/", "admin")).toEqual({ view: "overview" });
    expect(resolveTeacherRoute("/", "teacher")).toEqual({ view: "classes" });
  });

  it("treats a leftover student path (or any unrecognised path) as unset, not literal", () => {
    expect(resolveTeacherRoute("/readings/abc", "admin")).toEqual({ view: "overview" });
    expect(resolveTeacherRoute("/readings/abc", "teacher")).toEqual({ view: "classes" });
  });

  it("resolves a deep link the same way for both roles", () => {
    const expected = { view: "student", classId: "c1", userId: "u1" };
    expect(resolveTeacherRoute("/classes/c1/students/u1", "admin")).toEqual(expected);
    expect(resolveTeacherRoute("/classes/c1/students/u1", "teacher")).toEqual(expected);
  });

  // Without `parent-reports` in `isTeacherPath`, a reload on the editor would
  // silently land her on 班级 (or an admin on 概览).
  it("keeps both roles on the parent report routes", () => {
    for (const role of ["teacher", "admin"]) {
      expect(resolveTeacherRoute("/parent-reports", role)).toEqual({ view: "parentReports" });
      expect(resolveTeacherRoute("/parent-reports/r1/", role)).toEqual({ view: "parentReport", reportId: "r1" });
    }
  });

  it("leaves the class routes' reserved segments unchanged", () => {
    expect(resolveTeacherRoute("/classes/c1/weekly", "teacher")).toEqual({ view: "classWeekly", classId: "c1" });
    expect(resolveTeacherRoute("/classes/parent-reports", "teacher")).toEqual({ view: "class", classId: "parent-reports" });
    expect(parseTeacherRoute("/classes/c1/students/u1/parent-reports")).toEqual({
      view: "student",
      classId: "c1",
      userId: "u1",
    });
  });

  it("encodes a report id on the way out", () => {
    expect(teacherRoutePath({ view: "parentReport", reportId: "a/b" })).toBe("/parent-reports/a%2Fb");
    expect(parseTeacherRoute("/parent-reports/a%2Fb")).toEqual({ view: "parentReport", reportId: "a/b" });
  });

  it("keeps a teacher on the assignments routes instead of falling back to the landing route", () => {
    expect(resolveTeacherRoute("/assignments", "teacher")).toEqual({ view: "assignments" });
    expect(resolveTeacherRoute("/assignments/new", "teacher")).toEqual({ view: "assignmentNew" });
    expect(resolveTeacherRoute("/assignments/a1", "teacher")).toEqual({ view: "assignment", assignmentId: "a1" });
  });
});
