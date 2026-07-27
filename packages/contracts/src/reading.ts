import { z } from "zod";

export const SelectionCheck = z.object({
  key: z.string(),
  label: z.string(),
  status: z.enum(["pass", "partial", "miss"]),
  evidence: z.string(),
  explanation: z.string(),
});

export const SelectionEval = z.object({
  verdict: z.enum(["strong", "partial", "rethink"]),
  verdictLabel: z.string(),
  verdictReason: z.string(),
  checks: z.array(SelectionCheck),
  finding: z.string(),
  judgment: z.string(),
  support: z.string(),
  caveat: z.string(),
  nextStep: z.string(),
  spanIds: z.array(z.string()),
});

export const ReadTurnBody = z.object({
  student_text: z.string(),
  focused_spans: z.array(z.object({ block_id: z.string(), quote: z.string() })),
});

export const EvaluateBody = z.object({
  block_id: z.string(),
  start: z.number().int(),
  end: z.number().int(),
  quote: z.string(),
  dimension: z.string(),
});

export type SelectionCheck = z.infer<typeof SelectionCheck>;
export type SelectionEval = z.infer<typeof SelectionEval>;
export type ReadTurnBody = z.infer<typeof ReadTurnBody>;
export type EvaluateBody = z.infer<typeof EvaluateBody>;
