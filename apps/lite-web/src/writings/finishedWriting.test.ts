import { describe, expect, it } from "vitest";
import type { AssignmentForAtom } from "../api/assignments";
import { ApiError } from "../api/client";
import type { Writing } from "../api/writings";
import {
  chipToShow,
  effectiveDueAt,
  finishedChip,
  isReturnOpen,
  isWritingLockedError,
  returnedLine,
  showFinishedPage,
  stageAfterRevise,
  versionLine,
} from "./finishedWriting";

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

describe("chipToShow", () => {
  it("shows nothing while the assignment lookup is loading", () =>
    expect(chipToShow("loading", { assignment: null, locked: false })).toBeNull());
  it("shows nothing when the lookup failed — never guesses 已完成", () =>
    expect(chipToShow("failed", { assignment: homework(), locked: false })).toBeNull());
  it("defers to finishedChip once loaded", () => {
    expect(chipToShow("loaded", { assignment: null, locked: false })).toBe("已完成");
    expect(chipToShow("loaded", { assignment: homework(), locked: true })).toBe("已锁定");
  });
});

describe("stageAfterRevise", () => {
  it("forces 结构 onto 成稿 so 修改 opens the write view", () => expect(stageAfterRevise("outline")).toBe("draft"));
  it("leaves 段落 alone", () => expect(stageAfterRevise("snippets")).toBeNull());
  it("leaves 成稿 alone", () => expect(stageAfterRevise("draft")).toBeNull());
  it("leaves an unrecognised stage alone", () => expect(stageAfterRevise("finished")).toBeNull());
});

describe("returnedLine", () => {
  it("shows the return deadline in Beijing time", () =>
    expect(returnedLine(homework({ returnedAt: "r", returnDueAt: "2026-09-18T14:00:00Z" }))).toBe("已退回 · 截止 9月18日 22:00"));
});

describe("showFinishedPage", () => {
  const finished: Pick<Writing, "status" | "finishedAt" | "revisingAt"> = { status: "finished", finishedAt: "2026-09-15T06:00:00Z" };
  it("shows the page for a finished writing that is not being revised", () =>
    expect(showFinishedPage({ ...finished, revisingAt: null }, false)).toBe(true));
  it("opens the room while revising", () =>
    expect(showFinishedPage({ ...finished, revisingAt: "2026-09-15T07:00:00Z" }, false)).toBe(false));
  it("keeps the page when revising but locked", () =>
    expect(showFinishedPage({ ...finished, revisingAt: "2026-09-15T07:00:00Z" }, true)).toBe(true));
  it("never shows it for an open writing", () =>
    expect(showFinishedPage({ status: "active", finishedAt: null, revisingAt: null }, false)).toBe(false));
});

describe("isWritingLockedError", () => {
  it("recognises a 403 writing_locked ApiError", () =>
    expect(isWritingLockedError(new ApiError("writing_locked", "已过截止时间，作业已锁定", 403))).toBe(true));
  it("rejects a different code at 403", () =>
    expect(isWritingLockedError(new ApiError("writing_finished", "这篇已经完成", 403))).toBe(false));
  it("rejects writing_locked at a different status", () =>
    expect(isWritingLockedError(new ApiError("writing_locked", "已过截止时间，作业已锁定", 409))).toBe(false));
  it("rejects a non-ApiError", () => expect(isWritingLockedError(new Error("network down"))).toBe(false));
});
