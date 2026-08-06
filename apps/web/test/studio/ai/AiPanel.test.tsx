import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AiPanel } from "@/studio/ai/AiPanel";

describe("AiPanel", () => {
  it("expanded: shows title, children, flip button, and collapse button", () => {
    render(
      <AiPanel side="right" onFlip={vi.fn()} collapsed={false} onToggleCollapse={vi.fn()}>
        <div data-testid="coach-content">陪练内容</div>
      </AiPanel>,
    );
    expect(screen.getByText("印记")).toBeInTheDocument();
    expect(screen.getByTestId("coach-content")).toBeInTheDocument();
    expect(screen.getByLabelText("切换 AI 面板左右")).toBeInTheDocument();
    expect(screen.getByLabelText("折叠 AI 面板")).toBeInTheDocument();
  });

  it("uses a custom title when provided", () => {
    render(
      <AiPanel side="right" onFlip={vi.fn()} collapsed={false} onToggleCollapse={vi.fn()} title="陪练">
        <div>内容</div>
      </AiPanel>,
    );
    expect(screen.getByText("陪练")).toBeInTheDocument();
  });

  it("fires onFlip when the flip button is clicked", async () => {
    const user = userEvent.setup();
    const onFlip = vi.fn();
    render(
      <AiPanel side="right" onFlip={onFlip} collapsed={false} onToggleCollapse={vi.fn()}>
        <div>内容</div>
      </AiPanel>,
    );
    await user.click(screen.getByLabelText("切换 AI 面板左右"));
    expect(onFlip).toHaveBeenCalledTimes(1);
  });

  it("fires onToggleCollapse when the collapse button is clicked", async () => {
    const user = userEvent.setup();
    const onToggleCollapse = vi.fn();
    render(
      <AiPanel side="right" onFlip={vi.fn()} collapsed={false} onToggleCollapse={onToggleCollapse}>
        <div>内容</div>
      </AiPanel>,
    );
    await user.click(screen.getByLabelText("折叠 AI 面板"));
    expect(onToggleCollapse).toHaveBeenCalledTimes(1);
  });

  it("collapsed: hides children, shows only the strip + expand button", async () => {
    const user = userEvent.setup();
    const onToggleCollapse = vi.fn();
    render(
      <AiPanel side="right" onFlip={vi.fn()} collapsed onToggleCollapse={onToggleCollapse}>
        <div data-testid="coach-content">陪练内容</div>
      </AiPanel>,
    );
    expect(screen.queryByTestId("coach-content")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("切换 AI 面板左右")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("折叠 AI 面板")).not.toBeInTheDocument();
    const expandButton = screen.getByLabelText("展开 AI 面板");
    expect(expandButton).toBeInTheDocument();
    await user.click(expandButton);
    expect(onToggleCollapse).toHaveBeenCalledTimes(1);
  });

  it("side='right' puts the inner border on the left edge (border-l)", () => {
    const { container } = render(
      <AiPanel side="right" onFlip={vi.fn()} collapsed={false} onToggleCollapse={vi.fn()}>
        <div>内容</div>
      </AiPanel>,
    );
    const panel = container.firstElementChild as HTMLElement;
    expect(panel.className).toContain("border-l");
    expect(panel.className).not.toContain("border-r");
  });

  it("side='left' puts the inner border on the right edge (border-r)", () => {
    const { container } = render(
      <AiPanel side="left" onFlip={vi.fn()} collapsed={false} onToggleCollapse={vi.fn()}>
        <div>内容</div>
      </AiPanel>,
    );
    const panel = container.firstElementChild as HTMLElement;
    expect(panel.className).toContain("border-r");
    expect(panel.className).not.toContain("border-l");
  });
});
