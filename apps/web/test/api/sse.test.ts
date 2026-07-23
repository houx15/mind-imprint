import { describe, it, expect } from "vitest";
import { parseSSE } from "@/api/sse";

function streamOf(...chunks: string[]): ReadableStream<Uint8Array> {
  const enc = new TextEncoder();
  return new ReadableStream({ start(c) { for (const ch of chunks) c.enqueue(enc.encode(ch)); c.close(); } });
}

describe("parseSSE", () => {
  it("parses event+data frames split across chunks, skipping heartbeats", async () => {
    const s = streamOf("event: text\ndata: {\"delta\":\"a\"}\n\n: ping\n\nevent: do", "ne\ndata: {\"message_id\":\"m1\"}\n\n");
    const got: { event: string; data: string }[] = [];
    for await (const f of parseSSE(s)) got.push(f);
    expect(got).toEqual([
      { event: "text", data: '{"delta":"a"}' },
      { event: "done", data: '{"message_id":"m1"}' },
    ]);
  });
});
