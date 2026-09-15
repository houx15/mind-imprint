import { describe, expect, it } from "vitest";
import { isTeacherRailActive, teacherRailItems, type TeacherRailKey } from "./teacherRail";
import type { TeacherRoute } from "./teacherRouting";

describe("teacherRailItems", () => {
  it("teacher order", () =>
    expect(teacherRailItems("teacher").map((i) => i.label)).toEqual(["班级", "作业", "家长报告"]));
  it("admin order", () =>
    expect(teacherRailItems("admin").map((i) => i.label)).toEqual(["概览", "班级", "作业", "家长报告", "教师", "导入"]));
});

describe("isTeacherRailActive", () => {
  const cases: [TeacherRoute, TeacherRailKey | null][] = [
    [{ view: "classes" }, "classes"],
    [{ view: "class", classId: "c" }, "classes"],
    [{ view: "classWeekly", classId: "c" }, "classes"],
    [{ view: "student", classId: "c", userId: "u" }, "classes"],
    [{ view: "item", classId: "c", userId: "u", atomId: "a" }, "classes"],
    [{ view: "assignments" }, "assignments"],
    [{ view: "assignmentNew" }, "assignments"],
    [{ view: "assignment", assignmentId: "a" }, "assignments"],
    [{ view: "parentReports" }, "parentReports"],
    [{ view: "parentReport", reportId: "r" }, "parentReports"],
    [{ view: "overview" }, "overview"],
    [{ view: "teachers" }, "teachers"],
    [{ view: "import" }, "import"],
    [{ view: "settings" }, null],
  ];
  it.each(cases)("%o selects exactly %s", (route, expected) => {
    const active = teacherRailItems("admin")
      .map((i) => i.key)
      .filter((key) => isTeacherRailActive(key, route));
    expect(active).toEqual(expected ? [expected] : []);
  });
});
