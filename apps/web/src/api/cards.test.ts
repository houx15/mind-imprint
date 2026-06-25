import { describe, it, expect, vi } from "vitest";
import { submitCard, skipCard, activateCard } from "./cards";

const ok = (body: unknown) => vi.fn(async () => new Response(JSON.stringify(body), { status: 200 }));
const env = { id: "c1", card_id: "x", task_id: "t1", parent_node_id: null, status: "completed", field_values: { a: 1 }, event_trace: [{ kind: "submit", at: "z" }], rubric_tags: [], created_at: "z", completed_at: "z" } as const;

describe("cards api", () => {
  it("submitCard PUTs status completed + field_values + event_trace", async () => {
    const spy = ok({ card: env });
    vi.stubGlobal("fetch", spy);
    await submitCard("t1", "c1", env as never);
    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/tasks/t1/cards/c1");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body)).toMatchObject({ status: "completed", field_values: { a: 1 } });
  });
  it("skipCard POSTs to /skip with event_trace", async () => {
    const spy = ok({ card: { ...env, status: "skipped" } });
    vi.stubGlobal("fetch", spy);
    await skipCard("t1", "c1", env.event_trace as never);
    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/cards/c1/skip");
    expect(init.method).toBe("POST");
  });
  it("activateCard PATCHes status active", async () => {
    const spy = ok({ card: { ...env, status: "active" } });
    vi.stubGlobal("fetch", spy);
    await activateCard("t1", "c1");
    expect((spy.mock.calls[0] as unknown as [string, RequestInit])[1].method).toBe("PATCH");
  });
});
