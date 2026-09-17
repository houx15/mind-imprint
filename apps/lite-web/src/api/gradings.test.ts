import { describe, expect, it } from "vitest";
import {
  normalizeGradingContent,
  normalizeGradingRow,
  normalizeRubric,
  normalizeStudentGrading,
  normalizeTeacherGrading,
} from "./gradings";

describe("normalizeRubric", () => {
  it("keeps a points rubric and drops blank dimensions", () => {
    expect(
      normalizeRubric({ scale: "points", max: 20, dimensions: [{ name: "论证", note: "证据" }, { name: "" }], focus: "重点" }),
    ).toEqual({ scale: "points", max: 20, dimensions: [{ name: "论证", note: "证据" }], focus: "重点" });
  });
  it("returns null for a missing or dimensionless rubric", () => {
    expect(normalizeRubric(undefined)).toBeNull();
    expect(normalizeRubric({ scale: "letter", dimensions: [] })).toBeNull();
  });
  it("reads an unknown scale as letter", () => {
    expect(normalizeRubric({ scale: "stars", dimensions: [{ name: "内容" }] })?.scale).toBe("letter");
  });
});

describe("normalizeGradingContent", () => {
  it("maps quote/action to null when absent and source to teacher unless ai", () => {
    const c = normalizeGradingContent({
      overall: { grade: "B", comment: "x" },
      dimensions: [{ name: "内容", grade: "B", comment: "" }],
      points: [
        { kind: "good", quote: "雨", text: "具体", action: null, source: "ai" },
        { kind: "strange", text: "说明", quote: 3 },
      ],
    });
    expect(c?.points).toEqual([
      { kind: "good", quote: "雨", text: "具体", action: null, source: "ai" },
      { kind: "issue", quote: null, text: "说明", action: null, source: "teacher" },
    ]);
  });
  it("returns null for null content", () => {
    expect(normalizeGradingContent(null)).toBeNull();
  });
});

describe("normalizeGradingRow and normalizeTeacherGrading", () => {
  it("keeps a row with no version and no grading", () => {
    expect(normalizeGradingRow({ userId: "u1", displayName: "Phoebe", atomId: null, version: null, grading: null })).toEqual({
      userId: "u1",
      displayName: "Phoebe",
      atomId: null,
      version: null,
      grading: null,
    });
  });

  it("keeps a row's grading summary, including a draft that still carries a leftover error", () => {
    // A failed regrade keeps the previous draft editable (controller ruling,
    // task-5-report.md Ruling 1): status:"draft" with content already saved
    // AND error non-null at the same time. The normalizer must not force
    // one or the other to null.
    const row = normalizeGradingRow({
      userId: "u2",
      displayName: "Ada",
      atomId: "w2",
      version: { number: 2, submittedAt: "2026-09-15T06:00:00Z" },
      grading: {
        id: "g9",
        status: "draft",
        overallGrade: "B+",
        error: "批改失败：超时",
        reviewedAt: null,
        sentAt: null,
      },
    });
    expect(row.grading).toEqual({
      id: "g9",
      status: "draft",
      overallGrade: "B+",
      error: "批改失败：超时",
      reviewedAt: null,
      sentAt: null,
    });
  });

  it("falls back to a generic rubric when the server sends none (defensive only — every real row carries one)", () => {
    // Binding decision 4: the frontend must not keep its own copy of the Go
    // default rubric's names/notes (apps/api/internal/liteassign.DefaultRubric).
    // The server always bakes an effective/default rubric into every grading
    // row before it is created (gradingRubricFor, task-5-report.md), so this
    // path is only a defensive guard against a malformed payload — it must
    // not reproduce "Task Response"/"内容" etc.
    const g = normalizeTeacherGrading({ id: "g1", lang: "en", status: "draft", content: null, rubric: null, versionNumber: 1 });
    expect(g.rubric.dimensions.length).toBeGreaterThan(0);
    expect(g.rubric.dimensions[0]?.name).not.toBe("Task Response");
    expect(g.status).toBe("draft");
  });

  it("keeps a real rubric snapshot as sent", () => {
    const g = normalizeTeacherGrading({
      id: "g2",
      lang: "zh",
      status: "sent",
      content: null,
      rubric: { scale: "points", max: 20, dimensions: [{ name: "论证", note: "重点" }], focus: "" },
      versionNumber: 1,
    });
    expect(g.rubric).toEqual({ scale: "points", max: 20, dimensions: [{ name: "论证", note: "重点" }], focus: "" });
  });
});

describe("normalizeStudentGrading", () => {
  it("drops a row without content", () => {
    expect(normalizeStudentGrading({ id: "g1", versionNumber: 1, content: null })).toBeNull();
  });

  it("keeps a sent row's rubric, content and seen flag", () => {
    const g = normalizeStudentGrading({
      id: "g3",
      versionNumber: 2,
      rubric: { scale: "letter", dimensions: [{ name: "内容", note: "" }], focus: "" },
      content: { overall: { grade: "A-", comment: "好" }, dimensions: [], points: [] },
      sentAt: "2026-09-15T06:20:00Z",
      seen: false,
      source: "teacher",
    });
    expect(g).toEqual({
      id: "g3",
      versionNumber: 2,
      rubric: { scale: "letter", dimensions: [{ name: "内容", note: "" }], focus: "" },
      content: { overall: { grade: "A-", comment: "好" }, dimensions: [], points: [] },
      sentAt: "2026-09-15T06:20:00Z",
      seen: false,
      source: "teacher",
    });
  });

  // A server from before 人工批改 sends no `source`, and every grading it had
  // was AI-drafted. Only the exact value "teacher" means a 人工批改.
  it("reads a missing or unknown source as ai", () => {
    const base = { id: "g4", content: { overall: { grade: "B", comment: "" }, dimensions: [], points: [] } };
    expect(normalizeStudentGrading(base)?.source).toBe("ai");
    expect(normalizeStudentGrading({ ...base, source: "TEACHER" })?.source).toBe("ai");
    expect(normalizeTeacherGrading({ ...base, source: "teacher" }).source).toBe("teacher");
  });
});
