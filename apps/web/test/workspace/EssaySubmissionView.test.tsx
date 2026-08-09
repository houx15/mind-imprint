import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { EssaySubmissionView } from "@/workspace/blocks/EssaySubmissionGuide";
import type { ProposalGuideStep } from "@mind-imprint/contracts";

function step(over: Partial<ProposalGuideStep> = {}): ProposalGuideStep {
  return { key: "sub:intro", title: "引言", kind: "fixed", index: 0, total: 4, mode: "guided", started: true, subQuestions: [], card: { prompt: "写引言", example: "" }, steps: [], ...over };
}
const handlers = () => ({
  onChange: vi.fn(), onStart: vi.fn(), onStillStuck: vi.fn(), onDone: vi.fn(), onNext: vi.fn(), onPrev: vi.fn(),
  onCompose: vi.fn(), onGoToDraft: vi.fn(), onRequestFinish: vi.fn(),
});

describe("EssaySubmissionView", () => {
  it("ready gate → onStart", () => {
    const h = handlers();
    render(<EssaySubmissionView step={step({ started: false })} value="" {...h} />);
    expect(screen.getByText(/组装成一篇完整的论文/)).toBeTruthy();
    fireEvent.click(screen.getByText("准备好了，开始"));
    expect(h.onStart).toHaveBeenCalled();
  });

  it("引言 step renders a GuidedWritingCard; 我写好了 → onDone", () => {
    const h = handlers();
    render(<EssaySubmissionView step={step({ key: "sub:intro", title: "引言" })} value="" {...h} />);
    expect(screen.getByPlaceholderText("在这里写这一部分……")).toBeTruthy();
    fireEvent.click(screen.getByText("我写好了"));
    expect(h.onDone).toHaveBeenCalled();
  });

  it("成文 step → onCompose + onGoToDraft", () => {
    const h = handlers();
    render(<EssaySubmissionView step={step({ key: "sub:compose", title: "成文", index: 2 })} value="" {...h} />);
    fireEvent.click(screen.getByText("把各部分拼接成全文"));
    expect(h.onCompose).toHaveBeenCalled();
    fireEvent.click(screen.getByText("去正文里编辑"));
    expect(h.onGoToDraft).toHaveBeenCalled();
  });

  it("润色 step → onRequestFinish", () => {
    const h = handlers();
    render(<EssaySubmissionView step={step({ key: "sub:polish", title: "润色定稿", index: 3 })} value="" {...h} />);
    fireEvent.click(screen.getByText("完成整篇论文"));
    expect(h.onRequestFinish).toHaveBeenCalled();
  });
});
