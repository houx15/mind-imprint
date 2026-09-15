import { describe, expect, it } from "vitest";
import { lensFieldLabels, summarizeOutput } from "./outputSummary";

// Payload interpretation is a data boundary: retain zero/false, never expose
// internal IDs as student prose, and derive card labels from the shared registry.
describe("teacher output summaries", () => {
  it("summarizes result text and counts without leaking IDs", () => {
    expect(summarizeOutput({ count: 0, picked: "internal-id", idea: "保留学生原话" })).toEqual([
      { label: "想法数量", text: "0" }, { label: "选定方案", text: "保留学生原话" },
    ]);
  });
  it("preserves false rather than calling it missing", () => {
    expect(summarizeOutput({ published: false })).toEqual([{ label: "已发布", text: "否" }]);
  });
  it("does not stringify unfamiliar objects into the summary", () => {
    expect(summarizeOutput({ newField: { arbitrary: "x" }, body: { broken: true } })).toEqual([]);
    expect(summarizeOutput(null)).toEqual([]);
    expect(summarizeOutput([])).toEqual([]);
    expect(summarizeOutput({ constructor: "not a label" })).toEqual([]);
  });
  it("preserves literal user wording and renders list fields", () => {
    expect(summarizeOutput("<script>kept</script>")[0]?.text).toBe("<script>kept</script>");
    expect(summarizeOutput({ keywords: ["数据", "证据"] })[0]?.text).toBe("数据；证据");
  });
  it("interprets only the verdict field as an enum", () => {
    expect(summarizeOutput({ verdict: "revise", body: "revise" })).toEqual([
      { label: "审核结论", text: "需要修改" }, { label: "正文", text: "revise" },
    ]);
  });
  it("uses registry labels for known lenses; unknown lenses have no invented labels", () => {
    expect(lensFieldLabels("逻辑学：推理有没有跳步？").finding).toContain("推理缺口");
    expect(lensFieldLabels("unknown")).toEqual({});
  });
});
