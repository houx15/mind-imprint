import { z } from "zod";

// One 批注 (annotation): a persisted 整稿体检 (whole-draft review) item — the
// same review_item intervention rows the review coach writes, projected down
// to what 印记 curates into the writing stage's reference panel via
// curate_reference kind:"annotation". `text` is the missing/fix guidance
// already combined server-side (apps/api's getAnnotations DTO) — this schema
// is intentionally flat, no nested envelope.
export const Annotation = z.object({
  id: z.string(),
  criterion: z.string(),
  band: z.string(),
  text: z.string(),
});
export type Annotation = z.infer<typeof Annotation>;
