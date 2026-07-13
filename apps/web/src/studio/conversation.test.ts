import { describe, it, expect, vi } from "vitest";
import { createStudioConversation } from "./conversation";

const fakeApi = {
  async *studioTurn() {
    yield { type: "intervention", interventionId: "iid", body: "连到治理决心", anchor: "论证图 · 治理决心主张", criterion: "D5", level: "I2" };
    yield { type: "done" };
  },
  postDisposition: async () => {},
} as any;

describe("createStudioConversation", () => {
  it("appends the student bubble then the coach reply, tracks the disposable id", async () => {
    const conv = createStudioConversation({ projectId: "p1", api: fakeApi });
    await conv.send("它想证明中国在认真转型");
    const s = conv.getSnapshot();
    expect(s.messages[0]).toMatchObject({ kind: "student", body: "它想证明中国在认真转型" });
    expect(s.messages[1]).toMatchObject({ kind: "ai", body: "连到治理决心", tag: "D5", anchor: "论证图 · 治理决心主张" });
    expect(s.sending).toBe(false);
    expect(s.disposableInterventionId).toBe("iid");
  });

  it("holds a proposed card, opens it, submits + refeeds", async () => {
    const api = {
      async *studioTurn() { yield { type: "card", cardInstanceId: "ci1", cardId: "craap", nudgeText: "n", anchors: [] }; yield { type: "done" }; },
      activateProjectCard: vi.fn(async () => {}),
      async *submitProjectCard() { yield { type: "intervention", interventionId: "i1", body: "补得不错", anchor: "", criterion: "D5", level: "I2" }; yield { type: "done" }; },
      skipProjectCard: vi.fn(async () => {}),
      postDisposition: vi.fn(async () => {}),
    } as any;
    const conv = createStudioConversation({ projectId: "p1", api });
    await conv.send("hi");
    expect(conv.getSnapshot().card).toMatchObject({ cardInstanceId: "ci1", cardId: "craap", status: "proposed" });
    await conv.openCard();
    expect(conv.getSnapshot().card?.status).toBe("active");
    await conv.submitCard({ field_values: {}, event_trace: [], anchors: [] } as any);
    const s = conv.getSnapshot();
    expect(s.card).toBeNull();
    expect(s.messages.at(-1)).toMatchObject({ kind: "ai", body: "补得不错" });
  });

  it("keeps a newly-surfaced card alive when submit refeed surfaces a NEW card before done", async () => {
    const api = {
      async *studioTurn() { yield { type: "card", cardInstanceId: "ci1", cardId: "craap", nudgeText: "n", anchors: [] }; yield { type: "done" }; },
      activateProjectCard: vi.fn(async () => {}),
      // Refeed after submitting ci1 surfaces a brand-new card ci2 (e.g. CRAAP on the second material),
      // with NO intervening intervention event before "done".
      async *submitProjectCard() { yield { type: "card", cardInstanceId: "ci2", cardId: "craap", nudgeText: "n2", anchors: [] }; yield { type: "done" }; },
      skipProjectCard: vi.fn(async () => {}),
      postDisposition: vi.fn(async () => {}),
    } as any;
    const conv = createStudioConversation({ projectId: "p1", api });
    await conv.send("hi");
    expect(conv.getSnapshot().card).toMatchObject({ cardInstanceId: "ci1", cardId: "craap", status: "proposed" });
    await conv.openCard();
    await conv.submitCard({ field_values: {}, event_trace: [], anchors: [] } as any);
    const s = conv.getSnapshot();
    // The NEW card (ci2) must survive the "done" clear — it must NOT be wiped
    // just because the submitted card (ci1) is done. Against the old unconditional
    // `set({ card: null })` on "done", this assertion would fail (card would be null).
    expect(s.card).toMatchObject({ cardInstanceId: "ci2", cardId: "craap", status: "proposed" });
  });

  it("clearMessages empties the local turn buffer without touching card/sending/error/disposableInterventionId", async () => {
    const conv = createStudioConversation({ projectId: "p1", api: fakeApi });
    await conv.send("它想证明中国在认真转型");
    const before = conv.getSnapshot();
    expect(before.messages.length).toBeGreaterThan(0);
    expect(before.disposableInterventionId).toBe("iid");

    conv.clearMessages();
    const after = conv.getSnapshot();
    expect(after.messages).toEqual([]);
    // Everything else this session already knows must survive — a refetch
    // landing the persisted history must not also wipe the disposable
    // intervention id or clobber an in-flight card/sending/error state.
    expect(after.disposableInterventionId).toBe("iid");
    expect(after.sending).toBe(false);
    expect(after.error).toBeNull();
  });

  it("carries anchors from the card event onto the card state", async () => {
    const anchors = [
      { id: "a1", material_id: "m1", block_id: "b1", start: 0, end: 10, quote: "q1", dimension: "source", author: "ai", question: "谁写的?", answer: "" },
      { id: "a2", material_id: "m1", block_id: "b2", start: 11, end: 20, quote: "q2", dimension: "claim", author: "ai", question: "证据在哪?", answer: "" },
    ];
    const api = {
      async *studioTurn() { yield { type: "card", cardInstanceId: "ci1", cardId: "craap", nudgeText: "n", anchors }; yield { type: "done" }; },
      activateProjectCard: vi.fn(async () => {}),
      submitProjectCard: vi.fn(),
      skipProjectCard: vi.fn(async () => {}),
      postDisposition: vi.fn(async () => {}),
    } as any;
    const conv = createStudioConversation({ projectId: "p1", api });
    await conv.send("hi");
    const cardAnchors = conv.getSnapshot().card?.anchors;
    expect(cardAnchors).toHaveLength(2);
    expect(cardAnchors?.map((a) => a.dimension)).toEqual(["source", "claim"]);
  });
});
