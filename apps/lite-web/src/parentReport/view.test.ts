import { describe, expect, it } from "vitest";
import { emptyFacts, type ParentReportFacts } from "../api/parentReports";
import { keywordGroups, minutesParts, statTiles, visibleSections } from "./view";

function facts(over: Partial<ParentReportFacts> = {}): ParentReportFacts {
  return { ...emptyFacts(), ...over };
}

describe("visibleSections", () => {
  // A published body can still have an empty section: PATCH clears one with
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
