import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiError } from "./client";

vi.mock("./client", async (orig) => {
  const real = await orig<typeof import("./client")>();
  return { ...real, apiFetch: vi.fn() };
});

import { apiFetch } from "./client";
import { fetchMaterialFromSeed, listMaterials, MaterialFetchError } from "./materials";

const material = {
  id: "m1", task_id: "t1", kind: "article", source: "fetched", title: "T",
  source_url: "https://x", blocks: [{ id: "b0", text: "一段" }], scratch: "", created_at: "1",
};

describe("materials api", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("listMaterials unwraps the materials array", async () => {
    (apiFetch as any).mockResolvedValue({ materials: [material] });
    expect(await listMaterials("t1")).toEqual([material]);
  });

  it("fetchMaterialFromSeed returns the material on success", async () => {
    (apiFetch as any).mockResolvedValue({ material });
    expect(await fetchMaterialFromSeed("t1")).toEqual(material);
  });

  it("fetchMaterialFromSeed maps material_fetch_failed → MaterialFetchError(reason)", async () => {
    (apiFetch as any).mockRejectedValue(new ApiError("material_fetch_failed", "x", 422, { reason: "blocked" }));
    await expect(fetchMaterialFromSeed("t1")).rejects.toMatchObject({ name: "MaterialFetchError", reason: "blocked" });
  });

  it("fetchMaterialFromSeed rethrows other ApiErrors unchanged", async () => {
    (apiFetch as any).mockRejectedValue(new ApiError("not_found", "x", 404));
    await expect(fetchMaterialFromSeed("t1")).rejects.toMatchObject({ code: "not_found" });
  });
});
