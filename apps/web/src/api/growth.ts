import { GrowthHistory, type GrowthHistoryEntry } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// A3: the 成长报告 history hub. Read-only — no generation lives here (reports are
// one-time terminal artifacts). newest-first, owner-filtered server-side.
export async function getGrowthHistory(): Promise<GrowthHistoryEntry[]> {
  const raw = await apiFetch<unknown>(`/api/v1/growth/history`);
  return GrowthHistory.parse(raw).entries;
}
