import { z } from "zod";

// A structured reference to a completed tool card, carried on the card-turn
// chat_message so a reloaded thread can re-render the card as a clickable chip
// (content-first) that opens a READ-ONLY view of the student's own answers — no
// separate fetch needed. cardId resolves the spec from CARD_REGISTRY;
// fieldValues are the student's raw answers (the record — never editable).
export const CardTurnRef = z.object({
  cardId: z.string(),
  fieldValues: z.record(z.unknown()),
});
export type CardTurnRef = z.infer<typeof CardTurnRef>;

// Slice 2 · the card-reflect turn response (POST /projects/{id}/cards/reflect).
// A completed card is persisted AND the coach replies to its content. reply is
// "" for an empty card (a no-op: no persist, no spend); cardInstanceId is ""
// in that same no-op case, else the created card_instance id. `card` echoes the
// persisted card reference (cardId + the student's fieldValues) so the live turn
// renders the same chip a reloaded thread does; null on the empty-card no-op.
export const CardReflectReply = z.object({
  cardInstanceId: z.string(),
  reply: z.string(),
  card: CardTurnRef.nullish(),
});
export type CardReflectReply = z.infer<typeof CardReflectReply>;
