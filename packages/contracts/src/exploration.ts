import { z } from "zod";

// S3 rabbit-hole exploration: a lead is a thread worth following, spawned from
// a reading takeaway's newLeads, typed in manually, or proposed by the 深挖一层
// guide. status/origin are closed enums — the S2 empty-string bug (a naive
// full-replace PUT persisting "" instead of a real enum member, which then
// broke the whole array parse) is exactly what these must never let through.
export const LeadStatus = z.enum(["open", "connected", "pruned"]);
export type LeadStatus = z.infer<typeof LeadStatus>;

export const LeadOrigin = z.enum(["takeaway", "manual", "guide"]);
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
});
export type ExplorationLead = z.infer<typeof ExplorationLead>;

// The exploration view's full projection: every lead plus which read sources
// have no leads hanging off them yet (danglingSourceIds), nudging the student
// to connect or prune rather than leaving them orphaned.
export const ExplorationView = z.object({
  leads: z.array(ExplorationLead),
  danglingSourceIds: z.array(z.string()),
});
export type ExplorationView = z.infer<typeof ExplorationView>;

// One proposed direction from the 深挖一层 (dig deeper) guide — never
// auto-added as a lead; the student chooses via [记为线索].
export const GuideDirection = z.object({ direction: z.string(), why: z.string() });
export type GuideDirection = z.infer<typeof GuideDirection>;

export const ExplorationGuide = z.object({ directions: z.array(GuideDirection) });
export type ExplorationGuide = z.infer<typeof ExplorationGuide>;
