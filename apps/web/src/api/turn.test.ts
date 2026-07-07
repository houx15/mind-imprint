import { describe, it, expect, vi } from "vitest";
import { runTurn } from "./turn";

function sseResponse(body: string): Response {
  const enc = new TextEncoder();
  const stream = new ReadableStream({ start(c) { c.enqueue(enc.encode(body)); c.close(); } });
  return new Response(stream, { status: 200, headers: { "Content-Type": "text/event-stream" } });
}

describe("runTurn", () => {
  it("maps SSE frames to TurnEvents", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => sseResponse(
      'event: text\ndata: {"delta":"hi"}\n\nevent: card\ndata: {"card_instance_id":"c1","card_id":"sift_craap","nudge_text":"溯源?","anchors":[{"id":"a0","question":"可信吗？"}]}\n\nevent: done\ndata: {"message_id":"m1"}\n\n',
    )));
    const events = [];
    for await (const e of runTurn("t1", "hello")) events.push(e);
    expect(events).toEqual([
      { type: "text", delta: "hi" },
      { type: "card", cardInstanceId: "c1", cardId: "sift_craap", nudgeText: "溯源?", anchors: [{ id: "a0", question: "可信吗？" }] },
      { type: "done", messageId: "m1" },
    ]);
  });
  it("yields a single error event when the gate returns a JSON error (non-SSE)", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ error: { code: "not_entitled", message: "无额度" } }), { status: 403 })));
    const events = [];
    for await (const e of runTurn("t1", "hello")) events.push(e);
    expect(events).toEqual([{ type: "error", code: "not_entitled", message: "无额度" }]);
  });
});
