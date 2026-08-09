import { describe, it, expect, vi } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import { useProposalTrack, type ProposalTrackDeps } from "@/workspace/blocks/useProposalTrack";
import type { ProposalGuideStep } from "@mind-imprint/contracts";

function stepFixture(over: Partial<ProposalGuideStep> = {}): ProposalGuideStep {
  return {
    key: "understanding",
    title: "对题目的理解",
    kind: "fixed",
    index: 0,
    total: 9,
    mode: "",
    started: false,
    subQuestions: [],
    card: null,
    steps: [],
    ...over,
  };
}

function makeDeps(): ProposalTrackDeps {
  return {
    getTrack: vi.fn(async () => stepFixture()),
    setMode: vi.fn(async (_id, m) => stepFixture({ mode: m })),
    start: vi.fn(async () => stepFixture({ mode: "guided", started: true, card: { prompt: "q", example: "e" } })),
    saveSubQuestions: vi.fn(async (_id, list) => stepFixture({ mode: "guided", started: true, subQuestions: list, total: 9 + list.length })),
    advance: vi.fn(async (_id, dir) => stepFixture({ mode: "guided", started: true, index: dir === "next" ? 1 : 0 })),
  };
}

describe("useProposalTrack", () => {
  it("loads the track on mount", async () => {
    const deps = makeDeps();
    const { result } = renderHook(() => useProposalTrack("p1", deps));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.step?.key).toBe("understanding");
    expect(deps.getTrack).toHaveBeenCalledWith("p1");
  });

  it("chooseMode then start transitions to a guided card", async () => {
    const deps = makeDeps();
    const { result } = renderHook(() => useProposalTrack("p1", deps));
    await waitFor(() => expect(result.current.loading).toBe(false));

    await act(async () => { await result.current.chooseMode("guided"); });
    expect(result.current.step?.mode).toBe("guided");

    await act(async () => { await result.current.start(); });
    expect(result.current.step?.started).toBe(true);
    expect(result.current.step?.card?.prompt).toBe("q");
  });

  it("saveSubQuestions expands the total", async () => {
    const deps = makeDeps();
    const { result } = renderHook(() => useProposalTrack("p1", deps));
    await waitFor(() => expect(result.current.loading).toBe(false));

    await act(async () => {
      await result.current.saveSubQuestions([{ id: "", text: "q1" }, { id: "", text: "q2" }]);
    });
    expect(result.current.step?.total).toBe(11);
    expect(result.current.step?.subQuestions).toHaveLength(2);
  });

  it("next advances the step index", async () => {
    const deps = makeDeps();
    const { result } = renderHook(() => useProposalTrack("p1", deps));
    await waitFor(() => expect(result.current.loading).toBe(false));
    await act(async () => { await result.current.next(); });
    expect(result.current.step?.index).toBe(1);
    expect(deps.advance).toHaveBeenCalledWith("p1", "next");
  });
});
