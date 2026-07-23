import { describe, it, expect } from "vitest";
import { parseCsv } from "@/console/csv";

describe("parseCsv", () => {
  it("maps columns by header, any order, case-insensitive", () => {
    const rows = parseCsv("Student_Email,Class,Teacher_Email\ns@x,11A,t@x\n");
    expect(rows).toEqual([{ class: "11A", teacher_email: "t@x", student_email: "s@x" }]);
  });
  it("omits blank optional cells", () => {
    const rows = parseCsv("class,teacher_email,student_email\n11A,,\n");
    expect(rows).toEqual([{ class: "11A" }]);
  });
  it("supports quoted fields containing commas", () => {
    const rows = parseCsv('class,student_email\n"11A, TOK",s@x\n');
    expect(rows[0]!.class).toBe("11A, TOK");
  });
  it("skips blank lines", () => {
    const rows = parseCsv("class\n11A\n\n12B\n");
    expect(rows).toHaveLength(2);
  });
  it("throws on an empty file", () => {
    expect(() => parseCsv("   ")).toThrow();
  });
  it("throws when the class column is missing", () => {
    expect(() => parseCsv("teacher_email\nt@x\n")).toThrow();
  });
});
