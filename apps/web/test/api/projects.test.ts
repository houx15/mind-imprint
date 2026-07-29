import { describe, it, expect, vi, afterEach } from "vitest";
import { listProjects, createProject, submitOnboarding, submitSelfScore, submitReflection, submitFraming, submitPerspectives } from "@/api/projects";

afterEach(() => { vi.restoreAllMocks(); });

describe("listProjects", () => {
  it("unwraps the {projects} envelope into a bare array", async () => {
    const body = {
      projects: [
        { id: "00000000-0000-0000-0000-000000000101", title: "T", qualLabel: "0457 个人报告", activeStation: "S4", status: "working" },
      ],
    };
    vi.spyOn(global, "fetch").mockResolvedValue(new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } }));
    const list = await listProjects();
    expect(list.length).toBe(1);
    expect(list[0]?.id).toBe("00000000-0000-0000-0000-000000000101");
  });
});

describe("createProject", () => {
  it("posts the body and parses the id", async () => {
    const spy = vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ id: "p-123" }), { status: 201, headers: { "Content-Type": "application/json" } }));
    const out = await createProject({ title: "T", prompt: "讨论 X" });
    expect(out.id).toBe("p-123");
    const [, init] = spy.mock.calls[0]!;
    expect(init?.method).toBe("POST");
    expect(JSON.parse(init?.body as string).prompt).toBe("讨论 X");
  });
});

describe("submitOnboarding", () => {
  it("posts restate + weakPicks", async () => {
    const spy = vi.spyOn(global, "fetch").mockResolvedValue(new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }));
    await submitOnboarding("p-1", { restate: "我的理解够长了吗", weakPicks: [0, 1] });
    const [url, init] = spy.mock.calls[0]!;
    expect(String(url)).toContain("/projects/p-1/onboarding");
    expect(JSON.parse(init?.body as string).weakPicks).toEqual([0, 1]);
  });
});

describe("submitSelfScore / submitReflection", () => {
  it("posts self-score", async () => {
    const spy = vi.spyOn(global, "fetch").mockResolvedValue(new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }));
    await submitSelfScore("p1", { scores: [{ code: "表D", band: 2 }] });
    const [url, init] = spy.mock.calls[0]!;
    expect(String(url)).toContain("/projects/p1/self-score");
    expect(JSON.parse(init!.body as string).scores[0].band).toBe(2);
  });
  it("posts reflection", async () => {
    const spy = vi.spyOn(global, "fetch").mockResolvedValue(new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }));
    await submitReflection("p1", { text: "我的回顾至少二十个字这样才够长可以通过校验规则" });
    expect(String(spy.mock.calls[0]![0])).toContain("/projects/p1/reflection");
  });
});

describe("submitFraming / submitPerspectives", () => {
  it("posts terms/answers/searchPlan to the framing endpoint", async () => {
    const spy = vi.spyOn(global, "fetch").mockResolvedValue(new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }));
    const body = { terms: [{ term: "可持续", definition: "长期不损害后代满足自身需求的能力" }], answers: ["还没有答案"], searchPlan: ["查 NASA 卫星数据"] };
    await submitFraming("p-1", body);
    const [url, init] = spy.mock.calls[0]!;
    expect(String(url)).toContain("/projects/p-1/framing");
    expect(init?.method).toBe("POST");
    expect(JSON.parse(init?.body as string)).toEqual(body);
  });

  it("posts perspectives to the perspectives endpoint", async () => {
    const spy = vi.spyOn(global, "fetch").mockResolvedValue(new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }));
    const body = { perspectives: [{ text: "中国官方立场：治理决心真实且持续", level: "national" }] };
    await submitPerspectives("p-1", body);
    const [url, init] = spy.mock.calls[0]!;
    expect(String(url)).toContain("/projects/p-1/perspectives");
    expect(init?.method).toBe("POST");
    expect(JSON.parse(init?.body as string)).toEqual(body);
  });
});
