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

// One row in the reading library. materialId links to readable content (created
// lazily on first Reading-Room entry); notes are projected from card_instances/
// reading-outcomes anchored to materialId.
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
  decision: UseDecision,
  pending: z.boolean(),
  searchHints: z.array(z.string()),
  materialId: z.string().nullable(),
  notes: z.array(ReadingNote),
});
export type Reference = z.infer<typeof Reference>;
