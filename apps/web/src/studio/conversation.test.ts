import { describe, it, expect } from "vitest";
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
});
