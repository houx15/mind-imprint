import { CollectedCards, type CollectedCard } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// The 工具卡 tab: tool cards the student has completed across all surfaces.
// Read-only, cost-free.
export async function getGrowthCards(): Promise<CollectedCard[]> {
  const raw = await apiFetch<unknown>(`/api/v1/growth/cards`);
  return CollectedCards.parse(raw).cards;
}
