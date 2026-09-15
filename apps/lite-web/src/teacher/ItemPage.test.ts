import { describe, expect, it } from "vitest";
import { commentPointLines, itemGradingAction, prosePendingLabel, showReadingTakeaway, teacherManuscript } from "./ItemPage";

describe("teacherManuscript", () => {
  const v = (number: number) => ({ number, title: "雨", wordCount: 12, submittedAt: "2026-09-15T06:20:00Z" });
  it("shows the latest submitted version when versions exist", () =>
    expect(teacherManuscript({ versions: [v(2), v(1)], revising: false, draft: "还没提交的修改" })).toEqual({
      kind: "version",
      version: v(2),
      revising: false,
    }));
  // While she revises, the draft is unsubmitted: the teacher still reads the
  // latest version, labelled 修改中.
  it("keeps the version and flags revising", () =>
    expect(teacherManuscript({ versions: [v(1)], revising: true, draft: "还没提交的修改" })).toEqual({
      kind: "version",
      version: v(1),
      revising: true,
    }));
  it("falls back to the draft when there is no version", () =>
    expect(teacherManuscript({ versions: [], revising: false, draft: "草稿" })).toEqual({ kind: "draft", draft: "草稿" }));
  it("treats a missing draft as empty", () =>
    expect(teacherManuscript({ versions: [], revising: false, draft: null })).toEqual({ kind: "draft", draft: "" }));
});

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

describe("commentPointLines", () => {
  // The bug this pins: AI comment points are objects, and the page used to
  // render only string points, so teachers saw only the summaries.
  it("renders object points with their quote and action, and old string points as notes", () => {
    expect(
      commentPointLines([
        "旧格式的一条意见",
        { kind: "good", text: "用具体经历引出问题。", quote: "去年秋天，我在那里摔过一跤。" },
        { kind: "issue", text: "材料与观点之间没有说明。", action: "补一句说明。", quote: "我读到城市里的雨水花园。" },
        { kind: "issue", text: "" },
        42,
      ]),
    ).toEqual([
      { kind: "note", text: "旧格式的一条意见", action: null, quote: null },
      { kind: "good", text: "用具体经历引出问题。", action: null, quote: "去年秋天，我在那里摔过一跤。" },
      { kind: "issue", text: "材料与观点之间没有说明。", action: "补一句说明。", quote: "我读到城市里的雨水花园。" },
    ]);
  });

  it("drops a blank string and a malformed point entirely", () => {
    expect(commentPointLines(["   ", null, undefined, { text: "   " }])).toEqual([]);
  });
});

describe("itemGradingAction", () => {
  it("offers 批改 only for a submitted writing without a grading, and opens an existing one", () => {
    expect(itemGradingAction(0, null)).toBe("none");
    expect(itemGradingAction(1, null)).toBe("grade");
    expect(itemGradingAction(2, { id: "g", status: "draft", overallGrade: "B", error: null, reviewedAt: null, sentAt: null })).toBe("open");
  });

  it("opens the existing grading even with no submitted version left in the payload", () => {
    // Should never happen from a real response (a grading implies a
    // version), but the function must not crash or misclassify it as "grade".
    expect(itemGradingAction(0, { id: "g", status: "sent", overallGrade: "A", error: null, reviewedAt: null, sentAt: "2026-09-15T06:20:00Z" })).toBe(
      "open",
    );
  });
});
