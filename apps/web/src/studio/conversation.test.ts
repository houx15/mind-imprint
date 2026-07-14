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

  it("dropFirst(n) removes only the first n messages without touching card/sending/error/disposableInterventionId", async () => {
    const conv = createStudioConversation({ projectId: "p1", api: fakeApi });
    await conv.send("它想证明中国在认真转型");
    const before = conv.getSnapshot();
    expect(before.messages.length).toBeGreaterThan(0);
    expect(before.disposableInterventionId).toBe("iid");

    conv.dropFirst(before.messages.length);
    const after = conv.getSnapshot();
    expect(after.messages).toEqual([]);
    // Everything else this session already knows must survive — a refetch
    // landing the persisted history must not also wipe the disposable
    // intervention id or clobber an in-flight card/sending/error state.
    expect(after.disposableInterventionId).toBe("iid");
    expect(after.sending).toBe(false);
    expect(after.error).toBeNull();
  });

  it("dropFirst(n) never drops a message that arrived AFTER n was captured — the refetch race (bug A)", async () => {
    const conv = createStudioConversation({ projectId: "p1", api: fakeApi });
    await conv.send("它想证明中国在认真转型");
    const priorCount = conv.getSnapshot().messages.length;

    // Simulates a turn entering the buffer while a refetch's GET is still in
    // flight — this must survive a dropFirst keyed to the PRE-flight count.
    await conv.send("那反例呢？");
    expect(conv.getSnapshot().messages.length).toBeGreaterThan(priorCount);

    conv.dropFirst(priorCount);
    const after = conv.getSnapshot();
    expect(after.messages.some((m) => m.body === "那反例呢？")).toBe(true);
  });

  // Whole-branch review CRITICAL 1: FIX-D suppresses a competing surface_card
  // server-side while any card_instance is proposed/active, but the client
  // must not itself be the kind of thing that discards a student's open,
  // half-filled card just because a frame arrived — defense in depth. This
  // is a DIFFERENT scenario than the submit-refeed test above: here the
  // student is just chatting (send()) with an ACTIVE card already open, and
  // an (in production, now server-suppressed) frame for a totally
  // unrelated card arrives — the open card must survive untouched.
  it("refuses to let an ordinary send() turn clobber an ACTIVE card with a different one", async () => {
    // Models two separate POSTs through the same studioTurn function: the
    // first call proposes ci1 (which the student then opens); a SECOND,
    // later send() — not a submit/skip refeed — proposes a completely
    // different card (ci2) while ci1 is still open and active.
    let calls = 0;
    const api = {
      async *studioTurn() {
        calls += 1;
        if (calls === 1) {
          yield { type: "card", cardInstanceId: "ci1", cardId: "craap", nudgeText: "n", anchors: [] };
        } else {
          yield { type: "card", cardInstanceId: "ci2", cardId: "sift", nudgeText: "n2", anchors: [] };
        }
        yield { type: "done" };
      },
      activateProjectCard: vi.fn(async () => {}),
      submitProjectCard: vi.fn(),
      skipProjectCard: vi.fn(async () => {}),
      postDisposition: vi.fn(async () => {}),
    } as any;
    const conv = createStudioConversation({ projectId: "p1", api });
    await conv.send("hi");
    await conv.openCard();
    expect(conv.getSnapshot().card).toMatchObject({ cardInstanceId: "ci1", status: "active" });

    await conv.send("再来点什么");

    const s = conv.getSnapshot();
    expect(s.card).toMatchObject({ cardInstanceId: "ci1", cardId: "craap", status: "active" });
  });

  it("still applies a send()-surfaced card when nothing is currently active", async () => {
    const conv = createStudioConversation({
      projectId: "p1",
      api: {
        async *studioTurn() { yield { type: "card", cardInstanceId: "ci1", cardId: "craap", nudgeText: "n", anchors: [] }; yield { type: "done" }; },
        postDisposition: vi.fn(async () => {}),
      } as any,
    });
    await conv.send("hi");
    expect(conv.getSnapshot().card).toMatchObject({ cardInstanceId: "ci1", cardId: "craap", status: "proposed" });
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
