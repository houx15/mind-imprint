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
  keptSectionText,
  planReportPatch,
  reportArtifactPayload,
  reportPatchBody,
  runeCount,
  saveErrorText,
  SECTION_MAX_RUNES,
  sectionKeysToLabels,
  showsNoDraftHint,
  studentLeftReason,
} from "./parentReportLogic";
import { createSerialQueue } from "./serialQueue";

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

describe("reportArtifactPayload", () => {
  it("sends only the listed sections, in their current text, each cut to the limit", () => {
    const long = "字".repeat(SECTION_MAX_RUNES + 5);
    const payload = reportArtifactPayload({ overview: "未保存的输入", interests: "隐藏了", next: long }, [
      "overview",
      "next",
      "reading",
    ]);
    expect(Object.keys(payload.body)).toEqual(["overview", "next", "reading"]);
    expect(payload.body.overview).toBe("未保存的输入");
    expect(runeCount(payload.body.next!)).toBe(SECTION_MAX_RUNES);
    expect(payload.body.reading).toBe("");
  });

  it("cuts by code point, not by UTF-16 unit", () => {
    const emoji = "😀".repeat(SECTION_MAX_RUNES + 1);
    const cut = reportArtifactPayload({ overview: emoji }, ["overview"]).body.overview!;
    expect(runeCount(cut)).toBe(SECTION_MAX_RUNES);
    expect(cut.length).toBe(SECTION_MAX_RUNES * 2);
  });
});

describe("reportPatchBody", () => {
  it("reads the section texts and drops anything that is not a string", () => {
    expect(reportPatchBody({ body: { overview: "新的概述", next: 3, reading: null } })).toEqual({ overview: "新的概述" });
  });
  it("is empty for an empty or malformed patch", () => {
    expect(reportPatchBody({})).toEqual({});
    expect(reportPatchBody({ body: "overview" })).toEqual({});
    expect(reportPatchBody({ body: ["x"] })).toEqual({});
    expect(reportPatchBody({ body: null })).toEqual({});
  });
});

describe("planReportPatch", () => {
  const snapshot = { overview: "原概述", reading: "原阅读", next: "原建议" };

  it("writes a patched section she did not touch", () => {
    const plan = planReportPatch(snapshot, snapshot, { overview: "新概述" }, ["overview", "reading", "next"]);
    expect(plan.write).toEqual(["overview"]);
    expect(plan.kept).toEqual([]);
    expect(plan.texts).toEqual({ ...snapshot, overview: "新概述" });
  });

  it("keeps a section she changed while the turn ran, and still writes the others", () => {
    const live = { ...snapshot, reading: "她改过的阅读" };
    const plan = planReportPatch(live, snapshot, { reading: "模型的阅读", next: "模型的建议" }, [
      "overview",
      "reading",
      "next",
    ]);
    expect(plan.kept).toEqual(["reading"]);
    expect(plan.write).toEqual(["next"]);
    expect(plan.texts.reading).toBe("她改过的阅读");
    expect(plan.texts.next).toBe("模型的建议");
  });

  it("leaves her edit in a section the patch does not name, without reporting it", () => {
    const live = { ...snapshot, overview: "她在打字" };
    const plan = planReportPatch(live, snapshot, { next: "模型的建议" }, ["overview", "next"]);
    expect(plan.kept).toEqual([]);
    expect(plan.write).toEqual(["next"]);
    expect(plan.texts.overview).toBe("她在打字");
  });

  it("drops a section the report no longer shows, and one the editor has no entry for", () => {
    const plan = planReportPatch(snapshot, snapshot, { next: "新建议", interests: "兴趣" }, ["overview", "reading"]);
    expect(plan.write).toEqual([]);
    expect(plan.kept).toEqual([]);
    expect(plan.texts).toEqual(snapshot);
    const unknown = planReportPatch(snapshot, snapshot, { projects: "项目" }, ["overview", "projects"]);
    expect(unknown.write).toEqual([]);
    expect("projects" in unknown.texts).toBe(false);
  });

  it("writes nothing for a patched text equal to what is there", () => {
    const plan = planReportPatch(snapshot, snapshot, { overview: "原概述" }, ["overview"]);
    expect(plan.write).toEqual([]);
    expect(plan.kept).toEqual([]);
  });

  it("lists writes in report order and does not modify its inputs", () => {
    const live = { ...snapshot };
    const plan = planReportPatch(live, snapshot, { next: "b", overview: "a" }, ["overview", "reading", "next"]);
    expect(plan.write).toEqual(["overview", "next"]);
    expect(live).toEqual(snapshot);
  });
});

describe("keptSectionText", () => {
  it("names the section by its heading, never its key", () => {
    expect(keptSectionText("next")).toBe("下一步建议 已保留你的修改");
    expect(keptSectionText("unknown_key")).toBe("该段落 已保留你的修改");
  });
});

describe("studentLeftReason", () => {
  it("is the server's words for student_left, without a verb prefix", () => {
    expect(studentLeftReason(new ApiError("student_left", "该学生已不在本班", 409))).toBe("该学生已不在本班");
    expect(studentLeftReason(new ApiError("student_left", "对话失败：该学生已不在本班", 409))).toBe("该学生已不在本班");
  });
  it("is null for any other failure", () => {
    expect(studentLeftReason(new ApiError("ai_dialogue_failed", "对话失败：超时", 502))).toBeNull();
    expect(studentLeftReason(new Error("Failed to fetch"))).toBeNull();
  });
});

// The editor applies a plan by replacing its text ref, then enqueuing the
// section save for each written key on its one queue. The save reads the ref
// when it RUNS. `editor` below models that save against a server whose
// responses the test releases one by one, to pin the orderings the editor
// relies on.
describe("patch writes through the editor's one queue", () => {
  function editor(initial: Record<string, string>) {
    const queue = createSerialQueue();
    const state = { live: { ...initial }, saved: { ...initial } };
    const sent: [string, string][] = [];
    const release: (() => void)[] = [];
    function store(key: string): Promise<boolean> {
      const text = state.live[key] ?? "";
      if (text === state.saved[key]) return Promise.resolve(true);
      sent.push([key, text]);
      return new Promise<boolean>((resolve) => {
        release.push(() => {
          state.saved[key] = text;
          resolve(true);
        });
      });
    }
    const save = (key: string) => queue.enqueue(() => store(key));
    function applyPlan(snapshot: Record<string, string>, patch: Record<string, string>, sections: string[]) {
      const plan = planReportPatch(state.live, snapshot, patch, sections);
      state.live = plan.texts;
      return Promise.all(plan.write.map(save));
    }
    return { state, sent, release, save, applyPlan, queue };
  }
  const flush = () => new Promise<void>((r) => setTimeout(r, 0));

  it("saves a patch for a section after that section's save already in flight", async () => {
    const e = editor({ overview: "原概述" });
    e.state.live = { overview: "她的输入" };
    const hers = e.save("overview");
    await flush();
    // The turn was sent with her typing in it, and she has not typed since.
    const patched = e.applyPlan({ ...e.state.live }, { overview: "模型的概述" }, ["overview"]);
    await flush();
    expect(e.sent).toEqual([["overview", "她的输入"]]);
    e.release.shift()!();
    await hers;
    await flush();
    expect(e.sent).toEqual([
      ["overview", "她的输入"],
      ["overview", "模型的概述"],
    ]);
    e.release.shift()!();
    await patched;
    expect(e.state.saved.overview).toBe("模型的概述");
  });

  it("sends her newer text when she edits a patched section before its save runs", async () => {
    const e = editor({ overview: "原概述", next: "原建议" });
    e.state.live = { ...e.state.live, overview: "她的输入" };
    const hers = e.save("overview");
    await flush();
    const patched = e.applyPlan({ ...e.state.live }, { next: "模型的建议" }, ["overview", "next"]);
    expect(e.state.live.next).toBe("模型的建议");
    e.state.live = { ...e.state.live, next: "她又改了建议" };
    e.release.shift()!();
    await hers;
    await flush();
    expect(e.sent).toEqual([
      ["overview", "她的输入"],
      ["next", "她又改了建议"],
    ]);
    e.release.shift()!();
    await patched;
    await e.queue.idle();
    expect(e.state.saved).toEqual({ overview: "她的输入", next: "她又改了建议" });
  });
});
