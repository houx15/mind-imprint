import { AbilityModel } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// C: the student-level 能力素养 model — a deterministic cross-session merge. Read-only.
export async function getAbilityModel(): Promise<AbilityModel> {
  const raw = await apiFetch<unknown>(`/api/v1/growth/ability`);
  return AbilityModel.parse(raw);
}
