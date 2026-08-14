import { describe, it, expect, vi, afterEach } from "vitest";
import { getEvaluationReport, generateEvaluationReport, listEvaluationReports } from "@/api/evaluationReport";

afterEach(() => { vi.restoreAllMocks(); });

// Minimal valid EvaluationReport DTO (checked against packages/contracts/src/evaluationReport.ts)
const mockReportDto = {
  version: 1 as const,
  reportId: "report-1",
  projectId: "proj-1",
  student: { id: "student-1", name: "Test Student" },
  basics: {
    title: "Test Project",
    type: "COMPREHENSIVE",
    startDate: "2026-08-01T00:00:00Z",
    endDate: null,
    milestones: {
      started: "2026-08-01T00:00:00Z",
      frameworkFinished: null,
      proposalFinished: null,
      writingFinished: null,
      projectFinished: null,
    },
    counters: {
      aiTurns: 5,
      materialsRead: 3,
      wordsWritten: 1000,
      aiCommentCount: 2,
      editCount: 1,
    },
  },
  abstract: {
    overview: "Test overview",
    materialSentence: "Used 3 materials",
    writingSentence: "Wrote 1000 words",
    aiSentence: "AI assisted 5 turns",
    suggestionParagraph: "Consider deepening analysis",
    suggestionSentences: ["Explore alternative perspectives"],
    recommendedCourses: [],
  },
  events: [],
  materials: [],
  depth: [],
  autonomy: [],
  promptLens: {
    summary: "Prompt analysis summary",
    prompts: [],
  },
  toolUsage: [],
  risks: [],
  generatedAt: "2026-08-14T12:00:00Z",
};

describe("evaluationReport API client", () => {
  describe("getEvaluationReport", () => {
    it("GETs /api/v1/projects/{id}/evaluation-report and parses a ready envelope's report", async () => {
      const spy = vi.fn(async () => new Response(
        JSON.stringify({ status: "ready", report: mockReportDto }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ));
      vi.stubGlobal("fetch", spy);

      const result = await getEvaluationReport("proj-1");

      const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
      expect(url).toContain("/api/v1/projects/proj-1/evaluation-report");
      expect(init?.method ?? "GET").toBe("GET");
      expect(result).toEqual({ status: "ready", report: mockReportDto });
    });

    it("returns null when the server responds with a bare JSON null (no row yet)", async () => {
      const spy = vi.fn(async () => new Response(
        "null",
        { status: 200, headers: { "Content-Type": "application/json" } },
      ));
      vi.stubGlobal("fetch", spy);

      const result = await getEvaluationReport("proj-1");

      expect(result).toBeNull();
    });

    it("passes through {status:\"generating\"}", async () => {
      const spy = vi.fn(async () => new Response(
        JSON.stringify({ status: "generating" }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ));
      vi.stubGlobal("fetch", spy);

      const result = await getEvaluationReport("proj-1");

      expect(result).toEqual({ status: "generating" });
    });

    it("passes through {status:\"failed\"}", async () => {
      const spy = vi.fn(async () => new Response(
        JSON.stringify({ status: "failed" }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ));
      vi.stubGlobal("fetch", spy);

      const result = await getEvaluationReport("proj-1");

      expect(result).toEqual({ status: "failed" });
    });

    it("throws on a malformed DTO inside a ready envelope (schema drift)", async () => {
      const spy = vi.fn(async () => new Response(
        JSON.stringify({ status: "ready", report: { ...mockReportDto, version: 2 } }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ));
      vi.stubGlobal("fetch", spy);

      await expect(getEvaluationReport("proj-1")).rejects.toThrow();
    });
  });

  describe("generateEvaluationReport", () => {
    it("POSTs to /api/v1/projects/{id}/evaluation-report/generate and parses a ready envelope's report", async () => {
      const spy = vi.fn(async () => new Response(
        JSON.stringify({ status: "ready", report: mockReportDto }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ));
      vi.stubGlobal("fetch", spy);

      const result = await generateEvaluationReport("proj-1");

      const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
      expect(url).toContain("/api/v1/projects/proj-1/evaluation-report/generate");
      expect(init?.method).toBe("POST");
      expect(result).toEqual({ status: "ready", report: mockReportDto });
    });

    it("returns {status:\"generating\"} for a real (non-instant) generator", async () => {
      const spy = vi.fn(async () => new Response(
        JSON.stringify({ status: "generating" }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ));
      vi.stubGlobal("fetch", spy);

      const result = await generateEvaluationReport("proj-1");

      expect(result).toEqual({ status: "generating" });
    });

    it("returns null when the server responds with a bare JSON null", async () => {
      const spy = vi.fn(async () => new Response(
        "null",
        { status: 200, headers: { "Content-Type": "application/json" } },
      ));
      vi.stubGlobal("fetch", spy);

      const result = await generateEvaluationReport("proj-1");

      expect(result).toBeNull();
    });
  });

  describe("listEvaluationReports", () => {
    it("GETs /api/v1/evaluation-reports and returns entries array", async () => {
      const listResponse = {
        entries: [
          {
            projectId: "proj-1",
            title: "Project One",
            type: "COMPREHENSIVE",
            createdAt: "2026-08-14T10:00:00Z",
          },
          {
            projectId: "proj-2",
            title: "Project Two",
            type: "QUICK",
            createdAt: "2026-08-14T11:00:00Z",
          },
        ],
      };
      const spy = vi.fn(async () => new Response(
        JSON.stringify(listResponse),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ));
      vi.stubGlobal("fetch", spy);

      const result = await listEvaluationReports();

      const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
      expect(url).toContain("/api/v1/evaluation-reports");
      expect(init?.method ?? "GET").toBe("GET");
      expect(result).toHaveLength(2);
      expect(result[0]?.projectId).toBe("proj-1");
      expect(result[1]?.projectId).toBe("proj-2");
    });

    it("returns empty array when entries is empty", async () => {
      const spy = vi.fn(async () => new Response(
        JSON.stringify({ entries: [] }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ));
      vi.stubGlobal("fetch", spy);

      const result = await listEvaluationReports();

      expect(result).toEqual([]);
    });

    it("handles undefined entries gracefully", async () => {
      const spy = vi.fn(async () => new Response(
        JSON.stringify({}),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ));
      vi.stubGlobal("fetch", spy);

      const result = await listEvaluationReports();

      expect(result).toEqual([]);
    });
  });
});
