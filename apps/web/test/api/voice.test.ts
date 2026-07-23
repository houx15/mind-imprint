import { describe, it, expect, vi, beforeEach } from "vitest";
import { synthesize, AsrStream } from "@/api/voice";

describe("synthesize", () => {
  it("POSTs text and returns an object URL", async () => {
    const blob = new Blob([new Uint8Array([1, 2, 3])], { type: "audio/mpeg" });
    vi.stubGlobal("fetch", vi.fn(async () => new Response(blob, { status: 200 })));
    vi.stubGlobal("URL", { createObjectURL: vi.fn(() => "blob:xyz") } as never);
    const url = await synthesize("你好", { speed: 1 });
    expect(url).toBe("blob:xyz");
    expect(fetch).toHaveBeenCalledWith(expect.stringContaining("/voice/tts"), expect.objectContaining({ method: "POST" }));
  });
});

class FakeWebSocket {
  static OPEN = 1;
  static CONNECTING = 0;
  static instances: FakeWebSocket[] = [];
  readyState = FakeWebSocket.OPEN;
  url: string;
  sent: unknown[] = [];
  closed = false;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: (() => void) | null = null;

  constructor(url: string) {
    this.url = url;
    FakeWebSocket.instances.push(this);
  }

  send(data: unknown) {
    this.sent.push(data);
  }

  close() {
    this.closed = true;
  }

  emitMessage(data: unknown) {
    this.onmessage?.({ data: JSON.stringify(data) });
  }
}

describe("AsrStream", () => {
  beforeEach(() => {
    FakeWebSocket.instances = [];
    vi.stubGlobal("WebSocket", FakeWebSocket as unknown as typeof WebSocket);
  });

  it("dispatches partial transcripts", () => {
    const stream = new AsrStream();
    const onPartial = vi.fn();
    stream.onPartial(onPartial);
    const ws = FakeWebSocket.instances.at(0)!;
    ws.emitMessage({ type: "partial", text: "你" });
    expect(onPartial).toHaveBeenCalledWith("你");
  });

  it("dispatches final transcripts", () => {
    const stream = new AsrStream();
    const onFinal = vi.fn();
    stream.onFinal(onFinal);
    const ws = FakeWebSocket.instances.at(0)!;
    ws.emitMessage({ type: "final", text: "你好" });
    expect(onFinal).toHaveBeenCalledWith("你好");
  });

  it("sends PCM as binary frames", () => {
    const stream = new AsrStream();
    const ws = FakeWebSocket.instances.at(0)!;
    const pcm = new Int16Array([1, 2, 3]);
    stream.sendPCM(pcm);
    expect(ws.sent).toHaveLength(1);
    expect(ws.sent[0]).toBeInstanceOf(ArrayBuffer);
  });

  it("stops by sending a stop frame then closing", () => {
    const stream = new AsrStream();
    const ws = FakeWebSocket.instances.at(0)!;
    stream.stop();
    expect(ws.sent.some((m) => typeof m === "string" && m.includes("stop"))).toBe(true);
    expect(ws.closed).toBe(true);
  });

  it("invokes the error callback on a socket error", () => {
    const stream = new AsrStream();
    const onError = vi.fn();
    stream.onError(onError);
    const ws = FakeWebSocket.instances.at(0)!;
    ws.onerror?.();
    expect(onError).toHaveBeenCalledWith(expect.any(String));
  });

  it("invokes the error callback on an unexpected close", () => {
    const stream = new AsrStream();
    const onError = vi.fn();
    stream.onError(onError);
    const ws = FakeWebSocket.instances.at(0)!;
    ws.onclose?.();
    expect(onError).toHaveBeenCalledWith(expect.any(String));
  });

  it("does not invoke the error callback when stop() closes intentionally", () => {
    const stream = new AsrStream();
    const onError = vi.fn();
    stream.onError(onError);
    const ws = FakeWebSocket.instances.at(0)!;
    stream.stop();
    ws.onclose?.();
    expect(onError).not.toHaveBeenCalled();
  });
});
