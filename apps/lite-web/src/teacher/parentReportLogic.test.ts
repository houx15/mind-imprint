import { describe, expect, it } from "vitest";
import { ApiError } from "../api/client";
import {
  draftErrorText,
  generateErrorPlacement,
  isBodyBlank,
  linkStateLabel,
  saveErrorText,
  sectionKeysToLabels,
  shareUrl,
  showsNoDraftHint,
  statusLabel,
} from "./parentReportLogic";

// Ruling 18 D3: the hint covers a reload after a failed generate, when the
// in-memory banner is gone; it never doubles the banner.
describe("showsNoDraftHint", () => {
  const blank = { overview: "", next: "  " };
  it("shows for a draft with no model draft and a blank body", () => {
    expect(showsNoDraftHint("draft", false, blank, null)).toBe(true);
  });
  it("hides once any section has text, a draft exists, a banner shows, or the report is published", () => {
    expect(showsNoDraftHint("draft", false, { overview: "a", next: "" }, null)).toBe(false);
    expect(showsNoDraftHint("draft", true, blank, null)).toBe(false);
    expect(showsNoDraftHint("draft", false, blank, "草稿生成失败：x")).toBe(false);
    expect(showsNoDraftHint("published", false, blank, null)).toBe(false);
  });
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

describe("labels", () => {
  it("names status and link state", () => {
    expect(statusLabel("draft")).toBe("草稿");
    expect(statusLabel("published")).toBe("已发布");
    expect(linkStateLabel("draft", false)).toBe("—");
    expect(linkStateLabel("published", true)).toBe("已开启");
    expect(linkStateLabel("published", false)).toBe("已撤销");
  });

  it("builds the share link from the app origin", () => {
    expect(shareUrl("https://mind-web.uni-robot.cn", "abc")).toBe("https://mind-web.uni-robot.cn/r/abc");
    expect(shareUrl("http://localhost:5174/", "a b")).toBe("http://localhost:5174/r/a%20b");
  });
});
