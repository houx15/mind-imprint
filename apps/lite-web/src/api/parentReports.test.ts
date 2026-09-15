import { describe, expect, it } from "vitest";
import {
  normalizeHiddenMentions,
  normalizeParentReport,
  normalizeParentReportSummary,
  normalizeTeacherParentReport,
} from "./parentReports";

// The teacher DTO has names only inside facts and a null draft/body until one
// is stored.
describe("normalizeTeacherParentReport", () => {
  it("reads a fresh report whose draft failed", () => {
    const r = normalizeTeacherParentReport({
      id: "r1",
      studentId: "u1",
      classId: "c1",
      rangeStart: "2026-08-17",
      rangeEnd: "2026-09-13",
      facts: { studentName: "王思远", className: "高一（3）班", teacherName: "李老师" },
      hidden: { moments: [], keywords: [] },
      draft: null,
      body: null,
      sections: ["overview", "next"],
      hiddenMentions: {},
      createdAt: "2026-09-14T01:00:00Z",
    });
    expect(r.hasDraft).toBe(false);
    expect(r.view.studentName).toBe("王思远");
    expect(r.view.teacherName).toBe("李老师");
    expect(r.view.createdAt).toBe("2026-09-14T01:00:00Z");
    expect(r.view.body).toEqual({});
    expect(r.hidden).toEqual({ moments: [], keywords: [] });
    expect(r.hiddenMentions).toEqual({});
  });

  it("reads hidden items and a draft", () => {
    const r = normalizeTeacherParentReport({
      id: "r1",
      draft: { overview: "x" },
      hidden: { moments: ["雨水不是废水", 3], keywords: ["天气"] },
      hiddenMentions: { overview: ["雨水不是废水"] },
    });
    expect(r.hasDraft).toBe(true);
    expect(r.hidden).toEqual({ moments: ["雨水不是废水"], keywords: ["天气"] });
    expect(r.hiddenMentions).toEqual({ overview: ["雨水不是废水"] });
  });

  // An older server without the field, or a null list, must not crash the toggles.
  it("reads a missing or null hidden as empty lists", () => {
    expect(normalizeTeacherParentReport({}).hidden).toEqual({ moments: [], keywords: [] });
    expect(normalizeTeacherParentReport({ hidden: { moments: null } }).hidden).toEqual({ moments: [], keywords: [] });
  });

  it("reads a summary row", () => {
    const row = normalizeParentReportSummary({ id: "r1", studentName: "王思远", createdAt: "2026-09-14T01:00:00Z" });
    expect(row).toEqual({
      id: "r1",
      studentId: "",
      studentName: "王思远",
      rangeStart: "",
      rangeEnd: "",
      createdAt: "2026-09-14T01:00:00Z",
    });
  });
});

// A key present must mean "this section still quotes something hidden": the
// export block reads exactly that.
describe("normalizeHiddenMentions", () => {
  it("drops empty and non-list values", () =>
    expect(normalizeHiddenMentions({ overview: ["a", 1], next: [], reading: "a", writing: null })).toEqual({
      overview: ["a"],
    }));
  it("reads null as no mentions", () => expect(normalizeHiddenMentions(null)).toEqual({}));
});

describe("normalizeParentReport", () => {
  it("reads facts, sections and body", () => {
    const r = normalizeParentReport({
      studentName: "林知遥",
      className: "高一（3）班",
      teacherName: "李老师",
      rangeStart: "2026-08-17",
      rangeEnd: "2026-09-13",
      facts: {
        activeDays: 3,
        minutes: -1,
        readings: [{ kind: "reading", title: "一场雨", finishedAt: "2026-09-09" }],
        moments: [{ quote: "雨落在屋檐上像敲鼓", itemTitle: "一场雨" }],
        keywords: [{ text: "天气", field: "science", fieldLabel: "科学" }],
      },
      sections: ["overview", "reading", "next"],
      body: { overview: "整体", reading: "阅读" },
    });
    expect(r.studentName).toBe("林知遥");
    expect(r.facts.minutes).toBe(-1);
    expect(r.facts.readings).toHaveLength(1);
    expect(r.facts.writings).toEqual([]);
    expect(r.facts.moments[0]?.itemTitle).toBe("一场雨");
    expect(r.sections).toEqual(["overview", "reading", "next"]);
    expect(r.body).toEqual({ overview: "整体", reading: "阅读" });
  });

  // Go marshals a nil slice as null and a nil map as null; a render must
  // never see either.
  it("turns nulls into empty values", () => {
    const r = normalizeParentReport({ facts: { readings: null, keywords: null }, sections: null, body: null });
    expect(r.facts.readings).toEqual([]);
    expect(r.facts.keywords).toEqual([]);
    expect(r.sections).toEqual([]);
    expect(r.body).toEqual({});
    expect(r.createdAt).toBe("");
  });

  it("drops non-string body values", () =>
    expect(normalizeParentReport({ body: { overview: "a", reading: 3 } }).body).toEqual({ overview: "a" }));

  // Minutes -1 is a real value (no record), so a missing number must not
  // silently become it, nor become a fake 0 minutes either way.
  it("keeps a missing minutes as -1 (no record)", () => expect(normalizeParentReport({ facts: {} }).facts.minutes).toBe(-1));
});
