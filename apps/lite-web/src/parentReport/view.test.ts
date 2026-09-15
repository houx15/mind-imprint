import { describe, expect, it } from "vitest";
import { emptyFacts, type ParentReportFacts } from "../api/parentReports";
import {
  bylineText,
  keywordGroups,
  minutesParts,
  posterFileName,
  statTiles,
  toggleHidden,
  visibleFacts,
  visibleSections,
} from "./view";

function facts(over: Partial<ParentReportFacts> = {}): ParentReportFacts {
  return { ...emptyFacts(), ...over };
}

// The preview and the exported picture read these facts. A hidden 金句 that
// still shows up there is sent to parents.
describe("visibleFacts", () => {
  const full = facts({
    moments: [
      { quote: "雨水不是废水", itemTitle: "一场雨" },
      { quote: "碳排放总量第一，不等于人均排放第一", itemTitle: "中国是否让地球变得更可持续？" },
    ],
    keywords: [
      { text: "天气", field: "science", fieldLabel: "科学" },
      { text: "碳排放", field: "science", fieldLabel: "科学" },
    ],
  });

  it("keeps everything when nothing is hidden", () =>
    expect(visibleFacts(full, { moments: [], keywords: [] })).toEqual(full));

  it("drops a moment by exact quote and a keyword by exact text", () => {
    const v = visibleFacts(full, { moments: ["雨水不是废水"], keywords: ["碳排放"] });
    expect(v.moments.map((m) => m.quote)).toEqual(["碳排放总量第一，不等于人均排放第一"]);
    expect(v.keywords.map((k) => k.text)).toEqual(["天气"]);
  });

  it("ignores substrings, unknown entries and the wrong list", () => {
    const v = visibleFacts(full, { moments: ["雨水", "天气"], keywords: ["雨水不是废水", "碳"] });
    expect(v.moments).toHaveLength(2);
    expect(v.keywords).toHaveLength(2);
  });

  it("does not modify its input", () => {
    visibleFacts(full, { moments: ["雨水不是废水"], keywords: ["天气"] });
    expect(full.moments).toHaveLength(2);
    expect(full.keywords).toHaveLength(2);
  });
});

// A toggle sends the WHOLE set (PATCH replaces it), so the other list must survive.
describe("toggleHidden", () => {
  const hidden = { moments: ["a"], keywords: ["k"] };
  it("hides an item and keeps the other list", () =>
    expect(toggleHidden(hidden, "moments", "b")).toEqual({ moments: ["a", "b"], keywords: ["k"] }));
  it("shows a hidden item again", () =>
    expect(toggleHidden(hidden, "keywords", "k")).toEqual({ moments: ["a"], keywords: [] }));
  it("does not modify its input", () => {
    toggleHidden(hidden, "moments", "a");
    expect(hidden).toEqual({ moments: ["a"], keywords: ["k"] });
  });
});

describe("bylineText", () => {
  // Beijing date: 16:30Z on the 13th is already the 14th.
  it("names the author and the Beijing creation date", () =>
    expect(bylineText("李老师", "2026-09-13T16:30:00Z")).toBe("由 李老师 撰写 · 9月14日"));
  it("leaves the date out when it cannot be read", () => expect(bylineText("李老师", "")).toBe("由 李老师 撰写"));
});

describe("posterFileName", () => {
  it("uses the student's name", () => expect(posterFileName(" 王思远 ")).toBe("学习报告-王思远.png"));
  it("falls back without a name", () => expect(posterFileName("  ")).toBe("学习报告.png"));
});

describe("visibleSections", () => {
  // A body can still have an empty section: PATCH clears one with
  // "". A heading with nothing under it must never render.
  it("drops sections whose body is blank after trim", () =>
    expect(
      visibleSections(["overview", "reading", "next"], { overview: "  整体很好。\n", reading: "  \n ", next: "" }),
    ).toEqual([{ key: "overview", label: "总体概述", text: "整体很好。" }]));
  it("keeps the server's section order, not the body's key order", () =>
    expect(visibleSections(["next", "overview"], { overview: "a", next: "b" }).map((s) => s.key)).toEqual([
      "next",
      "overview",
    ]));
  it("skips a key with no label", () => expect(visibleSections(["mystery"], { mystery: "x" })).toEqual([]));
  it("reads a missing body as empty", () => expect(visibleSections(["overview"], null)).toEqual([]));
});

describe("minutesParts", () => {
  it("under an hour", () => expect(minutesParts(45)).toEqual([{ n: "45", unit: "分钟" }]));
  it("hours and minutes", () =>
    expect(minutesParts(80)).toEqual([
      { n: "1", unit: "小时" },
      { n: "20", unit: "分钟" },
    ]));
  it("whole hours", () => expect(minutesParts(120)).toEqual([{ n: "2", unit: "小时" }]));
  it("no record", () => expect(minutesParts(-1)).toEqual([{ n: "—" }]));
});

describe("statTiles", () => {
  it("shows only tiles whose source has something", () =>
    expect(statTiles(facts({ activeDays: 3, turns: 0, minutes: 0 })).map((t) => t.key)).toEqual(["activeDays"]));

  // -1 means the range has no time buckets at all: 学习时长 shows —, it is not
  // hidden like a real 0.
  it("keeps 学习时长 as — when there is no record", () =>
    expect(statTiles(facts({ minutes: -1 }))).toEqual([{ key: "minutes", label: "学习时长", parts: [{ n: "—" }] }]));

  it("builds every tile from full facts", () => {
    const item = { kind: "reading", title: "t", finishedAt: "2026-09-01" };
    const tiles = statTiles(
      facts({
        activeDays: 12,
        minutes: 95,
        turns: 40,
        readings: [item, item],
        writings: [item],
        projects: [item],
        assignmentsTotal: 5,
        assignmentsOnTime: 3,
        assignmentsLate: 1,
        assignmentsMissed: 1,
      }),
    );
    expect(tiles).toEqual([
      { key: "activeDays", label: "活跃天数", parts: [{ n: "12", unit: "天" }] },
      { key: "minutes", label: "学习时长", parts: [{ n: "1", unit: "小时" }, { n: "35", unit: "分钟" }] },
      { key: "turns", label: "对话轮次", parts: [{ n: "40", unit: "轮" }] },
      { key: "readings", label: "完成阅读", parts: [{ n: "2", unit: "篇" }] },
      { key: "writings", label: "完成写作", parts: [{ n: "1", unit: "篇" }] },
      { key: "projects", label: "完成项目", parts: [{ n: "1", unit: "个" }] },
      {
        key: "assignments",
        label: "作业",
        parts: [
          { lead: "按时", n: "3" },
          { lead: "逾期完成", n: "1" },
          { lead: "逾期未完成", n: "1" },
        ],
      },
    ]);
  });
});

describe("keywordGroups", () => {
  it("groups by field label in first-seen order", () =>
    expect(
      keywordGroups([
        { text: "天气", field: "science", fieldLabel: "科学" },
        { text: "诗", field: "arts", fieldLabel: "艺术" },
        { text: "云", field: "science", fieldLabel: "科学" },
        { text: "无名", field: "", fieldLabel: "" },
      ]),
    ).toEqual([
      { label: "科学", words: ["天气", "云"] },
      { label: "艺术", words: ["诗"] },
      { label: "其他", words: ["无名"] },
    ]));
});
