import { describe, it, expect, vi } from "vitest";
import { synthesize } from "./voice";

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
