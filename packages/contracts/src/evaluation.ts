import { z } from "zod";
import { SoloLevel } from "./rubric";

export const DimScore = z.object({ dim_id: z.string(), level: SoloLevel, note: z.string() });
export const EvalLlmOutput = z.object({ scores: z.array(DimScore), narrative: z.string() });

export const EvalStatus = z.enum(["queued", "running", "done", "failed"]);
export type EvalStatus = z.infer<typeof EvalStatus>;

export const Evaluation = z.object({
  // id/status/completed_at default so server payloads AND older fixtures both validate;
  // the server always sends real values.
  id: z.string().default(""),
  task_id: z.string(),
  status: EvalStatus.default("done"),
  scores: z.array(DimScore),
  narrative: z.string(),
  created_at: z.string(),
  completed_at: z.string().nullable().default(null),
});

export type DimScore = z.infer<typeof DimScore>;
export type EvalLlmOutput = z.infer<typeof EvalLlmOutput>;
export type Evaluation = z.infer<typeof Evaluation>;
