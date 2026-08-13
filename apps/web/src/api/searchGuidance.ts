import { z } from "zod";
import { SearchSuggestion } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// searchGuidance.ts — slice 5 (§113/§116) · POST generates 印记's search
// suggestions (spends on the fast model; the student taps to ask).
const Resp = z.object({ suggestions: z.array(SearchSuggestion) });

export async function proposeSearchGuidance(
  projectId: string,
  focusQuestion?: string,
): Promise<SearchSuggestion[]> {
  // focusQuestion (item 3.1): the question layer the student is currently inside,
  // so the directions bias toward it. Omitted ⇒ whole-topic guidance.
  const body = focusQuestion && focusQuestion.trim() !== "" ? JSON.stringify({ focusQuestion }) : undefined;
  return Resp.parse(
    await apiFetch<unknown>(`/api/v1/projects/${projectId}/search-guidance`, { method: "POST", body }),
  ).suggestions;
}
