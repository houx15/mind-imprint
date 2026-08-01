import { describe, it, expect, vi, afterEach } from "vitest";
import { getGrowthCards, getCardsCatalog } from "@/api/cards";

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

describe("getCardsCatalog", () => {
  it("parses the catalog + theme", async () => {
    const body = { theme: "cyber-sage", cards: [
      { cardId: "craap", name: "CRAAP", nameEn: "CRAAP", category: "信息素养", purpose: "p",
        stages: ["阅读"], example: "", hasAsset: true, coverUrl: "https://cdn/x.webp", courseId: "c1",
        encountered: true, score: 45, stars: 3, uses: 1, surfaces: ["project"], lastUsed: "2026-07-23T00:00:00Z" },
    ] };
    vi.spyOn(global, "fetch").mockResolvedValue(new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } }));
    const res = await getCardsCatalog("cyber-sage");
    expect(res.theme).toBe("cyber-sage");
    expect(res.cards[0]!.stars).toBe(3);
    expect(res.cards[0]!.coverUrl).toBe("https://cdn/x.webp");
  });
});
