import { z } from "zod";
import { ResourceNeed } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// resourceNeeds.ts — slice 5 · the "还需要探索的" box client (§101/§115). GET reads
// the list; PUT replaces it whole (the box saves the whole set, like snippets).

const ListResp = z.object({ needs: z.array(ResourceNeed) });

export async function getResourceNeeds(projectId: string): Promise<ResourceNeed[]> {
  return ListResp.parse(await apiFetch<unknown>(`/api/v1/projects/${projectId}/resource-needs`)).needs;
}

export async function putResourceNeeds(projectId: string, needs: ResourceNeed[]): Promise<ResourceNeed[]> {
  return ListResp.parse(
    await apiFetch<unknown>(`/api/v1/projects/${projectId}/resource-needs`, {
      method: "PUT",
      body: JSON.stringify({ needs }),
    }),
  ).needs;
}
