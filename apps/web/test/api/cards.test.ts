import { describe, it, expect, vi, afterEach } from "vitest";
import { getGrowthCards } from "@/api/cards";

afterEach(() => { vi.restoreAllMocks(); });

describe("getGrowthCards", () => {
  it("parses the collected-cards list", async () => {
    const body = { cards: [
      { cardId: "concession", uses: 2, surfaces: ["project"], lastUsed: "2026-07-18T00:00:00Z" },
      { cardId: "opcvl", uses: 1, surfaces: ["course"], lastUsed: "2026-07-17T00:00:00Z" },
    ] };
    vi.spyOn(global, "fetch").mockResolvedValue(new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } }));
    const cards = await getGrowthCards();
    expect(cards.length).toBe(2);
    expect(cards[0]!.cardId).toBe("concession");
  });
});
