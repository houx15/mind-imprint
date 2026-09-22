import type { AtomKind, ReportStat } from "@lite/api/reports";

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
const STAT_DISPLAY: Record<string, { label: string; unit?: string; fallbackUnit?: string }> = {
  // reading
  focusMinutes: { label: "阅读时长", unit: "分钟" },
  // No unit override: the server writes 字 or 词 by language (lengthUnit);
  // fallbackUnit only fills a stored stat that has no unit at all.
  wordsRead: { label: "读了", fallbackUnit: "字" },
  chatTurns: { label: "AI 对话轮数", unit: "" },
  // 2026-09-22：轻量版里每一处高亮都是她按「摘抄」留下的，标签跟着她屏幕上
  // 那颗按钮叫。🚨 这里是标签的真相源 —— 报告是**存下来的**一团 JSON，
  // 里面冻着旧标签，改 Go 只影响以后生成的那些。
  highlights: { label: "摘抄", unit: "处" },
  notes: { label: "笔记", unit: "条" },
  lenses: { label: "用了透镜", unit: "个" },
  stepsDone: { label: "阅读任务完成数", unit: "" },
  // writing
  words: { label: "写了", fallbackUnit: "字" },
  outline: { label: "搭了提纲", unit: "条" },
  snippets: { label: "改了", unit: "段" },
  comments: { label: "印记读了", unit: "遍" },
};

/**
 * Overrides that apply to ONE kind of report only, layered over the table
 * above.
 *
 * `focusMinutes` is the single key both rooms emit, and 「阅读时长」 — the
 * wording the server writes and the only one this file used to carry — is
 * simply false on a writing report, where it stands over the minutes she
 * spent WRITING. It was wrong on the page, on the exported picture, and on
 * the link she sends someone.
 *
 * A per-kind layer rather than a second key: the value means the same thing
 * in both rooms (minutes of attention on this atom), so it is one stat with
 * two names. `buildReadingReport` / `buildWritingReport` (atom_report.go) keep
 * emitting the same key, and every writing report ALREADY in the database
 * picks the corrected wording up on re-serve — which is the whole reason
 * labels resolve here instead of being trusted from the stored blob.
 */
const STAT_DISPLAY_BY_KIND: Record<AtomKind, Record<string, { label: string; unit?: string; fallbackUnit?: string }>> = {
  reading: {},
  writing: {
    focusMinutes: { label: "写作时长", unit: "分钟" },
  },
};

/**
 * The stat as it should read on screen. Never mutates the input.
 *
 * 🚨 `kind` is a required second parameter, so never call this as
 * `stats.map(displayStat)`: `Array.prototype.map` hands its callback
 * `(value, index, array)`, which would pass the INDEX as the kind. Both
 * call sites pass it explicitly for that reason.
 */
export function displayStat(stat: ReportStat, kind: AtomKind): ReportStat {
  const over = STAT_DISPLAY_BY_KIND[kind][stat.key] ?? STAT_DISPLAY[stat.key];
  if (!over) return stat;
  return { ...stat, label: over.label, unit: over.unit ?? (stat.unit || over.fallbackUnit || "") };
}
