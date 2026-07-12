import { describe, it, expect, vi, afterEach } from "vitest";
import { getProject } from "./projects";

afterEach(() => { vi.restoreAllMocks(); });

const sample = {
  project: { title: "T", qualLabel: "0457 个人报告" },
  stations: [{ code: "S4", name: "论证构建", view: "结构", state: "current", gate: { total: 7, passed: 2 } }],
  activeStation: "S4",
  coach: { anchor: "论证图 · 治理决心主张", messages: [{ kind: "ai", body: "b", tag: "D5", anchor: "论证图 · 治理决心主张" }], equipment: [] },
  onboarding: { restatePrompt: "r", rubricRows: [], planSteps: [] },
};

describe("getProject", () => {
  it("parses a valid StudioProjection", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(new Response(JSON.stringify(sample), { status: 200, headers: { "Content-Type": "application/json" } }));
    const p = await getProject("00000000-0000-0000-0000-000000000101");
    expect(p.activeStation).toBe("S4");
    expect(p.coach.messages[0]).toMatchObject({ kind: "ai", anchor: "论证图 · 治理决心主张" });
  });

  it("throws on a malformed projection (schema drift)", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(new Response(JSON.stringify({ ...sample, activeStation: "S9" }), { status: 200, headers: { "Content-Type": "application/json" } }));
    await expect(getProject("x")).rejects.toThrow();
  });
});
