import { describe, expect, it } from "vitest";
import { ApiError } from "./client";
import {
  proseErrorText,
  normalizeClassWeekly,
  normalizeClassWeeklyProse,
  normalizeStudentWeekly,
  normalizeStudentWeeklyProse,
  suggestionLabel,
  classCardProse,
} from "./weekly";
import { weekBeforeStartMessage } from "./weekly";

describe("normalizeStudentWeekly", () => {
  it("keeps the server's title and fills missing arrays and numbers", () => {
    const w = normalizeStudentWeekly({
      weekStart: "2026-09-07",
      weekLabel: "第 37 周（9.7–9.13）",
      title: "上周表现总结 · 第 37 周（9.7–9.13）",
      isLatest: true,
      facts: { activeDays: 2, finished: null, moments: null },
      cards: null,
      prose: null,
      proseReady: false,
    });
    expect(w.title).toBe("上周表现总结 · 第 37 周（9.7–9.13）");
    expect(w.isLatest).toBe(true);
    expect(w.facts.activeDays).toBe(2);
    expect(w.facts.turns).toBe(0);
    expect(w.facts.finished).toEqual([]);
    expect(w.facts.stalled).toEqual([]);
    expect(w.facts.newKeywords).toEqual([]);
    expect(w.facts.moments).toEqual([]);
    expect(w.cards).toEqual([]);
    expect(w.prose).toBeNull();
    expect(w.proseReady).toBe(false);
  });

  // -1 means "no time records"; a missing field must not turn into 0 分钟.
  it("defaults a missing minutes to -1 and keeps a real -1", () => {
    expect(normalizeStudentWeekly({ facts: {} }).facts.minutes).toBe(-1);
    expect(normalizeStudentWeekly({ facts: { minutes: -1 } }).facts.minutes).toBe(-1);
    expect(normalizeStudentWeekly({ facts: { minutes: 0 } }).facts.minutes).toBe(0);
  });

  it("reads stored prose and treats a null suggestions list as empty", () => {
    const w = normalizeStudentWeekly({
      prose: { summary: "该周读完《一场雨》。", suggestions: null },
      proseReady: true,
    });
    expect(w.prose).toEqual({ summary: "该周读完《一场雨》。", suggestions: [] });
    expect(w.proseReady).toBe(true);
  });

  it("drops a prose object with no summary rather than rendering an empty block", () => {
    expect(normalizeStudentWeekly({ prose: { summary: "", suggestions: [] }, proseReady: true }).prose).toBeNull();
  });
});

describe("normalizeStudentWeeklyProse", () => {
  it("carries proseError on a 200 with prose null", () => {
    const r = normalizeStudentWeeklyProse({
      weekStart: "2026-09-07",
      prose: null,
      proseReady: false,
      proseError: "agent: lite student weekly prose: rejected after 2 attempts: summary: quote not in corpus: x",
    });
    expect(r.week.prose).toBeNull();
    expect(r.proseError).toBe("agent: lite student weekly prose: rejected after 2 attempts: summary: quote not in corpus: x");
  });

  it("has no error when proseError is null or blank", () => {
    const prose = { summary: "该周读完《一场雨》。", suggestions: [] };
    expect(normalizeStudentWeeklyProse({ prose, proseReady: true, proseError: null }).proseError).toBeNull();
    expect(normalizeStudentWeeklyProse({ prose, proseReady: true, proseError: "  " }).proseError).toBeNull();
  });

  // A 200 with neither prose nor an error is still a failure to the page:
  // otherwise it would show nothing and offer no 重试.
  it("reports a missing prose without an error as a failure", () => {
    expect(normalizeStudentWeeklyProse({ prose: null, proseReady: false }).proseError).toBe("没有返回总结");
  });
});

describe("normalizeClassWeekly", () => {
  it("keeps assignmentRate -1 and fills card lists", () => {
    const w = normalizeClassWeekly({
      title: "上周班级周报 · 第 37 周（9.7–9.13）",
      isLatest: true,
      stats: { classSize: 2, activeStudents: 1, minutes: 20, turns: 3, finished: 1 },
      praise: null,
      watch: [{ kind: "watch", code: "overdue", label: "作业逾期", evidence: "该周到期的作业中有 1 份未完成。", userId: "u1", name: "王思远" }],
      prose: null,
    });
    expect(w.title).toBe("上周班级周报 · 第 37 周（9.7–9.13）");
    expect(w.stats.assignmentRate).toBe(-1);
    expect(w.stats.classSize).toBe(2);
    expect(w.praise).toEqual([]);
    expect(w.watch).toEqual([
      { kind: "watch", code: "overdue", label: "作业逾期", evidence: "该周到期的作业中有 1 份未完成。", userId: "u1", name: "王思远" },
    ]);
  });

  it("reads class prose with a null cards list", () => {
    const w = normalizeClassWeekly({ prose: { comment: "全班一人活跃。", cards: null }, proseReady: true });
    expect(w.prose).toEqual({ comment: "全班一人活跃。", cards: [] });
  });
});

describe("normalizeClassWeeklyProse", () => {
  it("carries proseError", () => {
    const r = normalizeClassWeeklyProse({ prose: null, proseError: "agent: lite class weekly prose: rejected" });
    expect(r.proseError).toBe("agent: lite class weekly prose: rejected");
  });
});

describe("weekBeforeStartMessage", () => {
  it("returns the server message for week_before_start", () => {
    const e = new ApiError("week_before_start", "该周早于学生加入班级的时间", 400);
    expect(weekBeforeStartMessage(e)).toBe("该周早于学生加入班级的时间");
  });
  it("is null for any other error", () => {
    expect(weekBeforeStartMessage(new ApiError("invalid_week", "请选择已经结束的一周", 400))).toBeNull();
    expect(weekBeforeStartMessage(new Error("network"))).toBeNull();
  });
});

describe("proseErrorText", () => {
  it("appends a string details to the message", () => {
    const e = new ApiError("ai_dialogue_failed", "AI 响应错误", 502, "lite_student_weekly route: no LLM provider configured");
    expect(proseErrorText(e)).toBe("AI 响应错误（lite_student_weekly route: no LLM provider configured）");
  });
  it("is the message alone without details", () => {
    expect(proseErrorText(new ApiError("x", "AI 响应错误", 502))).toBe("AI 响应错误");
    expect(proseErrorText(new TypeError("Failed to fetch"))).toBe("Failed to fetch");
  });
});

// hasPrev and empty are decided on the server. The page disables 上一周 and
// skips the prose POST on them, so a missing or non-boolean value must not
// read as true.
describe("hasPrev and empty", () => {
  it("reads both flags on a student week and defaults them to false", () => {
    const w = normalizeStudentWeekly({ hasPrev: true, empty: true });
    expect([w.hasPrev, w.empty]).toEqual([true, true]);
    const missing = normalizeStudentWeekly({});
    expect([missing.hasPrev, missing.empty]).toEqual([false, false]);
    const loose = normalizeStudentWeekly({ hasPrev: "true", empty: 1 });
    expect([loose.hasPrev, loose.empty]).toEqual([false, false]);
  });

  it("reads both flags on a class week and defaults them to false", () => {
    const w = normalizeClassWeekly({ hasPrev: true, empty: true });
    expect([w.hasPrev, w.empty]).toEqual([true, true]);
    const missing = normalizeClassWeekly({});
    expect([missing.hasPrev, missing.empty]).toEqual([false, false]);
  });

  // An empty week's POST carries neither prose nor an error, by design: it is
  // not the 没有返回总结 failure.
  it("treats an empty week's POST without prose as no failure", () => {
    const s = normalizeStudentWeeklyProse({ prose: null, proseError: null, empty: true });
    expect(s.week.empty).toBe(true);
    expect(s.proseError).toBeNull();
    const c = normalizeClassWeeklyProse({ prose: null, proseError: null, empty: true });
    expect(c.week.empty).toBe(true);
    expect(c.proseError).toBeNull();
  });

  it("keeps a real proseError on an empty week", () => {
    expect(normalizeStudentWeeklyProse({ prose: null, proseError: "x", empty: true }).proseError).toBe("x");
  });
});

describe("suggestionLabel", () => {
  const cards = [
    { kind: "watch", code: "overdue", label: "作业逾期", evidence: "" },
    { kind: "praise", code: "new_interest", label: "新的兴趣", evidence: "" },
  ];
  it("names the card a suggestion rests on", () => expect(suggestionLabel(cards, "new_interest")).toBe("新的兴趣"));
  it("is empty for a code with no card", () => expect(suggestionLabel(cards, "stalled")).toBe(""));
});

describe("classCardProse", () => {
  it("finds the lead and action written for a student", () => {
    const prose = { comment: "", cards: [{ userId: "u2", lead: "读完一篇。", action: "当面表扬。" }] };
    expect(classCardProse(prose, "u2")).toEqual({ userId: "u2", lead: "读完一篇。", action: "当面表扬。" });
    expect(classCardProse(prose, "u1")).toBeNull();
    expect(classCardProse(null, "u2")).toBeNull();
  });
});
