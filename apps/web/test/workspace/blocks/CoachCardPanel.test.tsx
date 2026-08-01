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

  it("self-summon: the card shelf is always visible (#8), defaulting to the thinking deck", () => {
    render(<CoachCardPanel projectId="p1" proposal={null} onProposalConsumed={() => {}} />);
    // #8 · the deck is no longer hidden behind a 工具卡 toggle — the three
    // default thinking cards show as tappable chips immediately.
    expect(screen.getByRole("button", { name: "事实/观点/价值判断卡" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "确定度光谱卡" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "让步段·反方最强卡" })).toBeTruthy();
  });

  it("honors a phase-scoped deck (#17/#18)", () => {
    render(<CoachCardPanel projectId="p1" proposal={null} onProposalConsumed={() => {}} deck={["question-card", "search-plan"]} />);
    expect(screen.getByRole("button", { name: "提问卡" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "检索方向审视" })).toBeTruthy();
    // a card outside the passed deck is not offered here
    expect(screen.queryByRole("button", { name: "确定度光谱卡" })).toBeNull();
  });
});
