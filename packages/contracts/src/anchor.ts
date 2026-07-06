import { z } from "zod";

export const AnchorAuthor = z.enum(["ai", "student"]);

export const Anchor = z.object({
  id: z.string(),
  material_id: z.string(),
  block_id: z.string(),
  start: z.number().int().nonnegative(),
  end: z.number().int().nonnegative(),
  quote: z.string(),
  dimension: z.string(),
  author: AnchorAuthor,
  question: z.string(),
  answer: z.string(),
});

export type AnchorAuthor = z.infer<typeof AnchorAuthor>;
export type Anchor = z.infer<typeof Anchor>;
