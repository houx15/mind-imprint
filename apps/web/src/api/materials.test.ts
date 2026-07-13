import { describe, it, expect, vi, afterEach } from "vitest";
import { addMaterial, logSourceOpen } from "./materials";

afterEach(() => { vi.restoreAllMocks(); });

const materialBody = {
  id: "m1",
  title: "《卫星图看中国变绿》",
  sourceUrl: "https://x.test/a",
  kind: "article",
  origin: "fetched",
  blocks: [{ id: "b1", text: "过去二十年……" }],
  locked: false,
  role: "",
  tier: "二手 · 需追源",
  takeaway: "结论被放大了。",
  anchors: [],
};

describe("addMaterial", () => {
  it("POSTs to .../materials with the body and parses the response through MaterialSource", async () => {
    const spy = vi.fn(async () => new Response(JSON.stringify(materialBody), {
      status: 201,
      headers: { "Content-Type": "application/json" },
    }));
    vi.stubGlobal("fetch", spy);

    const body = { url: "https://x.test/a", takeaway: "结论被放大了。", tier: "二手 · 需追源" };
    const result = await addMaterial("p1", body);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/materials");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual(body);
    expect(result).toEqual(materialBody);
  });

  it("accepts the pasted-text body shape", async () => {
    const spy = vi.fn(async () => new Response(JSON.stringify(materialBody), {
      status: 201,
      headers: { "Content-Type": "application/json" },
    }));
    vi.stubGlobal("fetch", spy);

    const body = { title: "标题", text: "正文……", takeaway: "结论被放大了。", tier: "二手 · 需追源" };
    await addMaterial("p1", body);

    const [, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(JSON.parse(init.body)).toEqual(body);
  });

  it("surfaces the server's Chinese error message on failure, verbatim", async () => {
    const spy = vi.fn(async () => new Response(
      JSON.stringify({ error: { code: "fetch_failed", message: "取不到这个链接的正文，可以直接把正文粘进来。" } }),
      { status: 400, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    await expect(addMaterial("p1", { url: "https://x.test/dead", takeaway: "t", tier: "t" }))
      .rejects.toThrow("取不到这个链接的正文，可以直接把正文粘进来。");
  });

  it("throws on a malformed MaterialSource (schema drift)", async () => {
    const spy = vi.fn(async () => new Response(JSON.stringify({ ...materialBody, blocks: "not-an-array" }), {
      status: 201,
      headers: { "Content-Type": "application/json" },
    }));
    vi.stubGlobal("fetch", spy);

    await expect(addMaterial("p1", { url: "https://x.test/a", takeaway: "t", tier: "t" })).rejects.toThrow();
  });
});

describe("logSourceOpen", () => {
  it("POSTs to .../materials/{mid}/open with time_spent_s and expects no body", async () => {
    const spy = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", spy);

    await logSourceOpen("p1", "m1", 42);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/materials/m1/open");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({ time_spent_s: 42 });
  });
});
