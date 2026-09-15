import { describe, expect, it } from "vitest";
import type { GradingContent, GradingRow, Rubric } from "../api/gradings";
import {
  contentForSave,
  gradeInScale,
  gradingContentReducer,
  gradingRowStatus,
  queueResultText,
  reviewedDraftIds,
  sendResultText,
  shouldPoll,
  validateGradingContent,
} from "./gradingLogic";

const row = (over: Partial<GradingRow>): GradingRow => ({
  userId: "u",
  displayName: "Phoebe",
  atomId: "a",
  version: { number: 1, submittedAt: "2026-09-15T06:20:00Z" },
  grading: null,
  ...over,
});
const summary = (status: string, reviewedAt: string | null = null, error: string | null = null) =>
  ({ id: "g", status, overallGrade: null, error, reviewedAt, sentAt: null }) as GradingRow["grading"];

describe("gradingRowStatus", () => {
  it("maps every server state to one row status", () => {
    expect(gradingRowStatus(row({ version: null }))).toBe("not_submitted");
    expect(gradingRowStatus(row({}))).toBe("pending");
    expect(gradingRowStatus(row({ grading: summary("queued") }))).toBe("running");
    expect(gradingRowStatus(row({ grading: summary("running") }))).toBe("running");
    expect(gradingRowStatus(row({ grading: summary("draft") }))).toBe("draft");
    expect(gradingRowStatus(row({ grading: summary("draft", "2026-09-15T07:00:00Z") }))).toBe("reviewed");
    expect(gradingRowStatus(row({ grading: summary("sent", "2026-09-15T07:00:00Z") }))).toBe("sent");
    expect(gradingRowStatus(row({ grading: summary("failed") }))).toBe("failed");
  });
  // Controller ruling: a failed REGRADE keeps the previous draft — the
  // server hands this back as status "draft" (or "reviewed", if it had
  // already been reviewed) with `error` still set, not as "failed". The row
  // status reads as an ordinary draft/reviewed row; the error is read off
  // `grading.error` directly (via `failureText`) so the editor stays open
  // instead of being replaced by the dead-end 批改失败 state.
  it("keeps a failed regrade's row as draft/reviewed, with the error still readable", () => {
    const withError = row({ grading: summary("draft", null, "模型调用失败：超时") });
    expect(gradingRowStatus(withError)).toBe("draft");
    expect(withError.grading?.error).toBe("模型调用失败：超时");
    const reviewedWithError = row({ grading: summary("draft", "2026-09-15T07:00:00Z", "模型调用失败：超时") });
    expect(gradingRowStatus(reviewedWithError)).toBe("reviewed");
    expect(reviewedWithError.grading?.error).toBe("模型调用失败：超时");
  });
  it("polls only while something is queued or running, and sends only reviewed drafts", () => {
    expect(shouldPoll(["draft", "queued"])).toBe(true);
    expect(shouldPoll(["draft", "failed", "sent"])).toBe(false);
    const rows = [
      row({ grading: { ...summary("draft", "t")!, id: "g1" } }),
      row({ grading: { ...summary("draft")!, id: "g2" } }),
      row({ grading: { ...summary("sent", "t")!, id: "g3" } }),
    ];
    expect(reviewedDraftIds(rows)).toEqual(["g1"]);
  });
});

const letter: Rubric = { scale: "letter", dimensions: [{ name: "内容", note: "" }], focus: "" };
const content = (): GradingContent => ({
  overall: { grade: "B", comment: "x" },
  dimensions: [{ name: "内容", grade: "B", comment: "" }],
  points: [{ kind: "issue", quote: "雨", text: "说明", action: "补充", source: "ai" }],
});

describe("gradingContentReducer", () => {
  it("edits, adds and deletes without mutating the input", () => {
    const start = content();
    let c = gradingContentReducer(start, { type: "overallGrade", value: "A-" });
    c = gradingContentReducer(c, { type: "dimensionComment", index: 0, value: "材料具体。" });
    c = gradingContentReducer(c, { type: "addPoint" });
    c = gradingContentReducer(c, { type: "pointText", index: 1, value: "请注明数据来源。" });
    c = gradingContentReducer(c, { type: "pointQuote", index: 1, value: "雨" });
    expect(start.overall.grade).toBe("B");
    expect(c.overall.grade).toBe("A-");
    expect(c.dimensions[0]!.comment).toBe("材料具体。");
    expect(c.points[1]).toEqual({ kind: "issue", quote: "雨", text: "请注明数据来源。", action: "", source: "teacher" });
    // points[0]'s action ("补充") is kept in memory across the switch to
    // "good" — only contentForSave nulls a good point's action, at the
    // save boundary, not the reducer (fix round 1: see the dedicated round
    // trip test below).
    c = gradingContentReducer(c, { type: "pointKind", index: 0, value: "good" });
    expect(c.points[0]!.kind).toBe("good");
    c = gradingContentReducer(c, { type: "deletePoint", index: 0 });
    expect(c.points.map((p) => p.text)).toEqual(["请注明数据来源。"]);
    expect(gradingContentReducer(c, { type: "load", content: start })).toBe(start);
  });
  // Fix round 1: switching issue → good → issue used to reset the action to
  // "" on the good step, so a typed action was lost as soon as she toggled
  // the point to "good" and back, even without ever clearing the field.
  it("keeps a typed action across an issue → good → issue round trip", () => {
    const start = content(); // points[0].action === "补充"
    let c = gradingContentReducer(start, { type: "pointKind", index: 0, value: "good" });
    expect(c.points[0]!.action).toBe("补充"); // untouched in memory, not yet nulled
    c = gradingContentReducer(c, { type: "pointKind", index: 0, value: "issue" });
    expect(c.points[0]!.action).toBe("补充"); // restored, not reset to ""
  });
  it("contentForSave trims and turns blank quote/action into null", () => {
    const c = content();
    c.points[0] = { kind: "issue", quote: "  ", text: " 说明 ", action: "", source: "teacher" };
    expect(contentForSave(c).points[0]).toEqual({ kind: "issue", quote: null, text: "说明", action: null, source: "teacher" });
  });
  // The reducer no longer nulls a good point's action; contentForSave is
  // still the one place that must, since that's what the server receives.
  it("contentForSave still nulls a good point's leftover in-memory action", () => {
    const c = content();
    c.points[0] = { kind: "good", quote: "雨", text: "开头有画面感", action: "补充", source: "teacher" };
    expect(contentForSave(c).points[0]!.action).toBeNull();
  });
});

describe("queueResultText", () => {
  it("reports queued-only, failed-only and mixed results, always surfacing the backend error", () => {
    expect(queueResultText({ queued: 3, failed: 0, error: null })).toBe("已加入批改队列：3 份");
    expect(queueResultText({ queued: 0, failed: 0, error: null })).toBe("没有待批改的作业");
    expect(queueResultText({ queued: 2, failed: 1, error: "入队失败：connection refused" })).toBe(
      "已加入批改队列：2 份；入队失败 1 份（入队失败：connection refused）",
    );
    // A queue-all where every eligible recipient failed is a 503, not this
    // 200 shape (the caller never calls this with queued:0, failed>0 in
    // practice) — but the helper still degrades sensibly if it did.
    expect(queueResultText({ queued: 0, failed: 2, error: "入队失败：queue down" })).toBe("入队失败 2 份（入队失败：queue down）");
  });
});

describe("sendResultText", () => {
  it("reports a clean send, and explains a skip rather than leaving a bare number", () => {
    expect(sendResultText({ sent: 4, skipped: 0 })).toBe("已发送 4 份");
    expect(sendResultText({ sent: 3, skipped: 1 })).toBe("已发送 3 份，跳过 1 份（批改状态已变化或学生已不在班级）");
  });
});

describe("validateGradingContent", () => {
  it("checks grades against the scale and point texts", () => {
    expect(validateGradingContent(content(), letter)).toBeNull();
    const bad = content();
    bad.overall.grade = "E";
    expect(validateGradingContent(bad, letter)).toBe("总评的等级不在评分标准内：E");
    const empty = content();
    empty.points[0]!.text = " ";
    expect(validateGradingContent(empty, letter)).toBe("第 1 条意见的说明为空");
    const points: Rubric = { scale: "points", max: 20, dimensions: [{ name: "内容", note: "" }], focus: "" };
    expect(gradeInScale(points, "20")).toBe(true);
    expect(gradeInScale(points, "08")).toBe(false);
    expect(gradeInScale(points, "21")).toBe(false);
  });
});
