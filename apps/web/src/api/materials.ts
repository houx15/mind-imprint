import type { Material } from "@mind-imprint/contracts";
import { apiFetch, ApiError } from "./client";

// Thrown when the server could not fetch/extract the seed URL. The pane treats
// this as "fall back to paste", not an error to surface.
export class MaterialFetchError extends Error {
  constructor(public readonly reason: string) {
    super(`material_fetch_failed: ${reason}`);
    this.name = "MaterialFetchError";
  }
}

export async function listMaterials(taskId: string): Promise<Material[]> {
  const r = await apiFetch<{ materials: Material[] }>(`/api/v1/tasks/${taskId}/materials`);
  return r.materials;
}

export async function createMaterial(
  taskId: string,
  input: { kind: "article" | "draft"; title: string; text: string },
): Promise<Material> {
  const r = await apiFetch<{ material: Material }>(`/api/v1/tasks/${taskId}/materials`, {
    method: "POST",
    body: JSON.stringify(input),
  });
  return r.material;
}

export async function fetchMaterialFromSeed(taskId: string): Promise<Material> {
  try {
    const r = await apiFetch<{ material: Material }>(`/api/v1/tasks/${taskId}/materials/from-seed`, {
      method: "POST",
    });
    return r.material;
  } catch (e) {
    if (e instanceof ApiError && e.code === "material_fetch_failed") {
      const reason = (e.details as { reason?: string } | undefined)?.reason ?? "unreachable";
      throw new MaterialFetchError(reason);
    }
    throw e;
  }
}

export async function saveScratch(taskId: string, materialId: string, scratch: string): Promise<Material> {
  const r = await apiFetch<{ material: Material }>(
    `/api/v1/tasks/${taskId}/materials/${materialId}/scratch`,
    { method: "PUT", body: JSON.stringify({ scratch }) },
  );
  return r.material;
}
