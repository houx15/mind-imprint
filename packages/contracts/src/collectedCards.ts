import { z } from "zod";

// CollectedCard: one tool card the student has completed, with a usage summary.
// Mirrors apps/api/internal/api.collectedCardDTO (camelCase). Instance-derived
// only — name/purpose/category are resolved from CARD_REGISTRY on the web, not
// carried here. RL-5: uses is descriptive, never a grade.
export const CollectedCard = z.object({
  cardId: z.string(),
  uses: z.number().int(),
  surfaces: z.array(z.string()), // subset of "project" | "course" | "chat"
  lastUsed: z.string(),          // RFC3339
});
export type CollectedCard = z.infer<typeof CollectedCard>;

export const CollectedCards = z.object({ cards: z.array(CollectedCard) });
export type CollectedCards = z.infer<typeof CollectedCards>;
