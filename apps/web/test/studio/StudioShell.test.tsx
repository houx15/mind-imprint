import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { StudioShell } from "@/studio/StudioShell";
import { STUDIO_FIXTURE } from "@/studio/fixtures";

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

  it("opening 装备栏 and clicking a card renders the methodology modal at the shell level (not clipped to the coach rail)", () => {
    render(<StudioShell state={STUDIO_FIXTURE} callbacks={noop} />);
    // composer toolbox toggle opens the 装备栏 popover
    fireEvent.click(screen.getByTitle("装备栏 · 工具卡"));
    // 让步段卡's meth key is "concession" (design ~L2287-2294)
    fireEvent.click(screen.getByText("让步段卡"));
    expect(screen.getByText("工具说明书 · 我不懂为什么")).toBeInTheDocument();
    expect(screen.getByText("让步段 · 以退为进")).toBeInTheDocument();
  });
});
