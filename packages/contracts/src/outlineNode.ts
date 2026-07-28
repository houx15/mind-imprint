import { z } from "zod";

// One bullet in the Write-room outline. depth drives indent (bullets ⇄ mind-map
// over the same persisted data); position orders siblings.
export const OutlineNode = z.object({
  id: z.string(),
  text: z.string(),
  depth: z.number().int(),
  position: z.number().int(),
});
export type OutlineNode = z.infer<typeof OutlineNode>;
