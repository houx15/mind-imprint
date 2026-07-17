import { describe, it, expect, vi, afterEach } from "vitest";
import { getCourseAssessment, generateCourseAssessment } from "./courseAssessment";

afterEach(() => { vi.restoreAllMocks(); });

const dto = {
  dimensions: [
    { code: "D2", name: "信源辨识", level: "L4", evidence: "溯源到 NASA 与 Nature Sustainability。" },
  ],
  narrative: "你在这次任务中主动溯源、并正面处理了反例。",
  generatedAt: "2026-07-13T12:34:56Z",
};

describe("getCourseAssessment", () => {
  it("GETs .../session/assessment (scoped by course id, no session id in the URL) and parses the DTO", async () => {
    const spy = vi.fn(async () => new Response(
      JSON.stringify(dto),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const result = await getCourseAssessment("co1");

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/courses/co1/session/assessment");
    expect(init?.method ?? "GET").toBe("GET");
    expect(result).toEqual(dto);
  });

  it("returns null when the server responds with a bare JSON null (no assessment yet)", async () => {
    const spy = vi.fn(async () => new Response(
      "null",
      { status: 200, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const result = await getCourseAssessment("co1");

    expect(result).toBeNull();
  });

  it("throws on a malformed DTO (schema drift)", async () => {
    const spy = vi.fn(async () => new Response(
      JSON.stringify({ dimensions: [{ code: "D2", name: "信源辨识", level: "卓越", evidence: "e" }], narrative: "n", generatedAt: "t" }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    await expect(getCourseAssessment("co1")).rejects.toThrow();
  });
});

describe("generateCourseAssessment", () => {
  it("POSTs to .../session/assessment and parses the freshly generated DTO", async () => {
    const spy = vi.fn(async () => new Response(
      JSON.stringify(dto),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    const result = await generateCourseAssessment("co1");

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/courses/co1/session/assessment");
    expect(init.method).toBe("POST");
    expect(result).toEqual(dto);
  });

  it("surfaces the server's Chinese error message on failure, verbatim", async () => {
    const spy = vi.fn(async () => new Response(
      JSON.stringify({ error: { code: "assessment_rejected", message: "这次评估没通过内部校验，请再试一次" } }),
      { status: 422, headers: { "Content-Type": "application/json" } },
    ));
    vi.stubGlobal("fetch", spy);

    await expect(generateCourseAssessment("co1")).rejects.toThrow("这次评估没通过内部校验，请再试一次");
  });
});
