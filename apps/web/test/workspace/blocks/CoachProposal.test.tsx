import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CoachProposal } from "@/workspace/blocks/CoachProposal";

// S4 · the cross-phase card OFFER chip. It must render the coach's nudge, open
// ONLY on the student's tap (never auto-open — 克制/铁律), and dismiss on 跳过.
describe("CoachProposal", () => {
  const proposal = { cardId: "sift", reason: "横向核查这条来源", nudgeText: "要不要打开 SIFT？" };

  it("renders the nudge and opens on 打开", () => {
    const onOpen = vi.fn();
    const onDismiss = vi.fn();
    render(<CoachProposal proposal={proposal} onOpen={onOpen} onDismiss={onDismiss} />);
    expect(screen.getByText(/SIFT/)).toBeInTheDocument();
    expect(screen.getByText(/横向核查/)).toBeInTheDocument();
    // 克制: nothing opens on render.
    expect(onOpen).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "打开" }));
    expect(onOpen).toHaveBeenCalledWith("sift");
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
