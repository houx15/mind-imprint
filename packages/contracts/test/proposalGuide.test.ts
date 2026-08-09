import { describe, it, expect } from "vitest";
import { ProposalGuideStep, QuestionCardTurnReply } from "../src/index";

describe("ProposalGuideStep", () => {
  it("parses a guided subq step with a card", () => {
    const parsed = ProposalGuideStep.parse({
      key: "subq:a",
      title: "子问题 1",
      kind: "subq",
      index: 4,
      total: 12,
      mode: "guided",
      started: true,
      subQuestions: [{ id: "a", text: "q1" }],
      card: { prompt: "这个子问题要解决什么？", example: "For a prompt about X, ...", refHint: "framework 目标" },
    });
    expect(parsed.card?.prompt).toContain("子问题");
    expect(parsed.subQuestions).toHaveLength(1);
  });

  it("parses a define step with card null", () => {
    const parsed = ProposalGuideStep.parse({
      key: "research-plan",
      title: "研究计划 · 定子问题",
      kind: "subq-define",
      index: 3,
      total: 9,
      mode: "guided",
      started: false,
      subQuestions: [],
      card: null,
    });
    expect(parsed.card).toBeNull();
    expect(parsed.mode).toBe("guided");
  });

  it("parses an unchosen-mode step", () => {
    const parsed = ProposalGuideStep.parse({
      key: "understanding",
      title: "对题目的理解",
      kind: "fixed",
      index: 0,
      total: 9,
      mode: "",
      started: false,
      subQuestions: [],
      card: null,
    });
    expect(parsed.mode).toBe("");
  });
});

describe("QuestionCardTurnReply", () => {
  it("parses a mid-conversation turn (null objective, not done)", () => {
    const parsed = QuestionCardTurnReply.parse({
      narrate: "用你自己的话说说你对题目的理解？",
      suggestedObjective: null,
      done: false,
    });
    expect(parsed.done).toBe(false);
    expect(parsed.suggestedObjective).toBeNull();
  });

  it("parses a final turn that fills the objective", () => {
    const parsed = QuestionCardTurnReply.parse({
      narrate: "很好，这就是你的研究问题。",
      suggestedObjective: "在 X 条件下 Y 是否影响 Z",
      done: true,
    });
    expect(parsed.done).toBe(true);
    expect(parsed.suggestedObjective).toContain("X");
  });
});
