import { describe, expect, it } from "vitest";
import { prosePendingLabel, showReadingTakeaway } from "./ItemPage";

describe("showReadingTakeaway", () => {
  it("hides the reading takeaway when the report already shows it as her own keep", () => {
    expect(showReadingTakeaway("多看数据来源", { text: "多看数据来源", source: "student" })).toBe(false);
    expect(showReadingTakeaway("多看数据来源 ", { text: "多看数据来源", source: "student" })).toBe(false);
  });
  it("shows it when the keep came from 印记 or says something else", () => {
    expect(showReadingTakeaway("多看数据来源", { text: "多看数据来源", source: "coach" })).toBe(true);
    expect(showReadingTakeaway("多看数据来源", { text: "另一句", source: "student" })).toBe(true);
    expect(showReadingTakeaway("多看数据来源", null)).toBe(true);
  });
  it("shows nothing when there is no takeaway", () => {
    expect(showReadingTakeaway("", null)).toBe(false);
  });
});

describe("prosePendingLabel", () => {
  it("shows nothing once the prose is not pending", () => {
    expect(prosePendingLabel(false, false)).toBeNull();
    expect(prosePendingLabel(false, true)).toBeNull();
  });

  it("says it's still generating before the one retry has run", () => {
    expect(prosePendingLabel(true, false)).toBe("报告文字生成中");
  });

  it("gives up honestly once the one retry has run and it's still pending", () => {
    expect(prosePendingLabel(true, true)).toBe("报告文字暂未生成");
  });
});
