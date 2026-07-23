import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProposalBubble } from "@/cards/states/ProposalBubble";

const base = { category: "信息素养", name: "SIFT×CRAAP 信息核查", nudge: "这儿先别急着写，我们用 SIFT 核一下这个来源？" };

describe("ProposalBubble", () => {
  it("proposed: shows 打开卡 and 暂不，先继续 and wires callbacks", async () => {
    const onOpen = vi.fn(); const onSkip = vi.fn();
    render(<ProposalBubble status="proposed" {...base} onOpen={onOpen} onSkip={onSkip} />);
    await userEvent.click(screen.getByRole("button", { name: "打开卡" }));
    await userEvent.click(screen.getByRole("button", { name: "暂不，先继续" }));
    expect(onOpen).toHaveBeenCalledOnce();
    expect(onSkip).toHaveBeenCalledOnce();
  });
  it("completed: shows the pinned-to-tree confirmation, no 打开卡", () => {
    render(<ProposalBubble status="completed" {...base} onOpen={vi.fn()} onSkip={vi.fn()} />);
    expect(screen.getByText(/已钉到过程树/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "打开卡" })).not.toBeInTheDocument();
  });
  it("skipped: shows the recorded-as-signal note and 仍可打开 wires onOpen", async () => {
    const onOpen = vi.fn();
    render(<ProposalBubble status="skipped" {...base} onOpen={onOpen} onSkip={vi.fn()} />);
    expect(screen.getByText(/已记录为信号/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "仍可打开" }));
    expect(onOpen).toHaveBeenCalledOnce();
  });
});
