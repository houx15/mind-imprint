import { z } from "zod";

// essayTrack.ts — slice 4 · the essay's three-stage position (§6): research
// (build the 证据地图 in the reading room) → statement (outline + per-claim
// arguments) → submission (引言/conclusion/compose/polish). Slice 4a lands the
// research stage + the per-sub-question saturation verdict.

export const EssayStage = z.enum(["research", "statement", "submission"]);
export type EssayStage = z.infer<typeof EssayStage>;

// SubQuestionVerdict — the flagship saturation review of ONE sub-question's
// evidence (§6): is there enough strong support + at least one real
// challenge/limitation, and has new reading started repeating what's gathered?
// Advisory only (铁律②).
export const SubQuestionVerdict = z.object({
  subQuestionId: z.string(),
  saturated: z.boolean(),
  why: z.string(),
  gaps: z.array(z.string()),
});
export type SubQuestionVerdict = z.infer<typeof SubQuestionVerdict>;

// ClaimRevisionVerdict — slice 4b-2 · when the student edits a sub-question at the
// 大纲 step (§133–134), the flagship classifies the edit: a "rephrase" keeps the
// same research target so the claim's materials/writing still apply; a
// "total_change" asks a different question, so they no longer do. Advisory (铁律②)
// — surfaced as a warning, never a block; the student's writing is never deleted.
export const ClaimRevisionVerdict = z.object({
  kind: z.enum(["rephrase", "total_change"]),
  why: z.string(),
});
export type ClaimRevisionVerdict = z.infer<typeof ClaimRevisionVerdict>;

// EvidenceMapPaper / EvidenceMapSubQuestion / EvidenceMap — the read projection
// of the seeded warren for the research stage (the map is the graph; this is the
// per-sub-question rollup the reading room reads).
export const EvidenceMapPaper = z.object({
  id: z.string(),
  title: z.string(),
  nature: z.enum(["", "support", "challenge"]),
  triage: z.enum(["", "red", "yellow"]),
  hasNote: z.boolean(),
});
export type EvidenceMapPaper = z.infer<typeof EvidenceMapPaper>;

export const EvidenceMapSubQuestion = z.object({
  id: z.string(),
  text: z.string(),
  papers: z.array(EvidenceMapPaper),
});
export type EvidenceMapSubQuestion = z.infer<typeof EvidenceMapSubQuestion>;

export const EvidenceMap = z.object({
  mainQuestion: z.string(),
  subQuestions: z.array(EvidenceMapSubQuestion),
});
export type EvidenceMap = z.infer<typeof EvidenceMap>;
