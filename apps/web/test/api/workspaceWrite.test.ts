import { describe, it, expect, vi, afterEach } from "vitest";
import { getOutline, putOutline, getSnippets, putSnippets, getDraft } from "@/workspace/api/workspace";

afterEach(() => { vi.restoreAllMocks(); });

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const NODES = [
  { id: "o1", text: "引言：为什么这个题目值得问", depth: 0, position: 0 },
  { id: "o2", text: "正方：中国在变绿", depth: 0, position: 1 },
  { id: "o3", text: "NASA 卫星植被覆盖数据", depth: 1, position: 2 },
];

describe("getOutline", () => {
  it("GETs /outline and Zod-parses the nodes array", async () => {
    const spy = vi.fn(async () => json({ nodes: NODES }));
    vi.stubGlobal("fetch", spy);

    const nodes = await getOutline("p1");
    expect(nodes).toEqual(NODES);

    const [url] = spy.mock.calls[0] as unknown as [string];
    expect(url).toContain("/api/v1/projects/p1/outline");
  });

  it("throws on schema drift", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json({ nodes: [{ id: "x" }] })));
    await expect(getOutline("p1")).rejects.toThrow();
  });
});

const SNIPPETS = [
  { id: "s1", text: "碳排放全球第一（反例）", position: 0, section: null },
  { id: "s2", text: "NASA 绿化数据", position: 1, section: null },
];

describe("getSnippets", () => {
  it("GETs /snippets and Zod-parses the snippets array", async () => {
    const spy = vi.fn(async () => json({ snippets: SNIPPETS }));
    vi.stubGlobal("fetch", spy);
    const got = await getSnippets("p1");
    expect(got).toEqual(SNIPPETS);
    const [url] = spy.mock.calls[0] as unknown as [string];
    expect(url).toContain("/api/v1/projects/p1/snippets");
  });
});

describe("putSnippets", () => {
  it("PUTs the {text}[] array and returns the fresh snippets", async () => {
    const spy = vi.fn(async () => json({ snippets: SNIPPETS }));
    vi.stubGlobal("fetch", spy);
    const got = await putSnippets("p1", [{ text: "碳排放全球第一（反例）" }, { text: "NASA 绿化数据" }]);
    expect(got).toEqual(SNIPPETS);
    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/snippets");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body)).toEqual({ snippets: [{ text: "碳排放全球第一（反例）" }, { text: "NASA 绿化数据" }] });
  });
});

describe("putOutline", () => {
  it("PUTs the flat {id?,text,depth} array and returns the fresh nodes", async () => {
    const spy = vi.fn(async () => json({ nodes: NODES }));
    vi.stubGlobal("fetch", spy);

    const body = [
      { id: "o1", text: "引言：为什么这个题目值得问", depth: 0 },
      { text: "正方：中国在变绿", depth: 0 },
      { text: "NASA 卫星植被覆盖数据", depth: 1 },
    ];
    const result = await putOutline("p1", body);
    expect(result).toEqual(NODES);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/outline");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body)).toEqual({ nodes: body });
  });

  it("throws on schema drift in the response", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json({ nodes: [{ text: "no id" }] })));
    await expect(putOutline("p1", [{ text: "x", depth: 0 }])).rejects.toThrow();
  });
});

describe("getDraft", () => {
  it("GETs /draft and returns the content string", async () => {
    const spy = vi.fn(async () => json({ content: "## 引言\n中国的发展……" }));
    vi.stubGlobal("fetch", spy);

    const content = await getDraft("p1");
    expect(content).toBe("## 引言\n中国的发展……");

    const [url] = spy.mock.calls[0] as unknown as [string];
    expect(url).toContain("/api/v1/projects/p1/draft");
  });

  it("returns empty string when the buffer is empty", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json({ content: "" })));
    await expect(getDraft("p1")).resolves.toBe("");
  });

  it("throws on schema drift", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json({})));
    await expect(getDraft("p1")).rejects.toThrow();
  });
});
