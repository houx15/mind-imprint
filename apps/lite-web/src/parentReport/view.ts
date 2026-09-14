// parentReport/view.ts — the pure decisions behind `ParentReportView` and the
// poster: which sections show, which stat tiles show, how keywords group.
// Both renderings read these, so the page and the exported picture cannot
// disagree about what is in the report.

import type { ParentReportFacts, ParentReportKeyword } from "../api/parentReports";
import { formatMinutes } from "../teacher/format";

/** Same labels as `liteparent.SectionLabels`. */
export const SECTION_LABELS: Record<string, string> = {
  overview: "总体概述",
  reading: "阅读",
  writing: "写作",
  projects: "项目",
  interests: "兴趣",
  next: "下一步建议",
};

export interface VisibleSection {
  key: string;
  label: string;
  text: string;
}

/**
 * The sections to render, in the server's order. A section whose body is
 * blank after trim is dropped: publishing only refuses a report where EVERY
 * section is empty, and PATCH can clear a single one with "". A key with no
 * known label is dropped too.
 */
export function visibleSections(
  sections: readonly string[],
  body: Readonly<Record<string, string>> | null | undefined,
): VisibleSection[] {
  const out: VisibleSection[] = [];
  for (const key of sections) {
    const label = SECTION_LABELS[key];
    const text = (body?.[key] ?? "").trim();
    if (label && text) out.push({ key, label, text });
  }
  return out;
}

/** One number in a tile: an optional word before it, the numeral, an
 * optional unit after it. */
export interface TilePart {
  lead?: string;
  n: string;
  unit?: string;
}

export interface StatTile {
  key: string;
  label: string;
  parts: TilePart[];
}

/** `formatMinutes` split into numeral/unit pairs, so each unit can be set
 * smaller than its numeral. -1 (no record) is a single `—`. */
export function minutesParts(minutes: number): TilePart[] {
  const text = formatMinutes(minutes);
  const parts: TilePart[] = [];
  for (const m of text.matchAll(/(\d+)\s*(小时|分钟)/g)) {
    parts.push({ n: m[1] ?? "", unit: m[2] ?? "" });
  }
  return parts.length > 0 ? parts : [{ n: text }];
}

/**
 * Stat tiles from the facts. A tile whose source is empty is left out (a
 * zero is absence, not a fact worth stating), with one exception:
 * 学习时长 of -1 means the range has no time record, and shows `—`.
 */
export function statTiles(f: ParentReportFacts): StatTile[] {
  const tiles: StatTile[] = [];
  if (f.activeDays > 0) tiles.push({ key: "activeDays", label: "活跃天数", parts: [{ n: `${f.activeDays}`, unit: "天" }] });
  if (f.minutes !== 0) tiles.push({ key: "minutes", label: "学习时长", parts: minutesParts(f.minutes) });
  if (f.turns > 0) tiles.push({ key: "turns", label: "对话轮次", parts: [{ n: `${f.turns}`, unit: "轮" }] });
  if (f.readings.length > 0)
    tiles.push({ key: "readings", label: "完成阅读", parts: [{ n: `${f.readings.length}`, unit: "篇" }] });
  if (f.writings.length > 0)
    tiles.push({ key: "writings", label: "完成写作", parts: [{ n: `${f.writings.length}`, unit: "篇" }] });
  if (f.projects.length > 0)
    tiles.push({ key: "projects", label: "完成项目", parts: [{ n: `${f.projects.length}`, unit: "个" }] });
  if (f.assignmentsTotal > 0 || f.assignmentsOnTime + f.assignmentsLate + f.assignmentsMissed > 0) {
    tiles.push({
      key: "assignments",
      label: "作业",
      parts: [
        { lead: "按时", n: `${f.assignmentsOnTime}` },
        { lead: "逾期完成", n: `${f.assignmentsLate}` },
        { lead: "未完成", n: `${f.assignmentsMissed}` },
      ],
    });
  }
  return tiles;
}

export interface KeywordGroup {
  label: string;
  words: string[];
}

/** Keywords grouped by field label, groups in first-seen order. A keyword
 * with no field label goes under 其他. */
export function keywordGroups(keywords: readonly ParentReportKeyword[]): KeywordGroup[] {
  const groups: KeywordGroup[] = [];
  for (const k of keywords) {
    if (!k.text.trim()) continue;
    const label = k.fieldLabel.trim() || "其他";
    let group = groups.find((g) => g.label === label);
    if (!group) {
      group = { label, words: [] };
      groups.push(group);
    }
    group.words.push(k.text);
  }
  return groups;
}

/** The finished-work lists shown under the sections, empty kinds left out. */
export function finishedLists(f: ParentReportFacts): { key: string; label: string; items: ParentReportFacts["readings"] }[] {
  return [
    { key: "readings", label: "完成的阅读", items: f.readings },
    { key: "writings", label: "完成的写作", items: f.writings },
    { key: "projects", label: "完成的项目", items: f.projects },
  ].filter((g) => g.items.length > 0);
}
