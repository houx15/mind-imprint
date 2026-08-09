import { z } from "zod";

// resourceNeed.ts — slice 5 · the "还需要探索的" (needs-resources) box (§101, §115).
// A small, student-authored list of things they realised they need to look up
// while writing. Lives on the proposal writing page AND the reading room; a
// "去探索" jump carries the note into the reading room. Persisted per project.
export const ResourceNeed = z.object({
  id: z.string(),
  text: z.string(),
  done: z.boolean().default(false),
});
export type ResourceNeed = z.infer<typeof ResourceNeed>;

export const ResourceNeedList = z.object({
  needs: z.array(ResourceNeed),
});
export type ResourceNeedList = z.infer<typeof ResourceNeedList>;
