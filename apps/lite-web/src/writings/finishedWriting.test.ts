import { describe, expect, it } from "vitest";
import type { AssignmentForAtom } from "../api/assignments";
import { effectiveDueAt, finishedChip, isReturnOpen, versionLine } from "./finishedWriting";

function homework(over: Partial<AssignmentForAtom> = {}): AssignmentForAtom {
  return {
    id: "a",
    kind: "writing",
    title: "雨",
    dueAt: "2026-09-20T14:00:00Z",
    returnedAt: null,
    returnDueAt: null,
    returnNote: null,
    resubmitted: false,
    ...over,
  };
}

describe("finishedChip", () => {
  it("is 已完成 for a writing that is not homework", () =>
    expect(finishedChip({ assignment: null, locked: false })).toBe("已完成"));
  it("is 已提交 for submitted homework", () => expect(finishedChip({ assignment: homework(), locked: false })).toBe("已提交"));
  it("is 已退回 while a return is open", () =>
    expect(finishedChip({ assignment: homework({ returnedAt: "r", returnDueAt: "d" }), locked: false })).toBe("已退回"));
  it("is back to 已提交 once she resubmits", () =>
    expect(finishedChip({ assignment: homework({ returnedAt: "r", returnDueAt: "d", resubmitted: true }), locked: false })).toBe(
      "已提交",
    ));
  it("is 已锁定 whenever locked, returned or not", () => {
    expect(finishedChip({ assignment: homework(), locked: true })).toBe("已锁定");
    expect(finishedChip({ assignment: homework({ returnedAt: "r", returnDueAt: "d" }), locked: true })).toBe("已锁定");
  });
});

describe("isReturnOpen / effectiveDueAt", () => {
  it("uses the return deadline once returned", () => {
    const a = homework({ returnedAt: "2026-09-21T01:00:00Z", returnDueAt: "2026-09-23T14:00:00Z" });
    expect(isReturnOpen(a)).toBe(true);
    expect(effectiveDueAt(a)).toBe("2026-09-23T14:00:00Z");
  });
  it("keeps the return deadline after a resubmission", () =>
    expect(effectiveDueAt(homework({ returnedAt: "r", returnDueAt: "2026-09-23T14:00:00Z", resubmitted: true }))).toBe(
      "2026-09-23T14:00:00Z",
    ));
  it("uses the assignment deadline when never returned", () => {
    expect(isReturnOpen(homework())).toBe(false);
    expect(effectiveDueAt(homework())).toBe("2026-09-20T14:00:00Z");
  });
});

describe("versionLine", () => {
  it("formats number, Beijing time and word count", () =>
    expect(versionLine({ number: 3, title: "t", wordCount: 812, submittedAt: "2026-09-15T06:20:00Z" }, "zh")).toBe(
      "v3 · 9月15日 14:20 · 812 字",
    ));
  it("counts words for an English piece", () =>
    expect(versionLine({ number: 1, title: "t", wordCount: 240, submittedAt: "2026-09-15T06:20:00Z" }, "en")).toBe(
      "v1 · 9月15日 14:20 · 240 词",
    ));
});
