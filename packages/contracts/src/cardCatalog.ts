import { z } from "zod";

// CardCatalogEntry: one tool card in the 工具卡图鉴 (成长报告 → 工具卡). Mirrors
// apps/api/internal/api.cardCatalogEntryDTO (camelCase). Every registered card
// appears — encountered or not. coverUrl is a short-lived signed URL for the
// chosen theme (absent when the card has no cover art or OSS is off → the web
// renders a text face). The proficiency fields are descriptive, never a grade
// (RL-5): encountered drives colored-vs-greyscale, stars is 0 when not
// encountered else 1..5.
export const CardCatalogEntry = z.object({
  cardId: z.string(),
  name: z.string(),
  nameEn: z.string(),
  category: z.string(),
  purpose: z.string(),
  // whenToUse mirrors the card's trigger_condition — the situation that calls
  // for it (何时使用). Optional/additive; empty when the card has none.
  whenToUse: z.string().optional().default(""),
  stages: z.array(z.string()),
  example: z.string(),
  hasAsset: z.boolean(),
  coverUrl: z.string().optional().default(""),
  courseId: z.string().optional().default(""),
  encountered: z.boolean(),
  score: z.number().int(),
  stars: z.number().int(),
  uses: z.number().int(),
  surfaces: z.array(z.string()),
  lastUsed: z.string().optional().default(""),
});
export type CardCatalogEntry = z.infer<typeof CardCatalogEntry>;

// COVER_THEMES is the closed set of colorways a student may choose; mirrors
// apps/api/internal/cards.Themes and the users.card_theme CHECK. "light" is the
// default.
export const COVER_THEMES = ["light", "cyber-sage", "cyber-slate", "cyber-warm"] as const;
export const CoverTheme = z.enum(COVER_THEMES);
export type CoverTheme = z.infer<typeof CoverTheme>;

export const CardCatalog = z.object({
  cards: z.array(CardCatalogEntry),
  theme: CoverTheme,
});
export type CardCatalog = z.infer<typeof CardCatalog>;
