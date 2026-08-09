import { z } from "zod";
import { SearchSuggestion } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// searchGuidance.ts — slice 5 (§113/§116) · POST generates 印记's search
// suggestions (spends on the fast model; the student taps to ask).
const Resp = z.object({ suggestions: z.array(SearchSuggestion) });

export async function proposeSearchGuidance(projectId: string): Promise<SearchSuggestion[]> {
  return Resp.parse(
    await apiFetch<unknown>(`/api/v1/projects/${projectId}/search-guidance`, { method: "POST" }),
  ).suggestions;
}
