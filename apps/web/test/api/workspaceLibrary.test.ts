import { describe, it, expect, vi, afterEach } from "vitest";
import {
  getLibrary,
  createCollection,
  patchCollection,
  deleteCollection,
  createReference,
  patchReference,
  deleteReference,
  enterReading,
  pasteContent,
  NoReadableContentError,
} from "@/workspace/api/workspace";

afterEach(() => { vi.restoreAllMocks(); });

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const COLLECTION = {
  id: "c1",
  name: "正方证据",
  parentId: null,
  position: 0,
};

const REFERENCE = {
  id: "r1",
  title: "NASA 卫星植被覆盖数据",
  classification: "数据集",
  author: "NASA Earth Observatory",
  credentials: "官方地球观测机构",
  year: "2023",
  url: "https://earthobservatory.nasa.gov/",
  tags: ["绿化"],
  collectionId: "c1",
  credibility: "strong" as const,
  evaluation: "能回答植被覆盖趋势。",
  decision: "use" as const,
  pending: false,
  searchHints: [] as string[],
  materialId: "m1",
  notes: [{ quote: "植被覆盖上升", finding: "支持正方" }],
};

const MATERIAL_SOURCE = {
  id: "m1",
  title: "NASA 卫星植被覆盖数据",
  sourceUrl: "https://earthobservatory.nasa.gov/",
  kind: "网页",
  origin: "earthobservatory.nasa.gov",
  blocks: [{ id: "b1", text: "植被覆盖在过去二十年间上升。" }],
  locked: false,
  role: "",
  tier: "",
  takeaway: "",
  anchors: [] as unknown[],
  timeSpentS: 0,
  lateralRead: false,
  isLateralInstrument: false,
  siftSkipped: false,
  lateralRelation: "",
  lateralJudgment: "",
};

describe("getLibrary", () => {
  it("GETs /library and Zod-parses collections + references", async () => {
    const spy = vi.fn(async () => json({ collections: [COLLECTION], references: [REFERENCE] }));
    vi.stubGlobal("fetch", spy);

    const lib = await getLibrary("p1");
    expect(lib.collections).toEqual([COLLECTION]);
    expect(lib.references).toEqual([REFERENCE]);

    const [url] = spy.mock.calls[0] as unknown as [string];
    expect(url).toContain("/api/v1/projects/p1/library");
  });

  it("throws on schema drift", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json({ collections: [], references: [{ id: "x" }] })));
    await expect(getLibrary("p1")).rejects.toThrow();
  });
});

describe("createCollection", () => {
  it("POSTs {name,parentId} and returns the created collection", async () => {
    const spy = vi.fn(async () => json({ collection: COLLECTION }));
    vi.stubGlobal("fetch", spy);

    const result = await createCollection("p1", { name: "正方证据" });
    expect(result).toEqual(COLLECTION);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/collections");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({ name: "正方证据" });
  });
});

describe("patchCollection", () => {
  it("PATCHes a partial edit to /collections/{cid}", async () => {
    const renamed = { ...COLLECTION, name: "改名" };
    const spy = vi.fn(async () => json({ collection: renamed }));
    vi.stubGlobal("fetch", spy);

    const result = await patchCollection("p1", "c1", { name: "改名" });
    expect(result).toEqual(renamed);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/collections/c1");
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(init.body)).toEqual({ name: "改名" });
  });
});

describe("deleteCollection", () => {
  it("DELETEs and resolves on 204", async () => {
    const spy = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", spy);

    await expect(deleteCollection("p1", "c1")).resolves.toBeUndefined();

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/projects/p1/collections/c1");
    expect(init.method).toBe("DELETE");
  });
});

describe("createReference", () => {
  it("POSTs the new-reference body and returns the created row", async () => {
    const spy = vi.fn(async () => json({ reference: REFERENCE }));
    vi.stubGlobal("fetch", spy);

    const body = { url: "https://earthobservatory.nasa.gov/", classification: "网页", collectionId: "c1" };
    const result = await createReference("p1", body);
    expect(result).toEqual(REFERENCE);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/references");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual(body);
  });
});

describe("patchReference", () => {
  it("PATCHes a partial metadata edit to /references/{rid}", async () => {
    const moved = { ...REFERENCE, collectionId: "c2" };
    const spy = vi.fn(async () => json({ reference: moved }));
    vi.stubGlobal("fetch", spy);

    const result = await patchReference("p1", "r1", { collectionId: "c2" });
    expect(result).toEqual(moved);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/references/r1");
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(init.body)).toEqual({ collectionId: "c2" });
  });
});

describe("deleteReference", () => {
  it("DELETEs and resolves on 204", async () => {
    const spy = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", spy);

    await expect(deleteReference("p1", "r1")).resolves.toBeUndefined();

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/projects/p1/references/r1");
    expect(init.method).toBe("DELETE");
  });
});

describe("enterReading", () => {
  it("POSTs enter-reading and Zod-parses the MaterialSource", async () => {
    const spy = vi.fn(async () => json(MATERIAL_SOURCE));
    vi.stubGlobal("fetch", spy);

    const source = await enterReading("p1", "r1");
    expect(source).toEqual(MATERIAL_SOURCE);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/projects/p1/references/r1/enter-reading");
    expect(init.method).toBe("POST");
  });

  it("throws NoReadableContentError on 422", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json({ error: "先补一个链接或粘贴正文" }, 422)));
    await expect(enterReading("p1", "r1")).rejects.toBeInstanceOf(NoReadableContentError);
  });

  it("rethrows other errors as-is", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json({ error: { message: "boom" } }, 500)));
    await expect(enterReading("p1", "r1")).rejects.not.toBeInstanceOf(NoReadableContentError);
  });
});

describe("pasteContent", () => {
  it("POSTs {text,title} and Zod-parses the MaterialSource", async () => {
    const spy = vi.fn(async () => json(MATERIAL_SOURCE));
    vi.stubGlobal("fetch", spy);

    const source = await pasteContent("p1", "r1", "植被覆盖上升。", "NASA 卫星植被覆盖数据");
    expect(source).toEqual(MATERIAL_SOURCE);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/references/r1/paste-content");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({ text: "植被覆盖上升。", title: "NASA 卫星植被覆盖数据" });
  });
});
