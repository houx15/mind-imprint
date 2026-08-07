import { describe, it, expect } from "vitest";
import { StudioState, OrchestratorReply } from "../src/orchestrator";

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
      },
      note: null,
      card: null,
      reviewRequested: false,
    };
    expect(() => OrchestratorReply.parse(reply)).not.toThrow();
  });

  it("rejects an unknown stage", () => {
    const bad = { stage: "nope", openTool: "chat", widthTier: "chat", reference: [], updatedAtTurn: 0 };
    expect(() => StudioState.parse(bad)).toThrow();
  });

  it("accepts a note-proposal reply with null directive fields defaulted", () => {
    const reply = {
      narrate: "要不要把这条记进「目标」？",
      directive: { stage: "proposal_forming", openTool: "plan", widthTier: "half", reference: [], updatedAtTurn: 1 },
      note: { section: "objective", value: "探究中国可持续发展对全球的净影响" },
      card: null,
      reviewRequested: false,
    };
    expect(() => OrchestratorReply.parse(reply)).not.toThrow();
  });
});
