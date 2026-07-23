import { describe, it, expect, vi } from "vitest";
import { apiFetch, ApiError } from "@/api/client";

describe("apiFetch", () => {
  it("returns parsed JSON on 200", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ ok: 1 }), { status: 200 })));
    expect(await apiFetch<{ ok: number }>("/x")).toEqual({ ok: 1 });
  });
  it("throws ApiError carrying the envelope code on non-2xx", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ error: { code: "not_entitled", message: "无额度" } }), { status: 403 })));
    await expect(apiFetch("/x")).rejects.toMatchObject({ code: "not_entitled", status: 403 });
  });
  it("attaches details from the error envelope to ApiError", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ error: { code: "validation_failed", message: "x", details: { row: 2 } } }), { status: 400 })));
    let caught: unknown;
    try { await apiFetch("/x"); } catch (e) { caught = e; }
    expect(caught).toBeInstanceOf(ApiError);
    const err = caught as ApiError;
    expect(err.code).toBe("validation_failed");
    expect(err.status).toBe(400);
    expect(err.details).toEqual({ row: 2 });
  });
  it("sends credentials and JSON content-type", async () => {
    const spy = vi.fn(async () => new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", spy);
    await apiFetch("/x", { method: "POST", body: "{}" });
    const callArgs = spy.mock.calls[0] as unknown as [string, RequestInit];
    const init = callArgs[1];
    expect(init?.credentials).toBe("include");
    expect((init?.headers as Record<string,string>)["Content-Type"]).toBe("application/json");
  });
});
