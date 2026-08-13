import { EvaluationReport } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

export interface EvalReportListEntry {
  projectId: string;
  title: string;
  type: string;
  createdAt: string;
}

export async function getEvaluationReport(projectId: string): Promise<EvaluationReport | null> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/evaluation-report`);
  if (raw == null) return null;
  return EvaluationReport.parse(raw);
}

export async function generateEvaluationReport(projectId: string): Promise<EvaluationReport | null> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/evaluation-report/generate`, { method: "POST" });
  if (raw == null) return null;
  return EvaluationReport.parse(raw);
}

export async function listEvaluationReports(): Promise<EvalReportListEntry[]> {
  const raw = (await apiFetch<unknown>("/api/v1/evaluation-reports")) as { entries: EvalReportListEntry[] };
  return raw?.entries ?? [];
}
