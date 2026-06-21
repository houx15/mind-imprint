import { z } from "zod";

export const SummonCardArgs = z.object({
  card_id: z.string(),
  reason: z.string(),
  nudge_text: z.string(),
});

export const SummonCardCall = z.object({
  id: z.string(),
  name: z.literal("summon_card"),
  args: SummonCardArgs,
  card_instance_id: z.string(),
});

export type SummonCardArgs = z.infer<typeof SummonCardArgs>;
export type SummonCardCall = z.infer<typeof SummonCardCall>;
