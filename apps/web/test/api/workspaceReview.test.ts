import { describe, it, expect, vi, afterEach } from "vitest";
import { getReflection, putReflection } from "@/workspace/api/workspace";

afterEach(() => { vi.restoreAllMocks(); });

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const REFLECTION = { answers: ["当初想弄清尺度之争", "读了 NASA / Chen (2019)"], done: false };

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

