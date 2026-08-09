import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ProposalGuide } from "@/workspace/blocks/ProposalGuide";
import type { ProposalGuideStep } from "@mind-imprint/contracts";

function step(over: Partial<ProposalGuideStep> = {}): ProposalGuideStep {
  return {
    key: "understanding",
    title: "对题目的理解",
    kind: "fixed",
    index: 0,
    total: 9,
    mode: "guided",
    started: true,
    subQuestions: [],
    card: { prompt: "解释你对题目的理解", example: "An English example" },
    steps: [],
    ...over,
  };
}

const noop = () => {};
function handlers() {
  return {
    onChooseMode: vi.fn(),
    onStart: vi.fn(),
    onSaveSubQuestions: vi.fn(),
    onNext: vi.fn(),
    onPrev: vi.fn(),
    onStillStuck: vi.fn(),
    onDone: vi.fn(),
    onOpenReading: vi.fn(),
  };
}

describe("ProposalGuide", () => {
  it("mode='' renders the choice gate; picking guided calls onChooseMode", () => {
    const h = handlers();
    render(<ProposalGuide step={step({ mode: "", started: false, card: null })} bufferNonEmpty={false} reviewing={false} {...h} />);
    fireEvent.click(screen.getByText("一步步带我写"));
    expect(h.onChooseMode).toHaveBeenCalledWith("guided");
  });

  it("guided+not-started renders the outline intro + 开始写作", () => {
    const h = handlers();
    render(<ProposalGuide step={step({ started: false, card: null })} bufferNonEmpty={false} reviewing={false} {...h} />);
    expect(screen.getByText(/一份扎实的提案/)).toBeTruthy();
    fireEvent.click(screen.getByText("开始写作"));
    expect(h.onStart).toHaveBeenCalled();
  });

  it("guided guide card: 我依然有问题 and 我写好了 fire", () => {
    const h = handlers();
    render(<ProposalGuide step={step()} bufferNonEmpty={false} reviewing={false} {...h} />);
    expect(screen.getByText("解释你对题目的理解")).toBeTruthy();
    expect(screen.getByText("An English example")).toBeTruthy();
    fireEvent.click(screen.getByText("我依然有问题"));
    expect(h.onStillStuck).toHaveBeenCalled();
    fireEvent.click(screen.getByText("我写好了"));
    expect(h.onDone).toHaveBeenCalled();
  });

  it("subq-define step renders the sub-question editor; 确认 needs ≥2", () => {
    const h = handlers();
    render(
      <ProposalGuide
        step={step({ key: "research-plan", title: "研究计划 · 定子问题", kind: "subq-define", index: 3, card: { prompt: "拆成 2–4 个子问题", example: "" } })}
        bufferNonEmpty={false}
        reviewing={false}
        {...h}
      />,
    );
    const confirm = screen.getByText("确认子问题") as HTMLButtonElement;
    expect(confirm.disabled).toBe(true); // no sub-questions yet
    const inputs = screen.getAllByPlaceholderText("写一个子问题……");
    fireEvent.change(inputs[0]!, { target: { value: "子问题一" } });
    fireEvent.change(inputs[1]!, { target: { value: "子问题二" } });
    expect((screen.getByText("确认子问题") as HTMLButtonElement).disabled).toBe(false);
    fireEvent.click(screen.getByText("确认子问题"));
    expect(h.onSaveSubQuestions).toHaveBeenCalledWith([
      { id: "", text: "子问题一" },
      { id: "", text: "子问题二" },
    ]);

    fireEvent.click(screen.getByText("先去做文献探索"));
    expect(h.onOpenReading).toHaveBeenCalled();
  });
});
