import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ChatLog, type ChatMessage } from "@/studio/ai/ChatLog";

describe("ChatLog", () => {
  it("renders an assistant bubble: white surface, radius 4/13/13/13, left-aligned", () => {
    const messages: ChatMessage[] = [{ id: "1", role: "assistant", text: "你想先从哪个来源查起？" }];
    render(<ChatLog messages={messages} />);
    const bubble = screen.getByText("你想先从哪个来源查起？");
    expect(bubble.className).toContain("bg-mk-surface");
    expect(bubble.className).toContain("rounded-[4px_13px_13px_13px]");
    expect(bubble.className).toContain("text-mk-ink");
    const row = bubble.parentElement as HTMLElement;
    expect(row.className).toContain("justify-start");
  });

  it("renders a student bubble: accent-50, radius 13/4/13/13, right-aligned", () => {
    const messages: ChatMessage[] = [{ id: "1", role: "student", text: "我觉得这个来源不太可靠" }];
    render(<ChatLog messages={messages} />);
    const bubble = screen.getByText("我觉得这个来源不太可靠");
    expect(bubble.className).toContain("bg-mk-accent-50");
    expect(bubble.className).toContain("rounded-[13px_4px_13px_13px]");
    const row = bubble.parentElement as HTMLElement;
    expect(row.className).toContain("justify-end");
  });

  it("renders a system message centered with caption/muted text", () => {
    const messages: ChatMessage[] = [{ id: "1", role: "system", text: "已切换到写作房间" }];
    render(<ChatLog messages={messages} />);
    const line = screen.getByText("已切换到写作房间");
    expect(line.className).toContain("text-center");
    expect(line.className).toContain("text-mk-caption");
    expect(line.className).toContain("text-mk-muted");
  });

  it("renders a message's node inline", () => {
    const messages: ChatMessage[] = [
      {
        id: "1",
        role: "assistant",
        node: <span data-testid="card-chip">CRAAP 卡</span>,
      },
    ];
    render(<ChatLog messages={messages} />);
    expect(screen.getByTestId("card-chip")).toBeInTheDocument();
  });

  it("shows the thinking dot indicator when thinking is true", () => {
    const { container } = render(<ChatLog messages={[]} thinking />);
    expect(container.querySelectorAll(".mk-think-dot")).toHaveLength(3);
    expect(screen.getByLabelText("印记正在打字")).toBeInTheDocument();
  });

  it("does not show the thinking indicator by default", () => {
    const { container } = render(<ChatLog messages={[]} />);
    expect(container.querySelectorAll(".mk-think-dot")).toHaveLength(0);
  });
});
