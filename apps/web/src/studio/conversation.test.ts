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
});
