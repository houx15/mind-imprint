import { z } from "zod";
import { Credibility5, KeyQuote, PhaseTag } from "./reference";

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

// ReadingBrief — S2: the client-side shape of "why read THIS source" (the
// student's editable brief, putReadingBrief). phaseTag allows "" (not yet
// set) alongside the closed PhaseTag enum — the api client maps this to the
// server's snake_case request body (Task 9).
export const ReadingBrief = z.object({
  readingReason: z.string(),
  readingFocus: z.string(),
  phaseTag: PhaseTag.or(z.literal("")),
});
export type ReadingBrief = z.infer<typeof ReadingBrief>;

// TakeawayDraft — S2: getTakeawayDraft's response. record is the assembled,
// deterministic half (never re-guessed — findings/credibility/keyQuotes come
// straight off the student's confirmed reading cards); suggestedNewLeads/
// suggestedProposalImpact are the one isolated compose call's seed for the
// synthesis half, which the student edits before finalize-reading persists it.
export const TakeawayDraft = z.object({
  record: z.object({
    findings: z.array(z.string()),
    credibility: Credibility5,
    keyQuotes: z.array(KeyQuote),
  }),
  suggestedNewLeads: z.array(z.string()),
  suggestedProposalImpact: z.string(),
});
export type TakeawayDraft = z.infer<typeof TakeawayDraft>;
