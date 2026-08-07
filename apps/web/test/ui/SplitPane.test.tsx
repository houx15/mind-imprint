import { describe, it, expect, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SplitPane } from "@/ui/SplitPane";

describe("SplitPane", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("renders both panes", () => {
    render(<SplitPane left={<div>左边</div>} right={<div>右边</div>} />);
    expect(screen.getByText("左边")).toBeInTheDocument();
    expect(screen.getByText("右边")).toBeInTheDocument();
  });

  it("exposes a keyboard-resizable separator with the ratio as aria-valuenow", () => {
    render(<SplitPane left={<div>L</div>} right={<div>R</div>} defaultRatio={0.4} />);
    const sep = screen.getByRole("separator");
    expect(sep).toHaveAttribute("aria-valuenow", "40");
    expect(sep).toHaveAttribute("aria-valuemin", "20");
    expect(sep).toHaveAttribute("aria-valuemax", "80");
  });

  it("ArrowRight widens the left pane, ArrowLeft narrows it (2% steps)", async () => {
    const user = userEvent.setup();
    render(<SplitPane left={<div>L</div>} right={<div>R</div>} defaultRatio={0.4} />);
    const sep = screen.getByRole("separator");
    sep.focus();
    await user.keyboard("{ArrowRight}");
    expect(sep).toHaveAttribute("aria-valuenow", "42");
    await user.keyboard("{ArrowLeft}{ArrowLeft}");
    expect(sep).toHaveAttribute("aria-valuenow", "38");
  });

  it("clamps the ratio to the 20–80% band", async () => {
    const user = userEvent.setup();
    render(<SplitPane left={<div>L</div>} right={<div>R</div>} defaultRatio={0.79} />);
    const sep = screen.getByRole("separator");
    sep.focus();
    // two +2% steps from 79% would reach 83% → clamp at 80%
    await user.keyboard("{ArrowRight}{ArrowRight}");
    expect(sep).toHaveAttribute("aria-valuenow", "80");
  });

  it("persists the ratio to storageKey and restores it on remount", async () => {
    const user = userEvent.setup();
    const { unmount } = render(
      <SplitPane left={<div>L</div>} right={<div>R</div>} defaultRatio={0.4} storageKey="mk-test-split" />,
    );
    const sep = screen.getByRole("separator");
    sep.focus();
    await user.keyboard("{ArrowRight}"); // 40 → 42
    expect(localStorage.getItem("mk-test-split")).toBe("0.4200");
    unmount();

    render(<SplitPane left={<div>L</div>} right={<div>R2</div>} defaultRatio={0.4} storageKey="mk-test-split" />);
    expect(screen.getByRole("separator")).toHaveAttribute("aria-valuenow", "42");
  });
});
