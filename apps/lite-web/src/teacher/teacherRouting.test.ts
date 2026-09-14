import { describe, expect, it } from "vitest";
import { parseTeacherRoute, teacherRoutePath, type TeacherRoute } from "./teacherRouting";

const cases: [string, TeacherRoute][] = [
  ["/classes", { view: "classes" }],
  ["/classes/c1", { view: "class", classId: "c1" }],
  ["/classes/c1/students/u1", { view: "student", classId: "c1", userId: "u1" }],
  ["/classes/c1/students/u1/items/a1", { view: "item", classId: "c1", userId: "u1", atomId: "a1" }],
  ["/settings", { view: "settings" }],
  ["/overview", { view: "overview" }],
  ["/teachers", { view: "teachers" }],
  ["/import", { view: "import" }],
];

describe("teacher routing", () => {
  it.each(cases)("parses %s", (path, route) => expect(parseTeacherRoute(path)).toEqual(route));
  it.each(cases)("round-trips %s", (path, route) => expect(teacherRoutePath(route)).toBe(path));
  it("unknown paths land on the class list", () => {
    expect(parseTeacherRoute("/")).toEqual({ view: "classes" });
    expect(parseTeacherRoute("/readings/abc")).toEqual({ view: "classes" });
    expect(parseTeacherRoute("/classes/c1/students")).toEqual({ view: "class", classId: "c1" });
  });
});
