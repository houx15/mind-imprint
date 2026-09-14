import { describe, expect, it } from "vitest";
import {
  normalizeClassWeekly,
  normalizeClassWeeklyProse,
  normalizeStudentWeekly,
  normalizeStudentWeeklyProse,
  studentWeekIsEmpty,
  suggestionLabel,
  classCardProse,
} from "./weekly";

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
      prose: { summary: "本周读完《一场雨》。", suggestions: null },
      proseReady: true,
    });
    expect(w.prose).toEqual({ summary: "本周读完《一场雨》。", suggestions: [] });
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
    const prose = { summary: "本周读完《一场雨》。", suggestions: [] };
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
      watch: [{ kind: "watch", code: "overdue", label: "作业逾期", evidence: "本周到期的作业中有 1 份未完成。", userId: "u1", name: "王思远" }],
      prose: null,
    });
    expect(w.title).toBe("上周班级周报 · 第 37 周（9.7–9.13）");
    expect(w.stats.assignmentRate).toBe(-1);
    expect(w.stats.classSize).toBe(2);
    expect(w.praise).toEqual([]);
    expect(w.watch).toEqual([
      { kind: "watch", code: "overdue", label: "作业逾期", evidence: "本周到期的作业中有 1 份未完成。", userId: "u1", name: "王思远" },
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

describe("studentWeekIsEmpty", () => {
  const base = normalizeStudentWeekly({});
  it("is empty with no cards and no activity", () => expect(studentWeekIsEmpty(base)).toBe(true));
  it("is not empty with a card", () =>
    expect(studentWeekIsEmpty({ ...base, cards: [{ kind: "watch", code: "never_used", label: "本周未使用", evidence: "" }] })).toBe(false));
  it("is not empty with an overdue assignment", () =>
    expect(studentWeekIsEmpty({ ...base, facts: { ...base.facts, assignmentsOverdue: 1 } })).toBe(false));
  it("is not empty with turns only", () => expect(studentWeekIsEmpty({ ...base, facts: { ...base.facts, turns: 2 } })).toBe(false));
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
