import { describe, it, expect, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useReadingLoop } from "@/studio/reading/readingLoop";

const EXAMPLE_ANCHOR = {
  id: "ex0",
  material_id: "m1",
  block_id: "b1",
  start: 0,
  end: 10,
  quote: "因此这项政策必然失败",
  dimension: "argument-map",
  author: "ai" as const,
  question: "先看这处示范",
  answer: "",
};

const EVAL_RESULT = {
  verdict: "rethink" as const,
  verdictLabel: "暂不匹配",
  verdictReason: "",
  checks: [],
  finding: "f",
  judgment: "",
  support: "",
  caveat: "",
  nextStep: "n",
  spanIds: ["sel0"],
};

function makeFakeApi() {
  return {
    readTurn: vi.fn(async function* () {
      yield {
        type: "card" as const,
        cardInstanceId: "ci1",
        cardId: "argument-map",
        nudgeText: "这句像结论没给证据",
        anchors: [EXAMPLE_ANCHOR],
        materialId: "m1",
      };
      yield { type: "done" as const };
    }),
    activateProjectCard: vi.fn(async () => {}),
    evaluateCardSelection: vi.fn(async () => EVAL_RESULT),
    submitProjectCard: vi.fn(async function* () {
      yield { type: "done" as const, cardStatus: "completed" };
    }),
    skipProjectCard: vi.fn(async () => {}),
  };
}

const SOURCE = { id: "m1", blocks: [{ id: "b1", text: "因此这项政策必然失败" }] } as any;

describe("useReadingLoop", () => {
  it("runs idle→proposed→active→evaluating→feedback→idle", async () => {
    const fakeApi = makeFakeApi();
    const { result } = renderHook(() => useReadingLoop("p1", SOURCE, fakeApi as any));

    expect(result.current.status).toBe("idle");

    await act(async () => {
      await result.current.sendTurn("这段怪怪的");
    });
    expect(result.current.status).toBe("proposed");
    expect(result.current.cardName).toBeTruthy();
    expect(result.current.exampleBlockId).toBe("b1");
    expect(result.current.exampleWhy).toBe("先看这处示范");
    // The dialogue log grew: greeting + the student's turn + the lens notice.
    expect(result.current.messages.some((m) => m.role === "student" && m.body === "这段怪怪的")).toBe(true);
    expect(result.current.messages.some((m) => m.role === "assistant" && m.kind === "lens")).toBe(true);

    await act(async () => {
      await result.current.startPick();
    });
    expect(result.current.status).toBe("active");
    expect(fakeApi.activateProjectCard).toHaveBeenCalledWith("p1", "ci1");

    // Picking a DIFFERENT sentence than the example (different range).
    await act(async () => {
      await result.current.pickSentence({ blockId: "b1", start: 0, end: 5, text: "因此这项" });
    });
    expect(result.current.status).toBe("feedback");
    expect(result.current.eval?.verdict).toBe("rethink");
    expect(fakeApi.evaluateCardSelection).toHaveBeenCalledWith("p1", "ci1", {
      block_id: "b1", start: 0, end: 5, quote: "因此这项", dimension: "argument-map",
    });

    await act(async () => {
      await result.current.confirm();
    });
    expect(result.current.status).toBe("idle");
    expect(fakeApi.submitProjectCard).toHaveBeenCalledWith("p1", "ci1", {
      field_values: {},
      event_trace: [],
      anchors: [
        {
          id: "sel0", material_id: "m1", block_id: "b1", start: 0, end: 5,
          quote: "因此这项", dimension: "argument-map", author: "student", question: "", answer: "",
        },
      ],
    });
    expect(result.current.eval).toBeNull();
    expect(result.current.exampleAnchor).toBeNull();
    // The confirmed finding is SAVED as an outcome, not discarded.
    expect(result.current.outcomes).toHaveLength(1);
    const outcome = result.current.outcomes[0]!;
    expect(outcome.finding).toBe("f");
    expect(outcome.quote).toBe("因此这项");
    expect(outcome.blockId).toBe("b1");
  });

  it("rejects a pick that exactly matches the example — no evaluate call, status stays active", async () => {
    const fakeApi = makeFakeApi();
    const { result } = renderHook(() => useReadingLoop("p1", SOURCE, fakeApi as any));

    await act(async () => {
      await result.current.sendTurn("这段怪怪的");
    });
    await act(async () => {
      await result.current.startPick();
    });

    // Exact same block+range as the example anchor.
    await act(async () => {
      await result.current.pickSentence({ blockId: "b1", start: 0, end: 10, text: "因此这项政策必然失败" });
    });

    expect(result.current.status).toBe("active");
    expect(fakeApi.evaluateCardSelection).not.toHaveBeenCalled();
  });

  it("repick() clears the student pick and eval, returning to active", async () => {
    const fakeApi = makeFakeApi();
    const { result } = renderHook(() => useReadingLoop("p1", SOURCE, fakeApi as any));

    await act(async () => {
      await result.current.sendTurn("这段怪怪的");
    });
    await act(async () => {
      await result.current.startPick();
    });
    await act(async () => {
      await result.current.pickSentence({ blockId: "b1", start: 0, end: 5, text: "因此这项" });
    });
    expect(result.current.status).toBe("feedback");

    act(() => {
      result.current.repick();
    });
    expect(result.current.status).toBe("active");
    expect(result.current.studentSpan).toBeNull();
    expect(result.current.eval).toBeNull();
  });

  it("skip() calls skipProjectCard and returns to idle", async () => {
    const fakeApi = makeFakeApi();
    const { result } = renderHook(() => useReadingLoop("p1", SOURCE, fakeApi as any));

    await act(async () => {
      await result.current.sendTurn("这段怪怪的");
    });
    await act(async () => {
      await result.current.skip();
    });

    expect(fakeApi.skipProjectCard).toHaveBeenCalledWith("p1", "ci1", { event_trace: [] });
    expect(result.current.status).toBe("idle");
  });

  it("an intervention event appends a coach line and stays idle", async () => {
    const fakeApi = {
      readTurn: vi.fn(async function* () {
        yield { type: "intervention" as const, interventionId: "i1", body: "再想想这句的证据在哪", anchor: "", criterion: "", level: "" };
        yield { type: "done" as const };
      }),
      activateProjectCard: vi.fn(async () => {}),
      evaluateCardSelection: vi.fn(async () => EVAL_RESULT),
      submitProjectCard: vi.fn(async function* () {}),
      skipProjectCard: vi.fn(async () => {}),
    };
    const { result } = renderHook(() => useReadingLoop("p1", SOURCE, fakeApi as any));

    await act(async () => {
      await result.current.sendTurn("这段怪怪的");
    });

    expect(result.current.status).toBe("idle");
    expect(result.current.messages.some((m) => m.role === "assistant" && m.kind === "text" && m.body === "再想想这句的证据在哪")).toBe(true);
  });
});
