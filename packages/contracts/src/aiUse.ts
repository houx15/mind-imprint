import { z } from "zod";

// S5 · AI-interaction retrospective (回顾 · 复盘我与 AI 的互动). Split-hybrid: the
// student AUTHORS the two synthesis fields (AI 克制 — the reflection must be her
// own); the AI only assembles the objective record and seeds a draft.
export const AIUseStatement = z.object({
  usedFor: z.string(),
  notUsedFor: z.string(),
});
export type AIUseStatement = z.infer<typeof AIUseStatement>;

// The objective, un-forgeable interaction record — assembled server-side from
// the event stream + llm_call rows. Read-only to the student; the AI cannot
// influence it (it is not a model output). Includes the ABSENCES that matter
// (no essay ghostwriting, no score prediction) as asserted facts.
export const AIUseRecord = z.object({
  coachTurns: z.number().int().nonnegative(),
  cardsProposed: z.number().int().nonnegative(),
  cardsAccepted: z.number().int().nonnegative(),
  cardsDismissed: z.number().int().nonnegative(),
  sourcesOpened: z.number().int().nonnegative(),
  llmCallsByPurpose: z.record(z.string(), z.number().int().nonnegative()),
  ghostwroteEssay: z.boolean(),
  predictedScore: z.boolean(),
});
export type AIUseRecord = z.infer<typeof AIUseRecord>;

// GET /ai-use-draft returns the objective record + a seeded (or already-saved)
// statement draft the student edits before finalizing.
export const AIUseDraft = z.object({
  record: AIUseRecord,
  draft: AIUseStatement,
});
export type AIUseDraft = z.infer<typeof AIUseDraft>;
