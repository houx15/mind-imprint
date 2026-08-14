import { apiFetch } from "./client";
import { parseEnvelope, type EvalReportEnvelope } from "./evaluationReport";

export type { EvalReportEnvelope } from "./evaluationReport";

export interface RosterReportEntry {
  id: string;
  displayName: string;
  avatarColor: string;
  dBadge: string;
  aBadge: string;
  activeDays: number;
  turns: number;
  hasReport: boolean;
  unrated: boolean;
}

export interface StudentRecord {
  surface: "project" | "course" | "chat";
  scopeId: string;
  title: string;
  date: string;
  status: string;
  hasReport: boolean;
}

export interface StudentDetail {
  student: { id: string; displayName: string; avatarColor: string; dBadge: string; aBadge: string; unrated: boolean };
  usage: { activeDays: number; turns: number; reportCount: number; courseCount: number };
  records: StudentRecord[];
}

export async function getClassRosterReport(classId: string): Promise<RosterReportEntry[]> {
  const r = await apiFetch<{ roster: RosterReportEntry[] }>(`/api/v1/classes/${classId}/roster-report`);
  return r.roster;
}

export async function getStudentDetail(classId: string, userId: string): Promise<StudentDetail> {
  return apiFetch<StudentDetail>(`/api/v1/classes/${classId}/students/${userId}`);
}

// Task 13: the teacher's read of the SAME EvaluationReport the student sees
// for one of their projects — read-no-call, mirrors ../api/evaluationReport's
// getEvaluationReport (null-passthrough, parsed via the shared contract).
export async function getStudentEvaluationReport(
  classId: string,
  userId: string,
  projectId: string,
): Promise<EvalReportEnvelope | null> {
  const raw = await apiFetch<unknown>(`/api/v1/classes/${classId}/students/${userId}/evaluation-report/${projectId}`);
  return parseEnvelope(raw);
}

export interface WeeklyCard {
  userId: string;
  displayName: string;
  avatarColor: string;
  tagCode: string;
  tagLabel: string;
  kind: "praise" | "watch";
  evidence: string;
  lead: string;
  action: string;
  hasReport: boolean;
  reportSurface?: string;
  reportScopeId?: string;
}

export interface WeeklyReport {
  weekLabel: string;
  weekStart: string;
  weekEnd: string;
  asOf: string;
  className: string;
  classSize: number;
  stats: { key: string; label: string; value: number; unit: string; foot: string; delta: string; deltaDir: "up" | "down" | "flat" }[];
  praise: WeeklyCard[];
  watch: WeeklyCard[];
  depth: { buckets: { code: string; label: string; count: number }[]; ratedCount: number; note: string };
  autonomy: { mean: string; delta: string; deltaDir: "up" | "down" | "flat"; ratedCount: number; note: string };
  comment: string | null;
  proseReady: boolean;
}

export async function getClassWeeklyReport(classId: string): Promise<WeeklyReport> {
  return apiFetch<WeeklyReport>(`/api/v1/classes/${classId}/weekly-report`);
}

export async function generateClassWeeklyProse(classId: string): Promise<WeeklyReport> {
  return apiFetch<WeeklyReport>(`/api/v1/classes/${classId}/weekly-report/prose`, { method: "POST" });
}
