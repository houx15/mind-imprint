import { describe, it, expect, vi, afterEach } from "vitest";
import { getReflection, putReflection, getMirror, postMirror } from "@/workspace/api/workspace";

afterEach(() => { vi.restoreAllMocks(); });

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const REFLECTION = { answers: ["当初想弄清尺度之争", "读了 NASA / Chen (2019)"], done: false };
const MIRROR = {
  sections: [
    { title: "你的论点是怎么长出来的", body: "你从通俗判断出发，一路把它复杂化成「取决于尺度」。" },
    { title: "阅读怎样喂养了写作", body: "溯源 NASA 数据这条判断后来进了你的正方段。" },
  ],
  carryForwards: ["下次在提纲阶段就先把反方埋进去。", "遇到通俗结论先追问「用什么尺度」。"],
};

describe("getReflection", () => {
  it("GETs /reflection-doc and Zod-parses the doc", async () => {
    const spy = vi.fn(async () => json(REFLECTION));
    vi.stubGlobal("fetch", spy);

    const doc = await getReflection("p1");
    expect(doc).toEqual(REFLECTION);

    const [url] = spy.mock.calls[0] as unknown as [string];
    expect(url).toContain("/api/v1/projects/p1/reflection-doc");
  });

  it("throws on schema drift", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json({ answers: "not-an-array", done: false })));
    await expect(getReflection("p1")).rejects.toThrow();
  });
});

describe("putReflection", () => {
  it("PUTs {answers} and returns the stored doc", async () => {
    const spy = vi.fn(async () => json({ answers: ["a", "b"], done: false }));
    vi.stubGlobal("fetch", spy);

    const doc = await putReflection("p1", { answers: ["a", "b"] });
    expect(doc).toEqual({ answers: ["a", "b"], done: false });

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/reflection-doc");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body)).toEqual({ answers: ["a", "b"] });
  });

  it("PUTs {answers, done:true} when finishing the reflection", async () => {
    const spy = vi.fn(async () => json({ answers: ["a"], done: true }));
    vi.stubGlobal("fetch", spy);

    const doc = await putReflection("p1", { answers: ["a"], done: true });
    expect(doc.done).toBe(true);

    const [, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(JSON.parse(init.body)).toEqual({ answers: ["a"], done: true });
  });
});

describe("getMirror", () => {
  it("GETs /mirror and Zod-parses the mirror", async () => {
    const spy = vi.fn(async () => json(MIRROR));
    vi.stubGlobal("fetch", spy);

    const m = await getMirror("p1");
    expect(m).toEqual(MIRROR);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit | undefined];
    expect(url).toContain("/api/v1/projects/p1/mirror");
    // GET never triggers a compose — no method means a GET.
    expect(init?.method).toBeUndefined();
  });

  it("returns null when the mirror hasn't been composed yet (JSON null)", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json(null)));
    await expect(getMirror("p1")).resolves.toBeNull();
  });

  it("throws on schema drift", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json({ sections: [{ title: "x" }], carryForwards: [] })));
    await expect(getMirror("p1")).rejects.toThrow();
  });
});

describe("postMirror", () => {
  it("POSTs /mirror and returns the composed mirror", async () => {
    const spy = vi.fn(async () => json(MIRROR));
    vi.stubGlobal("fetch", spy);

    const m = await postMirror("p1");
    expect(m).toEqual(MIRROR);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/projects/p1/mirror");
    expect(init.method).toBe("POST");
  });

  it("throws on schema drift", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json({ sections: [] })));
    await expect(postMirror("p1")).rejects.toThrow();
  });
});
