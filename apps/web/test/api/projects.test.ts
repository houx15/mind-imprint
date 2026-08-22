import { describe, it, expect, vi, afterEach } from "vitest";
import { listProjects, createProject } from "@/api/projects";

afterEach(() => { vi.restoreAllMocks(); });

describe("listProjects", () => {
  it("unwraps the {projects} envelope into a bare array", async () => {
    const body = {
      projects: [
        { id: "00000000-0000-0000-0000-000000000101", title: "T", qualLabel: "0457 个人报告", status: "working" },
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
