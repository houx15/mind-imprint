import { z } from "zod";

// One 片段 (#23): a freeform text fragment on the writing room's snippet board —
// an ordered list collected while drafting. position orders them. section (#5) is
// the label it's filed under — a top-level outline heading or a 线索's text — or
// null when 未归类 (uncategorized). Both are plain string labels: outline node
// ids are re-minted every PUT, so a snippet can't reference one stably.
export const Snippet = z.object({
  id: z.string(),
  text: z.string(),
  position: z.number().int(),
  section: z.string().nullable().default(null),
});
export type Snippet = z.infer<typeof Snippet>;
