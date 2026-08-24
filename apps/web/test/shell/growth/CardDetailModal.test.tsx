import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { CardDetailModal } from "@/shell/growth/CardDetailModal";
import { CARD_REGISTRY } from "@mind-imprint/contracts";

// knower-perspective is authored with **bold** in its tagline, step details and
// watchOut — the three teaching fields that used to be dropped in as raw
// strings, so students read literal asterisks.
const CARD_ID = "knower-perspective";
const spec = CARD_REGISTRY[CARD_ID]!;

const ENTRY = {
  cardId: CARD_ID, name: spec.name, nameEn: "Knower Perspective", category: "自我认知",
  purpose: "看清你自己的位置", stages: ["反思"], example: "", hasAsset: true,
  coverUrl: "", courseId: "",
  encountered: true, score: 70, stars: 4, uses: 3, surfaces: ["project"], lastUsed: "2026-08-01T00:00:00Z",
};

function open() {
  render(<CardDetailModal c={ENTRY as never} courses={[]} onClose={vi.fn()} />);
}

describe("CardDetailModal teaching markdown", () => {
  it("renders **bold** in the tagline as real emphasis, not literal asterisks", () => {
    open();
    const teaching = spec.teaching!;
    const marked = /\*\*(.+?)\*\*/.exec(teaching.tagline)![1]!;

    // The emphasised phrase is a <strong>, and the asterisks are gone.
    const strong = screen.getAllByText(marked).find((el) => el.tagName === "STRONG");
    expect(strong).toBeDefined();
    expect(document.body.textContent).not.toContain(`**${marked}**`);
  });

  it("renders markdown in step details and 最容易错 too", () => {
    open();
    const teaching = spec.teaching!;

    const detail = teaching.steps!.find((s) => s.detail.includes("**"))!.detail;
    const inDetail = /\*\*(.+?)\*\*/.exec(detail)![1]!;
    expect(screen.getAllByText(inDetail).some((el) => el.tagName === "STRONG")).toBe(true);

    const watch = /\*\*(.+?)\*\*/.exec(teaching.watchOut!)![1]!;
    expect(screen.getAllByText(watch).some((el) => el.tagName === "STRONG")).toBe(true);

    // Nothing anywhere in the modal still shows raw ** markup.
    expect(document.body.textContent).not.toMatch(/\*\*/);
  });

  it("keeps each teaching block's own typography (inline render, no <p> wrapper)", () => {
    open();
    const teaching = spec.teaching!;
    const marked = /\*\*(.+?)\*\*/.exec(teaching.tagline)![1]!;
    const strong = screen.getAllByText(marked).find((el) => el.tagName === "STRONG")!;

    // The tagline stays a single styled <p>; the markdown must not have nested
    // another block element inside it.
    const p = strong.closest("p")!;
    expect(p).not.toBeNull();
    expect(p.className).toContain("font-semibold");
    expect(within(p).queryByRole("list")).toBeNull();
  });
});
