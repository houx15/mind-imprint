import { EvaluationReport } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

export interface EvalReportListEntry {
  projectId: string;
  title: string;
  type: string;
  createdAt: string;
}

// Backend three-state envelope (2026-08-14 retirement pass): the read/generate
// endpoints no longer return the bare report-or-null. `null` still means "no
// row at all"; once a row exists it's one of these three states so the page
// can poll while it's generating instead of racing the async generator.
export type EvalReportEnvelope =
  | { status: "generating" }
  | { status: "failed" }
  | { status: "ready"; report: EvaluationReport };

function parseEnvelope(raw: unknown): EvalReportEnvelope | null {
  if (raw == null) return null;
  const o = raw as { status?: string; report?: unknown };
  if (o.status === "ready") return { status: "ready", report: EvaluationReport.parse(o.report) };
  if (o.status === "generating") return { status: "generating" };
  if (o.status === "failed") return { status: "failed" };
  return null;
}

export async function getEvaluationReport(projectId: string): Promise<EvalReportEnvelope | null> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/evaluation-report`);
  return parseEnvelope(raw);
}

export async function generateEvaluationReport(projectId: string): Promise<EvalReportEnvelope | null> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/evaluation-report/generate`, { method: "POST" });
  return parseEnvelope(raw);
}

export async function listEvaluationReports(): Promise<EvalReportListEntry[]> {
  const raw = (await apiFetch<unknown>("/api/v1/evaluation-reports")) as { entries: EvalReportListEntry[] };
  return raw?.entries ?? [];
}
