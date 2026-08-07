import { describe, it, expect, vi, afterEach } from "vitest";
import {
  putProposal,
  getPlan,
  createPlanItem,
  patchPlanItem,
  deletePlanItem,
  getLog,
  addLog,
} from "@/workspace/api/workspace";

afterEach(() => { vi.restoreAllMocks(); });

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const ITEM = {
  id: "i1",
  title: "读：NASA 卫星植被覆盖数据",
  tag: "read" as const,
  column: "todo" as const,
  stage: "阶段一 · 研究与写作",
  refMaterialId: null,
  start: 0,
  days: 2,
  position: 0,
};

describe("putProposal", () => {
  it("PUTs the four dims and returns the stored proposal", async () => {
    const proposal = { objective: "o", reason: "r", activities: "a", resources: "s", counterpoints: "c" };
    const spy = vi.fn(async () => json({ proposal }));
    vi.stubGlobal("fetch", spy);

    const result = await putProposal("p1", proposal);
    expect(result).toEqual(proposal);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/proposal");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body)).toEqual(proposal);
  });
});

describe("getPlan", () => {
  it("GETs /plan and Zod-parses the items array", async () => {
    const spy = vi.fn(async () => json({ items: [ITEM] }));
    vi.stubGlobal("fetch", spy);

    const items = await getPlan("p1");
    expect(items).toEqual([ITEM]);

    const [url] = spy.mock.calls[0] as unknown as [string];
    expect(url).toContain("/api/v1/projects/p1/plan");
  });

  it("throws on schema drift", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json({ items: [{ id: "x" }] })));
    await expect(getPlan("p1")).rejects.toThrow();
  });
});

describe("createPlanItem", () => {
  it("POSTs the new-item body and returns the created row", async () => {
    const spy = vi.fn(async () => json({ item: ITEM }));
    vi.stubGlobal("fetch", spy);

    const body = { title: "写：新任务", tag: "write" as const, column: "todo" as const, stage: "阶段一 · 研究与写作", start: 0, days: 2 };
    const result = await createPlanItem("p1", body);
    expect(result).toEqual(ITEM);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/plan/items");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual(body);
  });
});

describe("patchPlanItem", () => {
  it("PATCHes a partial edit to /plan/items/{iid}", async () => {
    const moved = { ...ITEM, column: "doing" as const };
    const spy = vi.fn(async () => json({ item: moved }));
    vi.stubGlobal("fetch", spy);

    const result = await patchPlanItem("p1", "i1", { column: "doing" });
    expect(result).toEqual(moved);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/plan/items/i1");
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(init.body)).toEqual({ column: "doing" });
  });
});

describe("deletePlanItem", () => {
  it("DELETEs and resolves on 204", async () => {
    const spy = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", spy);

    await expect(deletePlanItem("p1", "i1")).resolves.toBeUndefined();

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/projects/p1/plan/items/i1");
    expect(init.method).toBe("DELETE");
  });
});

describe("getLog / addLog", () => {
  it("GETs the entries array", async () => {
    const entry = { id: "l1", date: "07-02", text: "确定题目方向", source: "me" as const };
    vi.stubGlobal("fetch", vi.fn(async () => json({ entries: [entry] })));

    const entries = await getLog("p1");
    expect(entries).toEqual([entry]);
  });

  it("POSTs a note and returns the created entry", async () => {
    const entry = { id: "l9", date: "07-29", text: "补一笔", source: "me" as const };
    const spy = vi.fn(async () => json({ entry }));
    vi.stubGlobal("fetch", spy);

    const result = await addLog("p1", "补一笔");
    expect(result).toEqual(entry);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/log");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({ text: "补一笔" });
  });
});

// coach() moved to the orchestrator shape (2026-08-07 redesign) — its
// coverage now lives in test/api/workspaceOrchestrator.test.ts.
