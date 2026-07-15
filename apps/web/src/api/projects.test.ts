import { describe, it, expect, vi, afterEach } from "vitest";
import { getProject, listProjects } from "./projects";

afterEach(() => { vi.restoreAllMocks(); });

describe("listProjects", () => {
  it("unwraps the {projects} envelope into a bare array", async () => {
    const body = {
      projects: [
        { id: "00000000-0000-0000-0000-000000000101", title: "T", qualLabel: "0457 个人报告", activeStation: "S4" },
      ],
    };
    vi.spyOn(global, "fetch").mockResolvedValue(new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } }));
    const list = await listProjects();
    expect(list.length).toBe(1);
    expect(list[0]?.id).toBe("00000000-0000-0000-0000-000000000101");
  });
});

const sample = {
  project: { title: "T", qualLabel: "0457 个人报告" },
  stations: [{ code: "S4", name: "论证构建", view: "结构", state: "current", gate: { total: 7, passed: 2 } }],
  activeStation: "S4",
  coach: { anchor: "论证图 · 治理决心主张", messages: [{ kind: "ai", body: "b", tag: "D5", anchor: "论证图 · 治理决心主张" }], equipment: [] },
  onboarding: { restatePrompt: "r", rubricRows: [], planSteps: [] },
  materials: [],
  activeCard: null,
  structure: [],
  writing: {
    buffer: "",
    latestSnapshot: null,
    wordBudget: { min: 300, max: 500 },
    citationsMatched: false,
    review: { items: [] },
  },
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
