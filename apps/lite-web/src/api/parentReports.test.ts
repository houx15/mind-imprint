import { describe, expect, it } from "vitest";
import { normalizeParentReport, normalizeParentReportSummary, normalizeTeacherParentReport } from "./parentReports";

// The teacher DTO has names only inside facts, a null draft/body until one is
// stored, and a null token for a draft or a revoked link.
describe("normalizeTeacherParentReport", () => {
  it("reads a fresh report whose draft failed", () => {
    const r = normalizeTeacherParentReport({
      id: "r1",
      studentId: "u1",
      classId: "c1",
      rangeStart: "2026-08-17",
      rangeEnd: "2026-09-13",
      status: "draft",
      facts: { studentName: "王思远", className: "高一（3）班", teacherName: "李老师" },
      draft: null,
      body: null,
      sections: ["overview", "next"],
      shareToken: null,
      publishedAt: null,
      createdAt: "2026-09-14T01:00:00Z",
    });
    expect(r.hasDraft).toBe(false);
    expect(r.shareToken).toBeNull();
    expect(r.view.studentName).toBe("王思远");
    expect(r.view.teacherName).toBe("李老师");
    expect(r.view.body).toEqual({});
    expect(r.view.publishedAt).toBeNull();
  });

  it("reads a published report with its token, and an unknown status as draft", () => {
    const r = normalizeTeacherParentReport({ id: "r1", status: "published", draft: { overview: "x" }, shareToken: "tok" });
    expect(r.status).toBe("published");
    expect(r.hasDraft).toBe(true);
    expect(r.shareToken).toBe("tok");
    expect(normalizeTeacherParentReport({ status: "weird" }).status).toBe("draft");
  });

  it("reads a summary row", () => {
    const row = normalizeParentReportSummary({ id: "r1", studentName: "王思远", status: "published", shared: true });
    expect(row.shared).toBe(true);
    expect(row.publishedAt).toBeNull();
    expect(normalizeParentReportSummary({ shared: "yes" }).shared).toBe(false);
  });
});

describe("normalizeParentReport", () => {
  it("reads the public DTO, which has no id", () => {
    const r = normalizeParentReport({
      studentName: "林知遥",
      className: "高一（3）班",
      teacherName: "李老师",
      rangeStart: "2026-08-17",
      rangeEnd: "2026-09-13",
      publishedAt: "2026-09-14T02:00:00Z",
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
    expect(r.id).toBeNull();
    expect(r.studentName).toBe("林知遥");
    expect(r.facts.minutes).toBe(-1);
    expect(r.facts.readings).toHaveLength(1);
    expect(r.facts.writings).toEqual([]);
    expect(r.facts.moments[0]?.itemTitle).toBe("一场雨");
    expect(r.sections).toEqual(["overview", "reading", "next"]);
    expect(r.body).toEqual({ overview: "整体", reading: "阅读" });
  });

  it("reads the student DTO's id", () => expect(normalizeParentReport({ id: "p1" }).id).toBe("p1"));

  // Go marshals a nil slice as null and a nil map as null; a render must
  // never see either.
  it("turns nulls into empty values", () => {
    const r = normalizeParentReport({ facts: { readings: null, keywords: null }, sections: null, body: null });
    expect(r.facts.readings).toEqual([]);
    expect(r.facts.keywords).toEqual([]);
    expect(r.sections).toEqual([]);
    expect(r.body).toEqual({});
    expect(r.publishedAt).toBeNull();
  });

  it("drops non-string body values", () =>
    expect(normalizeParentReport({ body: { overview: "a", reading: 3 } }).body).toEqual({ overview: "a" }));

  // Minutes -1 is a real value (no record), so a missing number must not
  // silently become it, nor become a fake 0 minutes either way.
  it("keeps a missing minutes as -1 (no record)", () => expect(normalizeParentReport({ facts: {} }).facts.minutes).toBe(-1));
});
