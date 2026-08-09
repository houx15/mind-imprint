import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { EssayStatementView } from "@/workspace/blocks/EssayStatementGuide";
import type { ProposalGuideStep } from "@mind-imprint/contracts";

function step(over: Partial<ProposalGuideStep> = {}): ProposalGuideStep {
  return { key: "outline", title: "搭大纲", kind: "fixed", index: 0, total: 7, mode: "guided", started: true, subQuestions: [], card: { prompt: "调整大纲", example: "" }, steps: [], ...over };
}
const handlers = () => ({ onChange: vi.fn(), onStart: vi.fn(), onStillStuck: vi.fn(), onDone: vi.fn(), onNext: vi.fn(), onPrev: vi.fn(), onOfferCard: vi.fn() });

describe("EssayStatementView", () => {
  it("ready gate shows the plan intro + starts the walk", () => {
    const h = handlers();
    render(<EssayStatementView step={step({ started: false })} value="" {...h} />);
    expect(screen.getByText(/接下来我们开始写正文/)).toBeTruthy();
    fireEvent.click(screen.getByText("准备好了，开始写作"));
    expect(h.onStart).toHaveBeenCalled();
  });

  it("outline step advances to writing without a textarea", () => {
    const h = handlers();
    render(<EssayStatementView step={step({ key: "outline" })} value="" {...h} />);
    expect(screen.queryByPlaceholderText("在这里写这一段……")).toBeNull();
    fireEvent.click(screen.getByText("大纲调好了，开始写论点"));
    expect(h.onNext).toHaveBeenCalled();
  });

  it("a claim step renders a GuidedWritingCard with the 写作卡 offer", () => {
    const h = handlers();
    render(<EssayStatementView step={step({ key: "claim:a", title: "论点 1", kind: "subq", index: 1, card: { prompt: "写这条论点", example: "ex" } })} value="" {...h} />);
    expect(screen.getByText("写这条论点")).toBeTruthy();
    expect(screen.getByPlaceholderText("在这里写这一段……")).toBeTruthy();
    fireEvent.click(screen.getByText("PEE 写作卡"));
    expect(h.onOfferCard).toHaveBeenCalledWith("pee");
    fireEvent.click(screen.getByText("我写好了"));
    expect(h.onDone).toHaveBeenCalled();
  });
});
