import { describe, expect, it } from "vitest";
import type { ClassSummary } from "@/api";
import { emptySettings, type AssignmentDraft } from "./assignmentLogic";
import { draftPreviewItem } from "./StudentViewPreview";

function draft(over: Partial<AssignmentDraft> = {}): AssignmentDraft {
  return {
    ...emptySettings("reading"),
    classId: "c1",
    title: "溯源体检",
    instructions: "用 CRAAP 检查这篇文章的来源",
    dueInput: "2026-09-20T22:00",
    userIds: [],
    ...over,
  };
}

const classes: ClassSummary[] = [
  { id: "c1", name: "高二 3 班", join_code: "AAAA-BBBB", school_id: "s1", created_at: "2026-01-01T00:00:00Z" },
];

describe("draftPreviewItem", () => {
  it("maps each kind to its Chinese label", () => {
    expect(draftPreviewItem(draft({ kind: "reading" }), classes).kindLabel).toBe("阅读");
    expect(draftPreviewItem(draft({ kind: "writing" }), classes).kindLabel).toBe("写作");
    expect(draftPreviewItem(draft({ kind: "project" }), classes).kindLabel).toBe("项目");
  });

  it("formats the Beijing wall-clock dueInput the same way the student end does", () => {
    // 22:00 Beijing time, shown as 9月20日 22:00 regardless of viewer zone.
    expect(draftPreviewItem(draft({ dueInput: "2026-09-20T22:00" }), classes).dueLabel).toBe("9月20日 22:00");
  });

  it("leaves dueLabel null for an empty or malformed dueInput", () => {
    expect(draftPreviewItem(draft({ dueInput: "" }), classes).dueLabel).toBeNull();
    expect(draftPreviewItem(draft({ dueInput: "not-a-date" }), classes).dueLabel).toBeNull();
  });

  it("looks up the class name from the draft's classId", () => {
    expect(draftPreviewItem(draft({ classId: "c1" }), classes).className).toBe("高二 3 班");
  });

  it("returns an empty className when the draft's class isn't in the list", () => {
    expect(draftPreviewItem(draft({ classId: "missing" }), classes).className).toBe("");
  });

  it("shows a plain placeholder for an empty title, never a blank string", () => {
    expect(draftPreviewItem(draft({ title: "" }), classes).title).toBe("未填写标题");
    expect(draftPreviewItem(draft({ title: "   " }), classes).title).toBe("未填写标题");
  });

  it("trims instructions, so a whitespace-only draft renders no instructions line", () => {
    expect(draftPreviewItem(draft({ instructions: "   " }), classes).instructions).toBe("");
  });
});
