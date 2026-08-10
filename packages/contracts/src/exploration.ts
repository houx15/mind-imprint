import { z } from "zod";

// S3 rabbit-hole exploration: a lead is a thread worth following, spawned from
// a reading takeaway's newLeads, typed in manually, or proposed by the 深挖一层
// guide. status/origin are closed enums — the S2 empty-string bug (a naive
// full-replace PUT persisting "" instead of a real enum member, which then
// broke the whole array parse) is exactly what these must never let through.
export const LeadStatus = z.enum(["open", "connected", "pruned"]);
export type LeadStatus = z.infer<typeof LeadStatus>;

export const LeadOrigin = z.enum(["takeaway", "manual", "guide", "note"]);
export type LeadOrigin = z.infer<typeof LeadOrigin>;

export const ExplorationLead = z.object({
  id: z.string(),
  text: z.string(),
  status: LeadStatus,
  origin: LeadOrigin,
  sourceReferenceId: z.string().nullable(),
  connectedReferenceId: z.string().nullable(),
  position: z.number(),
  // #12 · a 分支 hangs under a parent lead; null = a top-level thread.
  parentLeadId: z.string().nullable().default(null),
  // GVe · RFC3339, for the question-node sidebar (created date + origin).
  createdAt: z.string(),
});
export type ExplorationLead = z.infer<typeof ExplorationLead>;

// B1 · a labeled, directed edge between two top-level question leads — the
// data foundation of the two-level exploration graph. label is a closed
// vocabulary (no freeform jargon, 铁律②); status "proposed" means the AI
// surfaced it and the student hasn't confirmed it onto the map yet (铁律①),
// "confirmed" means it's a real edge (student-drawn, or an adopted proposal).
export const QuestionEdgeLabel = z.enum(["子问题", "支持", "反驳/张力", "细化", "依赖/前提"]);
export type QuestionEdgeLabel = z.infer<typeof QuestionEdgeLabel>;

export const QuestionEdgeStatus = z.enum(["proposed", "confirmed"]);
export type QuestionEdgeStatus = z.infer<typeof QuestionEdgeStatus>;

export const QuestionEdge = z.object({
  id: z.string(),
  fromLeadId: z.string(),
  toLeadId: z.string(),
  label: QuestionEdgeLabel,
  status: QuestionEdgeStatus,
});
export type QuestionEdge = z.infer<typeof QuestionEdge>;

// The exploration view's full projection: every lead plus which read sources
// have no leads hanging off them yet (danglingSourceIds), nudging the student
// to connect or prune rather than leaving them orphaned, plus the question_edge
// graph over the top-level leads.
export const ExplorationView = z.object({
  leads: z.array(ExplorationLead),
  danglingSourceIds: z.array(z.string()),
  edges: z.array(QuestionEdge),
});
export type ExplorationView = z.infer<typeof ExplorationView>;

// One proposed direction from the 深挖一层 (dig deeper) guide — never
// auto-added as a lead; the student chooses via [记为线索].
export const GuideDirection = z.object({ direction: z.string(), why: z.string() });
export type GuideDirection = z.infer<typeof GuideDirection>;

export const ExplorationGuide = z.object({ directions: z.array(GuideDirection) });
export type ExplorationGuide = z.infer<typeof ExplorationGuide>;

// #A3 · one OpenAlex candidate surfaced by POST /exploration/dig — the
// client-side tray, not the map. Adopting one into the map is a separate,
// explicit student action; dig itself never persists anything.
export const DigCandidate = z.object({
  doi: z.string(),
  title: z.string(),
  authors: z.string(),
  year: z.string(),
  journal: z.string(),
  abstract: z.string(),
  url: z.string(),
});
export type DigCandidate = z.infer<typeof DigCandidate>;

export const DigResult = z.object({ candidates: z.array(DigCandidate) });
export type DigResult = z.infer<typeof DigResult>;

// 印记 for one reference suggests the best-fit question to hang it under, or
// null (→ 未归类). Advisory only (铁律②): the student taps to confirm the placement.
export const PlacementSuggestion = z.object({
  leadId: z.string().nullable(),
  reason: z.string(),
});
export type PlacementSuggestion = z.infer<typeof PlacementSuggestion>;
