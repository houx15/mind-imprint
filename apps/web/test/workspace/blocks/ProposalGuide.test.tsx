import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import type { ProposalGuideStep } from "@mind-imprint/contracts";
import { ProposalGuideReadOnly } from "@/workspace/blocks/ProposalGuide";

// Task 5 (P7 guided-tour, demo slice) · when the writing room is LOCKED (a
// finished proposal, or the read-only demo project), ProposalGuidePane must NOT
// fall back to an empty pane — the guided tour spotlights `writing-aicard` and
// needs a real, populated guide to frame. ProposalGuideReadOnly renders the
// guide's own structure (sub-questions + every part's guiding prompt/example)
// with the student's own saved text, every input disabled/read-only.
describe("ProposalGuideReadOnly (Task 5, P7)", () => {
  const step: ProposalGuideStep = {
    key: "polish",
    title: "写成稿 · 通读与润色",
    kind: "fixed",
    index: 8,
    total: 9,
    mode: "guided",
    started: true,
    subQuestions: [
      { id: "q1", text: "中国的碳排放总量近十年如何变化？" },
      { id: "q2", text: "中国的可再生能源投资规模有多大？" },
    ],
    card: null,
    steps: [
      { key: "understanding", title: "对题目的理解", kind: "fixed", card: { prompt: "题目问的是什么？先复述一遍。", example: "e.g. this asks whether..." } },
      { key: "subq-define", title: "研究计划", kind: "subq-define" },
      { key: "q1", title: "子问题 1：碳排放", kind: "subq", card: { prompt: "这个子问题打算怎么回答？", example: "e.g. using emissions data..." } },
    ],
  };

  it("renders the subquestion structure + guiding prompts (not an empty pane)", () => {
    render(<ProposalGuideReadOnly step={step} textByKey={{ understanding: "我对题目的理解是……", q1: "" }} />);
    // sub-question structure.
    expect(screen.getByText("研究计划的子问题")).toBeInTheDocument();
    expect(screen.getByText("中国的碳排放总量近十年如何变化？")).toBeInTheDocument();
    expect(screen.getByText("中国的可再生能源投资规模有多大？")).toBeInTheDocument();
    // guiding prompts (the "可填的引导框"), one card per non-define step.
    expect(screen.getByText("对题目的理解")).toBeInTheDocument();
    expect(screen.getByText("题目问的是什么？先复述一遍。")).toBeInTheDocument();
    expect(screen.getByText("e.g. this asks whether...")).toBeInTheDocument();
    expect(screen.getByText("子问题 1：碳排放")).toBeInTheDocument();
    expect(screen.getByText("这个子问题打算怎么回答？")).toBeInTheDocument();
    // the subq-define step itself never renders its own card.
    expect(screen.queryByText("研究计划", { selector: "h3" })).toBeNull();
  });

  it("shows the student's own saved text, and every textarea is read-only", () => {
    render(<ProposalGuideReadOnly step={step} textByKey={{ understanding: "我对题目的理解是……", q1: "" }} />);
    const boxes = screen.getAllByRole("textbox") as HTMLTextAreaElement[];
    expect(boxes.length).toBe(2); // one per non-define step
    expect(boxes.every((b) => b.readOnly)).toBe(true);
    expect(boxes.map((b) => b.value)).toContain("我对题目的理解是……");
  });

  it("renders nothing when there is no step yet (still loading)", () => {
    const { container } = render(<ProposalGuideReadOnly step={null} textByKey={{}} />);
    expect(container.firstChild).toBeNull();
  });
});
