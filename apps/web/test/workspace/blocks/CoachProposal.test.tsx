import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CoachProposal } from "@/workspace/blocks/CoachProposal";

// S4 · the cross-phase card OFFER chip. It must render the coach's nudge, open
// ONLY on the student's tap (never auto-open — 克制/铁律), and dismiss on 跳过.
describe("CoachProposal", () => {
  // A real registry card id (spec §13: the chip leads with 卡图标 + 卡名).
  const proposal = { cardId: "concession", reason: "横向核查这条来源", nudgeText: "要不要打开「先说对方最强的版本」？" };

  it("renders the card name + nudge and opens on 打开", () => {
    const onOpen = vi.fn();
    const onDismiss = vi.fn();
    render(<CoachProposal proposal={proposal} onOpen={onOpen} onDismiss={onDismiss} />);
    // 卡名 (from CARD_REGISTRY) leads the chip.
    expect(screen.getByText("让步段 · 以退为进")).toBeInTheDocument();
    expect(screen.getByText(/先说对方最强的版本/)).toBeInTheDocument();
    expect(screen.getByText(/横向核查/)).toBeInTheDocument();
    // 克制: nothing opens on render.
    expect(onOpen).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "打开" }));
    expect(onOpen).toHaveBeenCalledWith("concession");
  });

  it("dismisses on 跳过 without opening", () => {
    const onOpen = vi.fn();
    const onDismiss = vi.fn();
    render(<CoachProposal proposal={proposal} onOpen={onOpen} onDismiss={onDismiss} />);
    fireEvent.click(screen.getByRole("button", { name: "跳过" }));
    expect(onDismiss).toHaveBeenCalled();
    expect(onOpen).not.toHaveBeenCalled();
  });
});
