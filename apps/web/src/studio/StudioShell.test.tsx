import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { StudioShell } from "./StudioShell";
import { STUDIO_FIXTURE } from "./fixtures";

const noop = { onSelectStation: () => {}, onToggleFocus: () => {}, onDisposition: () => {}, onOpenMethodology: () => {}, onComposerSend: () => {} };

describe("StudioShell", () => {
  it("renders the three regions: station rail, center view, coach rail", () => {
    render(<StudioShell state={STUDIO_FIXTURE} callbacks={noop} />);
    expect(screen.getByText("任务解码")).toBeInTheDocument();       // rail
    // "论证构建" legitimately renders twice: once as the S4 row in the
    // station rail, once as the active station's header in the center view.
    expect(screen.getAllByText("论证构建").length).toBe(2);
    expect(screen.getByText("AI 陪练")).toBeInTheDocument();        // coach rail
  });
  it("专注模式 toggle fires onToggleFocus", () => {
    const onToggleFocus = vi.fn();
    render(<StudioShell state={STUDIO_FIXTURE} callbacks={{ ...noop, onToggleFocus }} />);
    fireEvent.click(screen.getByText(/专注/));
    expect(onToggleFocus).toHaveBeenCalled();
  });
});
