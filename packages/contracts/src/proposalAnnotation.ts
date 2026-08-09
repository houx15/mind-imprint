import { z } from "zod";

// proposalAnnotation.ts — slice 3b · the 批注 primitive: layered, colored,
// VIEW-ONLY teacher annotations shown in the left reference panel (all-statuses.md
// §4). 批注 never touches the editable draft (铁律①). It is deliberately sparse —
// a teacher marks only what's worth marking.
//
// level:  paper (whole article) / paragraph / sentence
// nature: good=green / suggest=blue / problem=red
//   - sentence 批注: rendered with a blue/red UNDERLINE on the quoted text.
//   - paragraph 批注: a blue/red comment (no underline), only where warranted.
//   - paper 批注: a green/blue/red overall summary (green = what's done well).

export const AnnotationLevel = z.enum(["paper", "paragraph", "sentence"]);
export type AnnotationLevel = z.infer<typeof AnnotationLevel>;

export const AnnotationNature = z.enum(["good", "suggest", "problem"]);
export type AnnotationNature = z.infer<typeof AnnotationNature>;

export const DraftAnnotation = z.object({
  id: z.string(),
  level: AnnotationLevel,
  nature: AnnotationNature,
  quote: z.string(), // the sentence text (sentence level); "" for paper
  locator: z.string(), // a human paragraph reference (e.g. "第2段"); "" for paper
  note: z.string(), // the teacher's comment — a direction, never a rewrite
});
export type DraftAnnotation = z.infer<typeof DraftAnnotation>;
