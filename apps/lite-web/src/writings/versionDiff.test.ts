import { describe, expect, it } from "vitest";
import { diffChars, diffVersions } from "./versionDiff";

// The diff is shown as 与当前版本对比: `older` is the version she picked,
// `newer` is the latest version. "add" = in the latest only, "del" = gone from it.
describe("diffVersions", () => {
  it("marks identical texts as unchanged paragraphs", () =>
    expect(diffVersions("甲段。\n\n乙段。", "甲段。\n\n乙段。")).toEqual([
      { kind: "same", text: "甲段。" },
      { kind: "same", text: "乙段。" },
    ]));

  it("reports an added paragraph", () =>
    expect(diffVersions("甲段。", "甲段。\n\n新的一段。")).toEqual([
      { kind: "same", text: "甲段。" },
      { kind: "add", text: "新的一段。" },
    ]));

  it("reports a deleted paragraph", () =>
    expect(diffVersions("甲段。\n\n删掉的一段。\n\n乙段。", "甲段。\n\n乙段。")).toEqual([
      { kind: "same", text: "甲段。" },
      { kind: "del", text: "删掉的一段。" },
      { kind: "same", text: "乙段。" },
    ]));

  it("pairs a replaced paragraph and marks characters inside it", () =>
    expect(diffVersions("雨下了一整天。", "雨下了一整夜。")).toEqual([
      {
        kind: "change",
        parts: [
          { kind: "same", text: "雨下了一整" },
          { kind: "del", text: "天" },
          { kind: "add", text: "夜" },
          { kind: "same", text: "。" },
        ],
      },
    ]));

  it("pairs in order and leaves the extra paragraph as an addition", () =>
    expect(diffVersions("A段\n\nB段", "A段\n\nB段改\n\nC段")).toEqual([
      { kind: "same", text: "A段" },
      { kind: "change", parts: [{ kind: "same", text: "B段" }, { kind: "add", text: "改" }] },
      { kind: "add", text: "C段" },
    ]));

  it("ignores blank-line and carriage-return differences between paragraphs", () =>
    expect(diffVersions("甲段。\r\n\r\n\r\n乙段。", "甲段。\n\n乙段。")).toEqual([
      { kind: "same", text: "甲段。" },
      { kind: "same", text: "乙段。" },
    ]));

  it("handles empty inputs", () => {
    expect(diffVersions("", "")).toEqual([]);
    expect(diffVersions("", "新")).toEqual([{ kind: "add", text: "新" }]);
    expect(diffVersions("旧", "")).toEqual([{ kind: "del", text: "旧" }]);
  });
});

describe("diffChars", () => {
  it("keeps astral characters whole", () =>
    expect(diffChars("a😀b", "a😀c")).toEqual([
      { kind: "same", text: "a😀" },
      { kind: "del", text: "b" },
      { kind: "add", text: "c" },
    ]));

  it("falls back to whole-paragraph replace when the table would be too large", () => {
    const older = "甲".repeat(600);
    const newer = "乙".repeat(600);
    expect(diffChars(older, newer)).toEqual([
      { kind: "del", text: older },
      { kind: "add", text: newer },
    ]);
  });
});
