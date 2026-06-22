import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ChatLog } from "./ChatLog";
import type { ChatItem } from "./viewModel";

// ─── fixtures ────────────────────────────────────────────────────────────────

const studentItem: ChatItem = {
  kind: "student",
  text: "Phoebe：中国是否让地球变得更可持续？请看这篇文章。",
  link: "https://www.nature.com/articles/s41893-019-0220-7",
};

const aiTextItem: ChatItem = {
  kind: "ai_text",
  text: "好的，我们先停一下。你引用的这条来源来自 NASA Earth Observatory，值得先用 SIFT 框架核一下它的可信度。",
};

const proposedItem: ChatItem = {
  kind: "proposal",
  cardInstanceId: "ci-sift",
  category: "信息素养",
  cardName: "SIFT×CRAAP 信息核查",
  nudge: "这儿先别急着写，我们用 SIFT 核一下这个来源？",
  status: "proposed",
};

const completedItem: ChatItem = {
  kind: "proposal",
  cardInstanceId: "ci-concession",
  category: "论证写作",
  cardName: "让步段写作",
  nudge: "我们来写让步段，承认中国碳排放是全球第一。",
  status: "completed",
};

const skippedItem: ChatItem = {
  kind: "proposal",
  cardInstanceId: "ci-skip",
  category: "信息素养",
  cardName: "SIFT×CRAAP 信息核查",
  nudge: "你也可以稍后再用 SIFT 核这个来源。",
  status: "skipped",
};

// ─── ChatLog tests ────────────────────────────────────────────────────────────

describe("ChatLog", () => {
  it("renders a student bubble on the right with message text", () => {
    render(<ChatLog items={[studentItem]} onOpenCard={vi.fn()} onSkipCard={vi.fn()} />);
    expect(screen.getByText(/Phoebe：中国是否让地球变得更可持续/)).toBeInTheDocument();
  });

  it("renders the link chip when student message contains a URL", () => {
    render(<ChatLog items={[studentItem]} onOpenCard={vi.fn()} onSkipCard={vi.fn()} />);
    expect(screen.getByText(/nature\.com\/articles/)).toBeInTheDocument();
  });

  it("renders an AI text bubble with avatar", () => {
    render(<ChatLog items={[aiTextItem]} onOpenCard={vi.fn()} onSkipCard={vi.fn()} />);
    expect(screen.getByText(/SIFT 框架核一下/)).toBeInTheDocument();
  });

  it("proposed: shows 建议工具卡 badge, category, cardName, nudge", () => {
    render(<ChatLog items={[proposedItem]} onOpenCard={vi.fn()} onSkipCard={vi.fn()} />);
    expect(screen.getByText("建议工具卡")).toBeInTheDocument();
    expect(screen.getByText("信息素养")).toBeInTheDocument();
    expect(screen.getByText("SIFT×CRAAP 信息核查")).toBeInTheDocument();
    expect(screen.getByText(/这儿先别急着写/)).toBeInTheDocument();
  });

  it("proposed: shows 打开卡 and 暂不，先继续 buttons", () => {
    render(<ChatLog items={[proposedItem]} onOpenCard={vi.fn()} onSkipCard={vi.fn()} />);
    expect(screen.getByRole("button", { name: /打开卡/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "暂不，先继续" })).toBeInTheDocument();
  });

  it("proposed: clicking 打开卡 calls onOpenCard with cardInstanceId", async () => {
    const onOpenCard = vi.fn();
    render(<ChatLog items={[proposedItem]} onOpenCard={onOpenCard} onSkipCard={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /打开卡/ }));
    expect(onOpenCard).toHaveBeenCalledWith("ci-sift");
  });

  it("proposed: clicking 暂不，先继续 calls onSkipCard with cardInstanceId", async () => {
    const onSkipCard = vi.fn();
    render(<ChatLog items={[proposedItem]} onOpenCard={vi.fn()} onSkipCard={onSkipCard} />);
    await userEvent.click(screen.getByRole("button", { name: "暂不，先继续" }));
    expect(onSkipCard).toHaveBeenCalledWith("ci-sift");
  });

  it("proposed: does NOT show completed or skipped UI", () => {
    render(<ChatLog items={[proposedItem]} onOpenCard={vi.fn()} onSkipCard={vi.fn()} />);
    expect(screen.queryByText(/已钉到过程树/)).not.toBeInTheDocument();
    expect(screen.queryByText(/已记录为信号/)).not.toBeInTheDocument();
  });

  it("completed: shows 已完成 · 已钉到过程树, no 打开卡 button", () => {
    render(<ChatLog items={[completedItem]} onOpenCard={vi.fn()} onSkipCard={vi.fn()} />);
    expect(screen.getByText(/已完成 · 已钉到过程树/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /打开卡/ })).not.toBeInTheDocument();
  });

  it("skipped: shows 已跳过（已记录为信号）and 仍可打开", () => {
    render(<ChatLog items={[skippedItem]} onOpenCard={vi.fn()} onSkipCard={vi.fn()} />);
    expect(screen.getByText(/已跳过（已记录为信号）/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "仍可打开" })).toBeInTheDocument();
  });

  it("skipped: clicking 仍可打开 calls onOpenCard with cardInstanceId", async () => {
    const onOpenCard = vi.fn();
    render(<ChatLog items={[skippedItem]} onOpenCard={onOpenCard} onSkipCard={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: "仍可打开" }));
    expect(onOpenCard).toHaveBeenCalledWith("ci-skip");
  });

  it("renders all three kinds of items in order", () => {
    render(
      <ChatLog
        items={[studentItem, aiTextItem, proposedItem]}
        onOpenCard={vi.fn()}
        onSkipCard={vi.fn()}
      />,
    );
    expect(screen.getByText(/Phoebe：中国是否让地球变得更可持续/)).toBeInTheDocument();
    expect(screen.getByText(/SIFT 框架核一下/)).toBeInTheDocument();
    expect(screen.getByText("SIFT×CRAAP 信息核查")).toBeInTheDocument();
  });

  it("empty items list renders without error", () => {
    const { container } = render(<ChatLog items={[]} onOpenCard={vi.fn()} onSkipCard={vi.fn()} />);
    expect(container.firstChild).toBeInTheDocument();
  });

  it("renders markdown in AI text as real elements, not raw markup", () => {
    const md: ChatItem = { kind: "ai_text", text: "先**停一下**：\n\n- 看看来源\n- 再核实" };
    render(<ChatLog items={[md]} onOpenCard={vi.fn()} onSkipCard={vi.fn()} />);
    // "停一下" must be inside a <strong>, and the "**" markers must be gone.
    const strong = screen.getByText("停一下");
    expect(strong.tagName).toBe("STRONG");
    expect(document.body.textContent).not.toContain("**");
    expect(screen.getByText("看看来源").closest("li")).toBeInTheDocument();
  });

  it("shows the 思考中 indicator only when thinking is true", () => {
    const { rerender } = render(
      <ChatLog items={[aiTextItem]} onOpenCard={vi.fn()} onSkipCard={vi.fn()} />,
    );
    expect(screen.queryByText("思考中")).not.toBeInTheDocument();
    rerender(<ChatLog items={[aiTextItem]} onOpenCard={vi.fn()} onSkipCard={vi.fn()} thinking />);
    expect(screen.getByText("思考中")).toBeInTheDocument();
  });
});
