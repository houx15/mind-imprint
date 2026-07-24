import { describe, it, expect, vi, afterEach } from "vitest";
import { getCourseAssessment, generateCourseAssessment } from "@/api/courseAssessment";

afterEach(() => { vi.restoreAllMocks(); });

// Real DTO shape (checked against packages/contracts/src/dualAxisReport.ts):
// a full DualAxisReport, reusing the exact fixture shape from Task 7's
// apps/web/test/shell/report/DualAxisReport.test.tsx so it parses .strict().
const dto = {
  depthAxis: [
    { code: "D1", name: "任务理解与问题表述", level: "L3", evidence: "限定判断", promptEvidence: "R4" },
    { code: "D2", name: "证据与信源", level: "L2", evidence: "NASA", promptEvidence: "" },
    { code: "D3", name: "论证结构", level: "L3", evidence: "warrant", promptEvidence: "" },
    { code: "D4", name: "视角与偏见", level: "L2", evidence: "样本局限", promptEvidence: "" },
    { code: "D5", name: "反馈处理与修订", level: "L3", evidence: "理由", promptEvidence: "" },
    { code: "D6", name: "反思与元认知", level: "NA", evidence: "未见自写反思", promptEvidence: "" },
  ],
  autonomyAxis: [
    { code: "A1", name: "方向自主", level: 3, opportunity: "given_taken", evidence: "入场即设边界", promptEvidence: "" },
    { code: "A2", name: "发起自主", level: 2, opportunity: "given_taken", evidence: "主动补查", promptEvidence: "" },
    { code: "A3", name: "边界主权", level: 1, opportunity: "given_not_taken", evidence: "偶有边界句", promptEvidence: "" },
    { code: "A4", name: "对抗与检验", level: 0, opportunity: "not_supplied", evidence: "未出现对手邀请", promptEvidence: "" },
    { code: "A5", name: "判断署名", level: 2, opportunity: "given_taken", evidence: "自评了档位", promptEvidence: "" },
    { code: "A6", name: "求真优先", level: 1, opportunity: "given_not_taken", evidence: "未主动收窄结论", promptEvidence: "" },
  ],
  promptLens: {
    stats: [
      { label: "主动指令轮", value: "3 / 10" },
      { label: "边界设定", value: "3 次" },
      { label: "对手邀请", value: "0 次" },
    ],
    lenses: [
      { code: "L_decisions", name: "五个决定完整度", level: 2, evidence: "…" },
      { code: "L_maturity", name: "提示成熟度", level: 3, evidence: "…" },
      { code: "L_boundary", name: "边界意识", level: 2, evidence: "…" },
      { code: "L_adversary", name: "对手邀请", level: 0, evidence: "…" },
      { code: "L_directive", name: "主动指令率", level: 3, evidence: "…" },
      { code: "L_acceptance", name: "验收标准自给", level: 2, evidence: "…" },
    ],
    note: "提示词透镜只读 AI 互动痕迹，为双轴补过程证据；不是第三根评分轴，不并入任何总分。",
  },
  interactionEvidence: [
    { round: 4, student: "检查是否回扣 thesis", aiSummary: "齐备", signal: "D1 主动限定" },
  ],
  narrative: "深度侧 L3 结构稳定复现，自主侧未主动召唤对手。",
  guidance: { nextSteps: [{ title: "下一步强化 D3", task: "跑一张 SIFT 记录" }] },
  axiom: "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定",
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
      JSON.stringify({ ...dto, depthAxis: [{ ...dto.depthAxis[0], level: "L9" }, ...dto.depthAxis.slice(1)] }),
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
