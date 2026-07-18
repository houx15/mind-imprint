import { z } from "zod";
import { Assessment } from "./assessment";

// A3: one row of the 成长报告 history hub. surface + label + date, with the full
// report embedded. RL-5: no score/level at row level — only inside report.
export const GrowthHistoryEntry = z.object({
  surface: z.enum(["project", "course", "chat"]),
  scopeId: z.string(),
  label: z.string(),
  sublabel: z.string().nullable(),
  createdAt: z.string(),
  report: Assessment,
});
export type GrowthHistoryEntry = z.infer<typeof GrowthHistoryEntry>;

export const GrowthHistory = z.object({
  entries: z.array(GrowthHistoryEntry),
});
export type GrowthHistory = z.infer<typeof GrowthHistory>;
