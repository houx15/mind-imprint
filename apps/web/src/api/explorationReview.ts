import { z } from "zod";
import { apiFetch } from "./client";

// explorationReview.ts — §5 (user follow-up) · POST 印记's holistic review of the
// collected + tagged materials (advisory; spends on the flagship reviewer).
const Resp = z.object({ review: z.string() });

export async function reviewExploration(projectId: string): Promise<string> {
  return Resp.parse(await apiFetch<unknown>(`/api/v1/projects/${projectId}/exploration/review`, { method: "POST" })).review;
}
