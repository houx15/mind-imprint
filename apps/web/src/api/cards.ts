import {
  CollectedCards,
  type CollectedCard,
  CardCatalog,
  type CardCatalogEntry,
  type CoverTheme,
} from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// The 工具卡 tab: tool cards the student has completed across all surfaces.
// Read-only, cost-free.
export async function getGrowthCards(): Promise<CollectedCard[]> {
  const raw = await apiFetch<unknown>(`/api/v1/growth/cards`);
  return CollectedCards.parse(raw).cards;
}

// The 工具卡图鉴: EVERY registered card + gallery metadata + cover URLs for the
// chosen theme + the caller's proficiency. Read-only, cost-free. When theme is
// omitted the server uses the student's saved card_theme. Returns both the cards
// and the theme actually rendered.
export async function getCardsCatalog(
  theme?: CoverTheme,
): Promise<{ cards: CardCatalogEntry[]; theme: CoverTheme }> {
  const qs = theme ? `?theme=${encodeURIComponent(theme)}` : "";
  const raw = await apiFetch<unknown>(`/api/v1/cards/catalog${qs}`);
  return CardCatalog.parse(raw);
}

// Persist the student's chosen cover colorway. Pure preference write.
export async function setCardTheme(theme: CoverTheme): Promise<CoverTheme> {
  const raw = await apiFetch<{ theme: CoverTheme }>(`/api/v1/cards/theme`, {
    method: "PUT",
    body: JSON.stringify({ theme }),
  });
  return raw.theme;
}
