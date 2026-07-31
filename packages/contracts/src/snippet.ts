import { z } from "zod";

// One 片段 (#23): a freeform text fragment on the writing room's snippet board —
// a flat, ordered list (no depth), collected while drafting. position orders them.
export const Snippet = z.object({
  id: z.string(),
  text: z.string(),
  position: z.number().int(),
});
export type Snippet = z.infer<typeof Snippet>;
