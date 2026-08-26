import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { SelectionEval } from "@mind-imprint/contracts";
import { HangingCard, anchorBlockId } from "@/studio/reading/HangingCard";

describe("anchorBlockId (the anchor-move)", () => {
  it("hangs under the AI example while proposed", () => {
    expect(anchorBlockId("bEx", "bStu", "proposed")).toBe("bEx");
  });
  it("moves under the student's block once picking/active", () => {
    expect(anchorBlockId("bEx", "bStu", "active")).toBe("bStu");
    expect(anchorBlockId("bEx", "bStu", "feedback")).toBe("bStu");
  });
  it("stays on the example if the student has not picked yet", () => {
    expect(anchorBlockId("bEx", null, "active")).toBe("bEx");
  });
});

const EVAL: SelectionEval = {
  verdict: "strong",
  verdictLabel: "有力证据",
  verdictReason: "这句话直接支撑了论点。",
  checks: [
    { key: "relevance", label: "相关性", status: "pass", evidence: "……", explanation: "与论点直接相关。" },
    { key: "authority", label: "权威性", status: "pass", evidence: "……", explanation: "来自权威机构。" },
    { key: "specificity", label: "具体性", status: "partial", evidence: "……", explanation: "可以更具体一些。" },
  ],
  finding: "这句话很有力。",
  judgment: "可以用作核心证据。",
  support: "论点得到支撑。",
  caveat: "留意上下文。",
  nextStep: "把这句话接到你的论证段落里。",
  spanIds: ["s1"],
};

describe("HangingCard", () => {
  it("proposed: shows the primary button and calls onStartPick", () => {
    const onStartPick = vi.fn();
    render(
      <HangingCard
        cardName="证据溯源"
        status="proposed"
        exampleWhy="AI 选的这句话展示了如何找到支撑论点的证据。"
        onStartPick={onStartPick}
        onConfirm={() => {}}
        onRepick={() => {}}
      />
    );
    fireEvent.click(screen.getByText("看懂示范，开始选句"));
    expect(onStartPick).toHaveBeenCalled();
  });

  it("feedback: renders the verdict label, nextStep, and the confirm button", () => {
    render(
      <HangingCard
        cardName="证据溯源"
        status="feedback"
        exampleWhy="AI 选的这句话展示了如何找到支撑论点的证据。"
        eval={EVAL}
        onStartPick={() => {}}
        onConfirm={() => {}}
        onRepick={() => {}}
      />
    );
    expect(screen.getByText("有力证据")).toBeInTheDocument();
    expect(screen.getByText(/把这句话接到你的论证段落里/)).toBeInTheDocument();
    expect(screen.getByText("记下这条发现")).toBeInTheDocument();
  });

  it("proposed: offers 跳过这副透镜 and calls onSkip — the deadlock escape hatch", () => {
    const onSkip = vi.fn();
    render(
      <HangingCard
        cardName="证据溯源"
        status="proposed"
        exampleWhy="AI 选的这句话展示了如何找到支撑论点的证据。"
        onStartPick={() => {}}
        onConfirm={() => {}}
        onRepick={() => {}}
        onSkip={onSkip}
      />
    );
    fireEvent.click(screen.getByText("跳过这副透镜"));
    expect(onSkip).toHaveBeenCalled();
  });

  it("active: also offers 跳过这副透镜 and calls onSkip", () => {
    const onSkip = vi.fn();
    render(
      <HangingCard
        cardName="证据溯源"
        status="active"
        exampleWhy="AI 选的这句话展示了如何找到支撑论点的证据。"
        onStartPick={() => {}}
        onConfirm={() => {}}
        onRepick={() => {}}
        onSkip={onSkip}
      />
    );
    fireEvent.click(screen.getByText("跳过这副透镜"));
    expect(onSkip).toHaveBeenCalled();
  });

  it("active: shows the default instruction when there is no pickHint", () => {
    render(
      <HangingCard
        cardName="证据溯源"
        status="active"
        exampleWhy="AI 选的这句话展示了如何找到支撑论点的证据。"
        onStartPick={() => {}}
        onConfirm={() => {}}
        onRepick={() => {}}
      />
    );
    expect(screen.getByText("在文章里点出你自己的证据句")).toBeInTheDocument();
  });

  it("active: a pickHint (D1) replaces the default instruction, in the same voice/styling", () => {
    render(
      <HangingCard
        cardName="证据溯源"
        status="active"
        exampleWhy="AI 选的这句话展示了如何找到支撑论点的证据。"
        onStartPick={() => {}}
        onConfirm={() => {}}
        onRepick={() => {}}
        pickHint="这句是示范句——换一句你自己的证据句。"
      />
    );
    expect(screen.getByText("这句是示范句——换一句你自己的证据句。")).toBeInTheDocument();
    expect(screen.queryByText("在文章里点出你自己的证据句")).not.toBeInTheDocument();
  });

  it("without onSkip, no skip control renders (optional prop stays optional)", () => {
    render(
      <HangingCard
        cardName="证据溯源"
        status="proposed"
        exampleWhy="AI 选的这句话展示了如何找到支撑论点的证据。"
        onStartPick={() => {}}
        onConfirm={() => {}}
        onRepick={() => {}}
      />
    );
    expect(screen.queryByText("跳过这副透镜")).not.toBeInTheDocument();
  });
});
