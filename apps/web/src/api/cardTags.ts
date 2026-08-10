import { z } from "zod";
import { apiFetch } from "./client";

// cardTags.ts — §4 gap G8 · per-guided-part status tags (green=写好了 / yellow=待完善),
// keyed by step key, stored on studio_state.

export type CardTag = "green" | "yellow" | "";

const Resp = z.object({ tags: z.record(z.string(), z.string()) });

export async function getCardTags(projectId: string): Promise<Record<string, string>> {
  return Resp.parse(await apiFetch<unknown>(`/api/v1/projects/${projectId}/card-tags`)).tags;
}

export async function putCardTag(projectId: string, key: string, status: CardTag): Promise<Record<string, string>> {
  return Resp.parse(
    await apiFetch<unknown>(`/api/v1/projects/${projectId}/card-tags`, {
      method: "PUT",
      body: JSON.stringify({ key, status }),
    }),
  ).tags;
}
