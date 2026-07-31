import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CoachCardPanel } from "@/workspace/blocks/CoachCardPanel";

describe("CoachCardPanel", () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it("renders an AI card offer and consumes it on dismiss", () => {
    const onConsumed = vi.fn();
    render(
      <CoachCardPanel
        projectId="p1"
        proposal={{ cardId: "steelman", reason: "one_sided", nudgeText: "要不要打开「先说对方最强的版本」？" }}
        onProposalConsumed={onConsumed}
      />,
    );
    expect(screen.getByText(/要不要打开/)).toBeTruthy();
    // The chip has a 跳过 dismiss control; clicking it consumes the proposal.
    fireEvent.click(screen.getByRole("button", { name: "跳过" }));
    expect(onConsumed).toHaveBeenCalled();
  });

  it("self-summon: the 工具卡 toggle reveals the deck", () => {
    render(<CoachCardPanel projectId="p1" proposal={null} onProposalConsumed={() => {}} />);
    // Only the 工具卡 toggle is visible before opening the deck.
    expect(screen.getAllByRole("button")).toHaveLength(1);
    fireEvent.click(screen.getByRole("button", { name: /工具卡/ }));
    // The deck cards (the three thinking cards from CARD_REGISTRY) now show.
    expect(screen.getAllByRole("button").length).toBeGreaterThan(1);
  });
});
