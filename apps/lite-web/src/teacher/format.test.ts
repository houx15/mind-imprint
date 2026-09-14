import { describe, expect, it } from "vitest";
import { formatMinutes, itemStatusLabel, kindLabel, langLabel } from "./format";

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
