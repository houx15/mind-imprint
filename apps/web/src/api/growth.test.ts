import { describe, it, expect, vi, afterEach } from "vitest";
import { getGrowthHistory } from "./growth";

afterEach(() => { vi.restoreAllMocks(); });

describe("getGrowthHistory", () => {
  it("parses entries", async () => {
    const body = {
      entries: [
        {
          surface: "chat", scopeId: "t1", label: "CRAAP", sublabel: null, createdAt: "2026-07-18T00:00:00Z",
          report: { dimensions: [], narrative: "n", generatedAt: "2026-07-18T00:00:00Z" },
        },
      ],
    };
    vi.spyOn(global, "fetch").mockResolvedValue(new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } }));
    const list = await getGrowthHistory();
    expect(list).toHaveLength(1);
    expect(list[0]!.surface).toBe("chat");
  });

  it("throws on a malformed history (schema drift)", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(new Response(JSON.stringify({ entries: [{ surface: "bogus" }] }), { status: 200, headers: { "Content-Type": "application/json" } }));
    await expect(getGrowthHistory()).rejects.toThrow();
  });
});
