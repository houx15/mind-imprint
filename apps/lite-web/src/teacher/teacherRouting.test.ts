import { describe, expect, it } from "vitest";
import {
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
  ["/classes/c1/students/u1", { view: "student", classId: "c1", userId: "u1" }],
  ["/classes/c1/students/u1/items/a1", { view: "item", classId: "c1", userId: "u1", atomId: "a1" }],
  ["/settings", { view: "settings" }],
  ["/overview", { view: "overview" }],
  ["/teachers", { view: "teachers" }],
  ["/import", { view: "import" }],
  ["/assignments", { view: "assignments" }],
  ["/assignments/new", { view: "assignmentNew" }],
  ["/assignments/a1", { view: "assignment", assignmentId: "a1" }],
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

  it("keeps a teacher on the assignments routes instead of falling back to the landing route", () => {
    expect(resolveTeacherRoute("/assignments", "teacher")).toEqual({ view: "assignments" });
    expect(resolveTeacherRoute("/assignments/new", "teacher")).toEqual({ view: "assignmentNew" });
    expect(resolveTeacherRoute("/assignments/a1", "teacher")).toEqual({ view: "assignment", assignmentId: "a1" });
  });
});
