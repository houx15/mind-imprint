import { describe, expect, it } from "vitest";
import { ApiError } from "../api/client";
import { emptyFacts, type TeacherParentReport } from "../api/parentReports";
import { posterReportFrom } from "./parentReportLogic";
import {
  draftErrorText,
  EXPORT_BLOCKED_TEXT,
  exportBlockedReason,
  generateErrorPlacement,
  hiddenMentionText,
  isBodyBlank,
  saveErrorText,
  sectionKeysToLabels,
  showsNoDraftHint,
} from "./parentReportLogic";

// Fix round 1: the export once checked one snapshot and rasterized another,
// which could put a hidden 金句 into the PNG. The picture is built from the
// server snapshot alone.
describe("posterReportFrom", () => {
  const snapshot: TeacherParentReport = {
    id: "r1",
    studentId: "u1",
    classId: "c1",
    hasDraft: true,
    createdAt: "2026-09-14T02:00:00Z",
    hidden: { moments: ["雨落在屋檐上像敲鼓"], keywords: ["记叙文"] },
    hiddenMentions: {},
    view: {
      studentName: "王思远",
      className: "高一（3）班",
      teacherName: "李老师",
      rangeStart: "2026-08-17",
      rangeEnd: "2026-09-13",
      createdAt: "2026-09-14T02:00:00Z",
      facts: {
        ...emptyFacts(),
        moments: [
          { quote: "雨落在屋檐上像敲鼓", itemTitle: "一场雨" },
          { quote: "这两个数要分开看", itemTitle: "中国是否让地球变得更可持续？" },
        ],
        keywords: [
          { text: "天气", field: "science", fieldLabel: "科学" },
          { text: "记叙文", field: "humanities", fieldLabel: "人文" },
        ],
      },
      // Every keyword but one is visible, yet `interests` is not listed: its
      // stored text must stay out.
      sections: ["overview", "next"],
      body: { overview: "服务端存的概述", interests: "不在 sections 里的兴趣", next: "建议" },
    },
  };

  it("keeps only the snapshot's sections", () => {
    const poster = posterReportFrom(snapshot);
    expect(poster.body).toEqual({ overview: "服务端存的概述", next: "建议" });
    expect(poster.sections).toEqual(["overview", "next"]);
  });

  it("drops hidden moments and keywords", () => {
    const poster = posterReportFrom(snapshot);
    expect(poster.facts.moments.map((m) => m.quote)).toEqual(["这两个数要分开看"]);
    expect(poster.facts.keywords.map((k) => k.text)).toEqual(["天气"]);
  });

  it("takes the body from the snapshot, whatever local text the caller has", () => {
    const local = { overview: "导出时刚打的字「雨落在屋檐上像敲鼓」", next: "建议" };
    const poster = posterReportFrom({ ...snapshot, view: { ...snapshot.view } });
    expect(poster.body.overview).toBe("服务端存的概述");
    expect(poster.body.overview).not.toBe(local.overview);
  });

  it("does not modify the snapshot", () => {
    posterReportFrom(snapshot);
    expect(snapshot.view.facts.moments).toHaveLength(2);
    expect(Object.keys(snapshot.view.body)).toEqual(["overview", "interests", "next"]);
  });
});

// Ruling 18 D3: the hint covers a reload after a failed generate, when the
// in-memory banner is gone; it never doubles the banner.
describe("showsNoDraftHint", () => {
  const blank = { overview: "", next: "  " };
  it("shows with no model draft and a blank body", () => {
    expect(showsNoDraftHint(false, blank, null)).toBe(true);
  });
  it("hides once any section has text, a draft exists, or a banner shows", () => {
    expect(showsNoDraftHint(false, { overview: "a", next: "" }, null)).toBe(false);
    expect(showsNoDraftHint(true, blank, null)).toBe(false);
    expect(showsNoDraftHint(false, blank, "草稿生成失败：x")).toBe(false);
  });
});

// Follow-ups Ruling 2: a picture that still quotes a hidden item must not be made.
describe("exportBlockedReason", () => {
  it("allows the export with no mentions", () => expect(exportBlockedReason({ hiddenMentions: {} })).toBeNull());
  it("blocks while any section still quotes a hidden item", () =>
    expect(exportBlockedReason({ hiddenMentions: { overview: ["雨水不是废水"] } })).toBe(EXPORT_BLOCKED_TEXT));
  it("ignores an empty list", () => expect(exportBlockedReason({ hiddenMentions: { overview: [] } })).toBeNull());
  it("reads as a failure line", () => expect(EXPORT_BLOCKED_TEXT).toBe("导出失败：正文仍引用已隐藏的内容，请先修改"));
});

describe("hiddenMentionText", () => {
  it("joins the quoted items with 、", () =>
    expect(hiddenMentionText(["雨水不是废水", "天气"])).toBe("这一段仍引用了已隐藏的内容：雨水不是废水、天气，请修改"));
});

// Ruling 13: a draftError can already be a whole failure line from the server
// (「保存草稿失败：请求编号 …」). Prefixing it again reads 「草稿生成失败：保存草稿失败：…」.
describe("draftErrorText", () => {
  it("prefixes a raw composer error once", () => {
    expect(draftErrorText("agent: lite parent report: rejected after 2 attempts: digits")).toBe(
      "草稿生成失败：agent: lite parent report: rejected after 2 attempts: digits",
    );
    expect(draftErrorText("该学生已不在本班")).toBe("草稿生成失败：该学生已不在本班");
  });

  it("shows a message that already carries a failure verb as it is", () => {
    expect(draftErrorText("保存草稿失败：请求编号 abc123")).toBe("保存草稿失败：请求编号 abc123");
    expect(draftErrorText("  草稿生成失败：x ")).toBe("草稿生成失败：x");
  });

  it("returns nothing for an absent or blank error", () => {
    expect(draftErrorText(null)).toBeNull();
    expect(draftErrorText("   ")).toBeNull();
  });
});

// The server's PATCH messages end with the raw section key; the teacher must
// never see `overview`.
describe("sectionKeysToLabels", () => {
  it("replaces a trailing known key with its label", () => {
    expect(sectionKeysToLabels("每部分不超过 2000 字：overview")).toBe("每部分不超过 2000 字：总体概述");
    expect(sectionKeysToLabels("报告不包含该部分：projects")).toBe("报告不包含该部分：项目");
    expect(sectionKeysToLabels("报告不包含该部分: next")).toBe("报告不包含该部分: 下一步建议");
  });

  it("leaves other messages and unknown keys alone", () => {
    expect(sectionKeysToLabels("请提供报告内容")).toBe("请提供报告内容");
    expect(sectionKeysToLabels("报告不包含该部分：summary")).toBe("报告不包含该部分：summary");
    // A key inside a sentence is not a trailing key.
    expect(sectionKeysToLabels("reading 失败：网络错误")).toBe("reading 失败：网络错误");
  });

  it("builds the autosave failure line", () => {
    expect(saveErrorText(new ApiError("section_too_long", "每部分不超过 2000 字：reading", 400))).toBe(
      "保存失败：每部分不超过 2000 字：阅读",
    );
    expect(saveErrorText(new Error("Failed to fetch"))).toBe("保存失败：Failed to fetch");
  });
});

describe("generateErrorPlacement", () => {
  it("puts range errors under the dates and everything else by the buttons", () => {
    expect(generateErrorPlacement(new ApiError("range_before_start", "该时间段早于学生加入班级的时间", 400))).toBe("range");
    expect(generateErrorPlacement(new ApiError("invalid_range", "请选择有效的日期范围", 400))).toBe("range");
    expect(generateErrorPlacement(new ApiError("not_entitled", "当前没有可用额度", 403))).toBe("form");
    expect(generateErrorPlacement(new Error("Failed to fetch"))).toBe("form");
  });
});

describe("isBodyBlank", () => {
  it("is blank only when every value is whitespace", () => {
    expect(isBodyBlank({})).toBe(true);
    expect(isBodyBlank({ overview: "", next: " \n " })).toBe(true);
    expect(isBodyBlank({ overview: "", next: "建议" })).toBe(false);
  });
});
