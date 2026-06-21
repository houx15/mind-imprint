import { z } from "zod";
import { SoloLevel } from "./rubric";

export const DimScore = z.object({ dim_id: z.string(), level: SoloLevel, note: z.string() });
export const EvalLlmOutput = z.object({ scores: z.array(DimScore), narrative: z.string() });
export const Evaluation = z.object({
  task_id: z.string(),
  scores: z.array(DimScore),
  narrative: z.string(),
  created_at: z.string(),
});

export type DimScore = z.infer<typeof DimScore>;
export type EvalLlmOutput = z.infer<typeof EvalLlmOutput>;
export type Evaluation = z.infer<typeof Evaluation>;
