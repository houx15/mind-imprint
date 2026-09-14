import { describe, expect, it } from "vitest";
import { formatMinutes, itemStatusLabel, kindLabel, langLabel, safeHttpUrl } from "./format";

// safeHttpUrl guards a student-controlled href on a teacher screen (React does
// not block javascript: URLs). A regression here is invisible in the UI.
describe("safeHttpUrl", () => {
  it("keeps absolute http and https URLs", () => {
    expect(safeHttpUrl("https://example.org/a?b=1")).toBe("https://example.org/a?b=1");
    expect(safeHttpUrl("http://example.org")).toBe("http://example.org");
  });
  it("refuses other schemes", () => {
    expect(safeHttpUrl("javascript:alert(1)")).toBeNull();
    expect(safeHttpUrl("JavaScript:alert(1)")).toBeNull();
    expect(safeHttpUrl("data:text/html,<script>alert(1)</script>")).toBeNull();
  });
  it("refuses relative, empty and garbage input", () => {
    expect(safeHttpUrl("/p/abc")).toBeNull();
    expect(safeHttpUrl("")).toBeNull();
    expect(safeHttpUrl(null)).toBeNull();
    expect(safeHttpUrl("not a url at all")).toBeNull();
  });
});

describe("langLabel", () => {
  it("names zh and en, dashes empty, passes unknown codes through", () => {
    expect([langLabel("zh"), langLabel("en"), langLabel(""), langLabel("fr")]).toEqual(["中文", "英文", "—", "fr"]);
  });
});

describe("formatMinutes", () => {
  it("shows a dash when no time was recorded", () => expect(formatMinutes(-1)).toBe("—"));
  it("minutes under an hour", () => expect(formatMinutes(45)).toBe("45 分钟"));
  it("whole hours drop the minutes", () => expect(formatMinutes(120)).toBe("2 小时"));
  it("hours and minutes", () => expect(formatMinutes(95)).toBe("1 小时 35 分钟"));
  it("zero is zero, not a dash", () => expect(formatMinutes(0)).toBe("0 分钟"));
});

describe("itemStatusLabel", () => {
  it("reading finished", () => expect(itemStatusLabel("reading", "finished")).toBe("已完成"));
  it("project keeping counts as done", () => expect(itemStatusLabel("project", "keeping")).toBe("已完成"));
  it("unknown status passes through", () => expect(itemStatusLabel("project", "weird")).toBe("weird"));
});

describe("kindLabel", () => {
  it("names each kind", () => {
    expect([kindLabel("reading"), kindLabel("writing"), kindLabel("project")]).toEqual(["阅读", "写作", "项目"]);
  });
});
