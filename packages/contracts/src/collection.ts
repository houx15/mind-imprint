import { z } from "zod";

// A folder in the reading library. parentId is a self-reference for nesting;
// null at the top level. position orders siblings.
export const Collection = z.object({
  id: z.string(),
  name: z.string(),
  parentId: z.string().nullable(),
  position: z.number().int(),
});
export type Collection = z.infer<typeof Collection>;
