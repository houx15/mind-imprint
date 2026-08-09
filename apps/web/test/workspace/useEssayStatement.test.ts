import { describe, it, expect, vi } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import { useEssayStatement, type EssayStatementDeps } from "@/workspace/blocks/useEssayStatement";
import type { ProposalGuideStep } from "@mind-imprint/contracts";

function stepFixture(over: Partial<ProposalGuideStep> = {}): ProposalGuideStep {
  return {
    key: "outline", title: "搭大纲", kind: "fixed", index: 0, total: 7,
    mode: "guided", started: false, subQuestions: [], card: null, ...over,
  };
}

function makeDeps(): EssayStatementDeps {
  return {
    getStep: vi.fn(async () => stepFixture()),
    start: vi.fn(async () => stepFixture({ started: true, card: { prompt: "根据证据调整大纲", example: "" } })),
    advance: vi.fn(async (_id, dir) => stepFixture({ started: true, index: dir === "next" ? 1 : 0, key: dir === "next" ? "claim:a" : "outline", kind: dir === "next" ? "subq" : "fixed" })),
  };
}

describe("useEssayStatement", () => {
  it("loads, starts (ready gate), and advances to a claim", async () => {
    const deps = makeDeps();
    const { result } = renderHook(() => useEssayStatement("p1", deps));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.step?.key).toBe("outline");

    await act(async () => { await result.current.start(); });
    expect(result.current.step?.started).toBe(true);
    expect(result.current.step?.card?.prompt).toContain("大纲");

    await act(async () => { await result.current.next(); });
    expect(result.current.step?.kind).toBe("subq");
    expect(deps.advance).toHaveBeenCalledWith("p1", "next");
  });
});
