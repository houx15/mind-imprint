import { z } from "zod";

// The student's own use decision on a source; null until she decides.
export const UseDecision = z.enum(["use", "maybe", "drop"]).nullable();
export type UseDecision = z.infer<typeof UseDecision>;

// The credibility read on a source. null on Reference until assessed.
export const Credibility = z.enum(["strong", "mixed", "weak"]);
export type Credibility = z.infer<typeof Credibility>;

// One reading note, projected from reading outcomes anchored to the material —
// not stored on the reference row itself.
export const ReadingNote = z.object({
  quote: z.string(),
  finding: z.string(),
});
export type ReadingNote = z.infer<typeof ReadingNote>;

// PhaseTag — S2: which stage of the argument this source is being read FOR
// (set via the reading brief, putReadingBrief). Drives the reading coach's
// framing, never a free-text field — the five stages are the demo model's
// closed set.
export const PhaseTag = z.enum(["立题探索", "背景理解", "支持论点", "反例检验", "方法参考"]);
export type PhaseTag = z.infer<typeof PhaseTag>;

// Credibility5/KeyQuote/ReadingTakeaway — S2: the reading sub-agent's "return".
// Credibility5 (distinct from the source-level Credibility enum above) is the
// student's own verdict + reasoning from a completed CRAAP/SIFT card, carried
// through verbatim. ReadingTakeaway is the full persisted/echoed 5-field
// object (agent.ReadingTakeaway on the Go side, same camelCase tags): the
// record half (findings/credibility/keyQuotes — the student's ALREADY
// confirmed work, never re-guessed) plus the synthesis half (newLeads/
// proposalImpact — seeded by one isolated compose call, the student edits and
// finalizes).
export const Credibility5 = z.object({ verdict: z.string(), why: z.string() });
export type Credibility5 = z.infer<typeof Credibility5>;

export const KeyQuote = z.object({ quote: z.string(), why: z.string() });
export type KeyQuote = z.infer<typeof KeyQuote>;

export const ReadingTakeaway = z.object({
  findings: z.array(z.string()),
  credibility: Credibility5,
  keyQuotes: z.array(KeyQuote),
  newLeads: z.array(z.string()),
  proposalImpact: z.string(),
});
export type ReadingTakeaway = z.infer<typeof ReadingTakeaway>;

// One row in the reading library. materialId links to readable content (created
// lazily on first Reading-Room entry); notes are projected from card_instances/
// reading-outcomes anchored to materialId. phaseTag/takeaway (S2) fold the
// reading sub-agent's state onto the same row: both are optional+nullable —
// absent/null until the student sets a reading brief / finalizes a takeaway.
// readingReason/readingFocus (Task 9 fix) surface the SAME persisted brief
// fields putReadingBrief writes — without them a client reopening a source has
// no true saved value to seed its brief editor from, so its next full-replace
// PUT resends a stale/blank value and silently wipes whichever field it
// couldn't see.
export const Reference = z.object({
  id: z.string(),
  title: z.string(),
  classification: z.string(),
  author: z.string(),
  credentials: z.string(),
  year: z.string(),
  url: z.string(),
  tags: z.array(z.string()),
  collectionId: z.string().nullable(),
  credibility: Credibility.nullable(),
  evaluation: z.string(),
  // #8: the student's own freeform note on this source (我的笔记), edited in the
  // reading room. Optional so older fixtures/mocks without it still parse.
  readingNote: z.string().optional(),
  decision: UseDecision,
  pending: z.boolean(),
  searchHints: z.array(z.string()),
  materialId: z.string().nullable(),
  notes: z.array(ReadingNote),
  phaseTag: PhaseTag.nullable().optional(),
  readingReason: z.string().nullable().optional(),
  readingFocus: z.string().nullable().optional(),
  takeaway: ReadingTakeaway.nullable().optional(),
  // #4: bibliographic metadata recovered from a DOI via Crossref and persisted
  // on the reference — the abstract (context, not the article body) + the
  // journal/container title (annotated-bib field). Optional (like readingNote)
  // so older fixtures/mocks without them still parse; the Go DTO always emits
  // them (defaulting to "") so at runtime they're present.
  abstract: z.string().optional(),
  journal: z.string().optional(),
  // A1: one shelf, three states (待读/在读/读完) — replaces separate
  // to-read/reading collections with a single status on every reference.
  // NOT NULL with a DB default of "to_read"; defaulted here too so older
  // fixtures/mocks without it still parse.
  readingStatus: z.enum(["to_read", "reading", "done"]).default("to_read"),
  // Slice 4a · 证据地图 facets on a paper gathered under a sub-question. Optional
  // (like readingNote/abstract) so older fixtures/mocks parse; the Go DTO always
  // emits them (defaulting to "" / false) so at runtime they're present.
  // triage = 必读(red)/待定(yellow)/'' (before reading); evidenceNature = how it
  // bears on the sub-question (支持/反驳); the structured per-paper note; archived
  // = "interesting but not really related" (off the active map, not deleted).
  triage: z.enum(["", "red", "yellow"]).optional(),
  evidenceNature: z.enum(["", "support", "challenge"]).optional(),
  evidenceArgument: z.string().optional(),
  evidenceFinding: z.string().optional(),
  evidencePlacement: z.string().optional(),
  archived: z.boolean().optional(),
});
export type Reference = z.infer<typeof Reference>;
