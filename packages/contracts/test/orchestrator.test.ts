import { describe, it, expect } from "vitest";
import {
  StudioState,
  OrchestratorReply,
  OpenTool,
  OrchestratorTool,
  CoachHistoryMsg,
  CoachHistoryPage,
} from "../src/orchestrator";

describe("orchestrator contract", () => {
  it("accepts a full directive reply", () => {
    const reply = {
      narrate: "提案四点齐了 — 写作面板给你开好了。",
      directive: {
        stage: "body_writing",
        openTool: "writing",
        widthTier: "wide",
        reference: [{ kind: "material", id: "m1", label: "NASA 报告" }],
        updatedAtTurn: 3,
        started: true,
      },
      note: null,
      card: null,
      question: null,
      reviewRequested: false,
      planGenerated: false,
      compacted: false,
    };
    expect(() => OrchestratorReply.parse(reply)).not.toThrow();
  });

  it("rejects an unknown stage", () => {
    const bad = { stage: "nope", openTool: "chat", widthTier: "chat", reference: [], updatedAtTurn: 0, started: true };
    expect(() => StudioState.parse(bad)).toThrow();
  });

  it("accepts a note-proposal reply with null directive fields defaulted", () => {
    const reply = {
      narrate: "要不要把这条记进「目标」？",
      directive: {
        stage: "proposal_forming",
        openTool: "plan",
        widthTier: "half",
        reference: [],
        updatedAtTurn: 1,
        started: true,
      },
      note: { section: "objective", value: "探究中国可持续发展对全球的净影响" },
      card: null,
      question: null,
      reviewRequested: false,
      planGenerated: false,
      compacted: false,
    };
    expect(() => OrchestratorReply.parse(reply)).not.toThrow();
  });

  it("openTool includes forming", () => {
    expect(OpenTool.safeParse("forming").success).toBe(true);
  });

  it("parses generate_plan and propose_question tools", () => {
    expect(OrchestratorTool.safeParse({ name: "generate_plan", args: {} }).success).toBe(true);
    expect(OrchestratorTool.safeParse({ name: "propose_question", args: { text: "中国的人均碳排放算高吗？" } }).success).toBe(true);
  });

  it("reply carries a nullable question proposal", () => {
    const r = OrchestratorReply.safeParse({
      narrate: "x",
      directive: { stage: "topic_discussion", openTool: "forming", widthTier: "half", reference: [], updatedAtTurn: 0, started: true },
      note: null,
      card: null,
      question: { text: "q" },
      reviewRequested: false,
      planGenerated: false,
      compacted: false,
    });
    expect(r.success).toBe(true);
  });

  it("reply carries planGenerated and compacted flags", () => {
    const r = OrchestratorReply.safeParse({
      narrate: "计划生成好了。",
      directive: { stage: "plan_generation", openTool: "plan", widthTier: "half", reference: [], updatedAtTurn: 2, started: true },
      note: null,
      card: null,
      question: null,
      reviewRequested: false,
      planGenerated: true,
      compacted: true,
    });
    expect(r.success).toBe(true);
    if (r.success) {
      expect(r.data.planGenerated).toBe(true);
      expect(r.data.compacted).toBe(true);
    }
  });

  it("StudioState with started parses", () => {
    const state = {
      stage: "topic_discussion",
      openTool: "chat",
      widthTier: "chat",
      reference: [],
      updatedAtTurn: 0,
      started: true,
    };
    expect(() => StudioState.parse(state)).not.toThrow();
  });

  it("StudioState missing started throws on parse", () => {
    const state = {
      stage: "topic_discussion",
      openTool: "chat",
      widthTier: "chat",
      reference: [],
      updatedAtTurn: 0,
      // started omitted on purpose — the server always emits it, so a legacy
      // payload without it must fail loudly rather than silently default.
    };
    expect(() => StudioState.parse(state)).toThrow();
  });

  it("parses a CoachHistoryPage with a card-carrying message", () => {
    const page = {
      messages: [
        { role: "student", text: "帮我看看这段论证", card: null },
        {
          role: "ai",
          text: "已经帮你标出反例了。",
          card: { cardId: "craap", fieldValues: { currency: "2024 年发布" } },
        },
      ],
      hasMore: true,
      recap: "上一轮聚焦在来源溯源。",
      nextCursor: "cursor-abc",
    };
    expect(() => CoachHistoryPage.parse(page)).not.toThrow();
  });

  it("CoachHistoryPage accepts null recap and nextCursor", () => {
    const page = { messages: [], hasMore: false, recap: null, nextCursor: null };
    expect(() => CoachHistoryPage.parse(page)).not.toThrow();
  });

  it("CoachHistoryMsg accepts a message with an absent card", () => {
    expect(() => CoachHistoryMsg.parse({ role: "student", text: "hi" })).not.toThrow();
  });
});
