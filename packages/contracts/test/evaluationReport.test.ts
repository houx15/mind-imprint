import { describe, it, expect } from "vitest";
import { EvaluationReport } from "../src/evaluationReport";

const MIN = {
  version: 1, reportId: "rep-1", projectId: "p-1",
  student: { id: "u-1", name: "Phoebe" },
  basics: {
    title: "T", type: "Extended Essay", startDate: "2026-08-01T00:00:00Z", endDate: null,
    milestones: { started: "2026-08-01T00:00:00Z", frameworkFinished: null, proposalFinished: null, writingFinished: null, projectFinished: null },
    counters: { aiTurns: 0, materialsRead: 0, wordsWritten: 0, aiCommentCount: 0, editCount: 0 },
  },
  abstract: {
    overview: "o", materialSentence: "m", writingSentence: "w", aiSentence: "a",
    suggestionParagraph: "s", suggestionSentences: ["s1"], recommendedCourses: [{ courseId: "c-1", reason: "r" }],
  },
  events: [{ ts: "2026-08-01T00:00:00Z", kind: "chat", summary: "s", aiTurns: 2 }],
  materials: [{ materialId: "m-1", addedAt: "2026-08-01T00:00:00Z", source: "NASA", url: null, usedIn: null, finalStatus: "bridge source", comment: "c", cannotSupport: "x" }],
  depth: [{ id: "D1", level: 3, summary: "s", evidence: [{ id: "msg-1", ts: "2026-08-01T00:00:00Z", stage: "立题", quote: "q", observation: "obs", boundary: "b" }], suggestion: "sg" }],
  autonomy: [{ id: "A1", band: 4, summary: "s", evidence: [], suggestion: "sg" }],
  promptLens: { summary: "s", prompts: [{ stage: "写作", quote: "q", ref: { id: "msg-2" }, observation: "o", relatedDomains: ["A3", "D5"], attention: false }] },
  toolUsage: [{ toolId: "card:craap", name: "CRAAP", stage: "阅读", purpose: "p", summary: "s" }],
  risks: [{ type: "data-scope", behaviour: "b", suggestion: "sg" }],
  generatedAt: "2026-08-06T00:00:00Z",
};

describe("EvaluationReport", () => {
  it("parses a minimal valid report", () => {
    expect(() => EvaluationReport.parse(MIN)).not.toThrow();
  });
  it("rejects a bad depth level", () => {
    const bad = structuredClone(MIN); (bad.depth[0] as any).level = 5;
    expect(() => EvaluationReport.parse(bad)).toThrow();
  });
  it("rejects an unknown risk type", () => {
    const bad = structuredClone(MIN); (bad.risks[0] as any).type = "nope";
    expect(() => EvaluationReport.parse(bad)).toThrow();
  });
  // Regression (2026-08-25): older stored reports serialized empty depth/autonomy
  // evidence as JSON `null` (a nil Go slice). The Go validator only boundary-checks
  // the envelope, so those rows reach the client and served `ready`; a strict
  // `z.array(...)` here threw and blanked the whole report ("报告加载失败" on both the
  // teacher view and the student's own page). Null must coerce to [].
  it("tolerates null depth/autonomy evidence, coercing to []", () => {
    const legacy = structuredClone(MIN);
    (legacy.depth[0] as any).evidence = null;
    (legacy.autonomy[0] as any).evidence = null;
    const parsed = EvaluationReport.parse(legacy);
    expect(parsed.depth[0]!.evidence).toEqual([]);
    expect(parsed.autonomy[0]!.evidence).toEqual([]);
  });
});
