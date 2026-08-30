import type { ReportStat } from "@lite/api/reports";

/**
 * statLabels — the wording of a stat tile, resolved on the CLIENT from
 * `stat.key`.
 *
 * ## Why this exists rather than just fixing the server strings
 *
 * A report is GENERATED ONCE and STORED as a JSON blob (`atom_report`), then
 * re-served verbatim forever. So a label change on the server reaches only
 * readings finished after the deploy: every reading a student has already
 * finished keeps showing the old wording, and there is no natural moment at
 * which it would ever be regenerated. That is exactly what happened when
 * 「和印记聊了」/「读完」 were renamed and the live report still read
 * 「和印记聊了 1 轮 / 读完 1 步」.
 *
 * `key` is the stable machine name (see `reportStat` in atom_report.go) and
 * the value is a fact; the label is presentation. Resolving it here means a
 * wording change lands on every stored report at once, with no migration and
 * no regeneration.
 *
 * An unknown key falls through to whatever the server sent, so a stat added
 * server-side still renders — with server wording — before this map knows
 * about it.
 *
 * `unit: ""` deliberately CLEARS a stored unit: 「AI 对话轮数」 already names
 * the quantity, and a stored `unit: "轮"` would render 「1 轮 / AI 对话轮数」.
 */
const STAT_DISPLAY: Record<string, { label: string; unit?: string }> = {
  // reading
  focusMinutes: { label: "阅读时长", unit: "分钟" },
  wordsRead: { label: "读了", unit: "字" },
  chatTurns: { label: "AI 对话轮数", unit: "" },
  highlights: { label: "划线", unit: "处" },
  notes: { label: "笔记", unit: "条" },
  lenses: { label: "用了透镜", unit: "个" },
  stepsDone: { label: "阅读任务完成数", unit: "" },
  // writing
  words: { label: "写了", unit: "字" },
  outline: { label: "搭了提纲", unit: "条" },
  snippets: { label: "改了", unit: "段" },
  comments: { label: "印记读了", unit: "遍" },
};

/** The stat as it should read on screen. Never mutates the input. */
export function displayStat(stat: ReportStat): ReportStat {
  const over = STAT_DISPLAY[stat.key];
  if (!over) return stat;
  return { ...stat, label: over.label, unit: over.unit ?? stat.unit };
}
