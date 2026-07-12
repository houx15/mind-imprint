import { describe, it, expect, vi, afterEach } from "vitest";
import { studioTurn } from "./studioTurn";

afterEach(() => { vi.restoreAllMocks(); });

function sseBody(frames: string): Response {
  const stream = new ReadableStream<Uint8Array>({
    start(c) { c.enqueue(new TextEncoder().encode(frames)); c.close(); },
  });
  return new Response(stream, { status: 200, headers: { "Content-Type": "text/event-stream" } });
}

describe("studioTurn", () => {
  it("yields intervention then done", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(sseBody(
      `event: intervention\ndata: {"intervention_id":"iid","body":"连到治理决心","anchor":"论证图 · 治理决心主张","criterion":"D5","level":"I2"}\n\n` +
      `event: done\ndata: {}\n\n`,
    ));
    const events = [];
    for await (const e of studioTurn("p1", "它想证明中国在认真转型")) events.push(e);
    expect(events[0]).toMatchObject({ type: "intervention", interventionId: "iid", criterion: "D5" });
    expect(events.at(-1)).toEqual({ type: "done" });
  });

  it("yields a card event", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(sseBody(
      `event: card\ndata: {"card_instance_id":"ci1","card_id":"craap","nudge_text":"CRAAP 五维核查","anchors":[]}\n\n` +
      `event: done\ndata: {}\n\n`));
    const events = [];
    for await (const e of studioTurn("p1", "hi")) events.push(e);
    expect(events[0]).toMatchObject({ type: "card", cardInstanceId: "ci1", cardId: "craap", nudgeText: "CRAAP 五维核查" });
  });
});
