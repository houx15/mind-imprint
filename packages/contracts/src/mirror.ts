import { z } from "zod";

// One prose section of the generated "mirror" (first-open-wins, POST-only-spend).
export const MirrorSection = z.object({
  title: z.string(),
  body: z.string(),
});
export type MirrorSection = z.infer<typeof MirrorSection>;

// The whole mirror: sections composed from the process + two carry-forwards
// ("带走这两点").
export const Mirror = z.object({
  sections: z.array(MirrorSection),
  carryForwards: z.array(z.string()),
});
export type Mirror = z.infer<typeof Mirror>;
