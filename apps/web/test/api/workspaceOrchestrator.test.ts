import { describe, it, expect, vi, afterEach } from "vitest";
import { coach, getStudioState } from "@/workspace/api/workspace";
import type { OrchestratorReply, StudioState } from "@mind-imprint/contracts";

afterEach(() => { vi.restoreAllMocks(); });

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const DIRECTIVE: StudioState = {
  stage: "topic_discussion",
  openTool: "chat",
  widthTier: "chat",
  reference: [],
  updatedAtTurn: 1,
};

const REPLY: OrchestratorReply = {
  narrate: "先说说你想研究什么？",
  directive: DIRECTIVE,
  note: null,
  card: null,
  reviewRequested: false,
};

describe("coach (orchestrator turn)", () => {
  it("POSTs { user_input } only (no scope) and returns the parsed OrchestratorReply", async () => {
    const spy = vi.fn(async () => json(REPLY));
    vi.stubGlobal("fetch", spy);

    const result = await coach("p1", "hi");
    expect(result).toEqual(REPLY);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/coach");
    expect(init.method).toBe("POST");
    const body = JSON.parse(init.body);
    expect(body).toEqual({ user_input: "hi" });
    expect(body.scope).toBeUndefined();
  });

  it("throws on schema drift (e.g. missing directive)", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json({ narrate: "x" })));
    await expect(coach("p1", "hi")).rejects.toThrow();
  });
});

describe("getStudioState", () => {
  it("GETs /studio-state and returns the parsed StudioState", async () => {
    const spy = vi.fn(async () => json(DIRECTIVE));
    vi.stubGlobal("fetch", spy);

    const result = await getStudioState("p1");
    expect(result).toEqual(DIRECTIVE);

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit | undefined];
    expect(url).toContain("/api/v1/projects/p1/studio-state");
    expect(init?.method ?? undefined).toBeUndefined();
  });

  it("throws on schema drift", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json({ stage: "topic_discussion" })));
    await expect(getStudioState("p1")).rejects.toThrow();
  });
});
