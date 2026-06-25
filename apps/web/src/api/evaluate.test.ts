import { describe, it, expect, vi } from "vitest";
import { runEvaluation, getEvaluation } from "./evaluate";

describe("evaluate api", () => {
  it("runEvaluation POSTs and unwraps {evaluation}", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ evaluation: { task_id: "t1", scores: [], narrative: "n", created_at: "z" } }), { status: 200 })));
    expect((await runEvaluation("t1")).narrative).toBe("n");
  });
  it("getEvaluation returns null on 404", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ error: { code: "not_found", message: "无" } }), { status: 404 })));
    expect(await getEvaluation("t1")).toBeNull();
  });
});
