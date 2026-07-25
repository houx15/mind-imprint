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
