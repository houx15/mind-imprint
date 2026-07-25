import { z } from "zod";

// ParentReport: the parent-facing projection of the canonical assessment
// object (project mode). Deterministic badges/states + gentled prose. RL-5:
// no total; the A axis carries no number anywhere.
const ParentDRow = z.object({
  code: z.string(),
  name: z.string(),
  badge: z.string(), // 起步/发展/熟练/优秀/暂无 (server-single-sourced)
  reading: z.string(),
});

const ParentARow = z.object({
  code: z.string(),
  name: z.string(),
  state: z.string(), // 观察到主动信号 / 偶有·多在引导后 / 暂未观察到
  reading: z.string(),
});

const ParentAdvice = z.object({ title: z.string(), text: z.string() });

// ParentReportProse = the composer output / stored bundle.
export const ParentReportProse = z.object({
  glance: z.string(),
  dOverview: z.string(),
  aOverview: z.string(),
  opportunity: z.string(),
  warmLine: z.string(),
  dReadings: z.record(z.string()),
  aReadings: z.record(z.string()),
  advice: z.array(ParentAdvice),
});
export type ParentReportProse = z.infer<typeof ParentReportProse>;

export const ParentReport = z.object({
  cover: z.object({
    name: z.string(),
    subject: z.string(),
    klass: z.string(),
    typeLabel: z.string(),
    dateStr: z.string(),
    warmLine: z.string(),
  }),
  glance: z.string(),
  dOverview: z.string(),
  aOverview: z.string(),
  dRows: z.array(ParentDRow),
  aRows: z.array(ParentARow),
  opportunity: z.string(),
  advice: z.array(ParentAdvice),
  prose: z.union([z.literal("present"), z.null()]),
});
export type ParentReport = z.infer<typeof ParentReport>;

// ── E2: stage mode ──────────────────────────────────────────────────────────
// The stage report is a per-(student, week) parent projection: usage stats +
// a cross-session growth narrative. No per-dim D/A rows, no A number (RL-5),
// no ability level number.

export const ParentStageProse = z.object({
  warmLine: z.string(),
  stageGrowth: z.string(),
  stageHighlight: z.string(), // may be "" (敢于空白)
  stageForward: z.string(),
  advice: z.array(ParentAdvice), // exactly 3 when composed
});
export type ParentStageProse = z.infer<typeof ParentStageProse>;

export const ParentStageStat = z.object({
  value: z.string(), // "6 天" / "78" / "3 份" / "5 节" (unit folded in server-side)
  label: z.string(), // 本周活跃 / 对话轮次 / 生成报告 / 完成课程
});
export type ParentStageStat = z.infer<typeof ParentStageStat>;

export const ParentStageReport = z.object({
  cover: z.object({
    name: z.string(),
    subject: z.string(), // 第 N 周（M.D–M.D）
    klass: z.string(),
    typeLabel: z.string(), // 阶段报告
    dateStr: z.string(),
    warmLine: z.string(),
  }),
  stats: z.array(ParentStageStat), // 4
  stageGrowth: z.string(),
  stageHighlight: z.string(), // "" ⇒ section hidden
  stageForward: z.string(),
  advice: z.array(ParentAdvice), // 3, or [] pre-prose
  prose: z.union([z.literal("present"), z.null()]),
});
export type ParentStageReport = z.infer<typeof ParentStageReport>;
