import { z } from "zod";

// Slice 2 · the card-reflect turn response (POST /projects/{id}/cards/reflect).
// A completed card is persisted AND the coach replies to its content. reply is
// "" for an empty card (a no-op: no persist, no spend); cardInstanceId is ""
// in that same no-op case, else the created card_instance id.
export const CardReflectReply = z.object({
  cardInstanceId: z.string(),
  reply: z.string(),
});
export type CardReflectReply = z.infer<typeof CardReflectReply>;
