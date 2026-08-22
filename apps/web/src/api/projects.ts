import { CreateProjectResult, ProjectStatus } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

export type ProjectListItem = {
  id: string;
  title: string;
  qualLabel: string;
  status: ProjectStatus;
  /** Raw stored cover value ("img:<n>" / "grad:<name>" / ""). */
  cover: string;
  /** Signed CDN URL for an "img:" cover; "" for "grad:"/unset covers. */
  coverUrl: string;
  /** RFC3339 start date — shown on the project list (title · 开始于 <date> · status). */
  createdAt: string;
  /** RFC3339 last-activity date — the 最近 chip and the list's sort key. */
  lastActiveAt: string;
  /** Real AI calls made for this project (llm_call rows) — the AI chip. */
  aiCalls: number;
  /** Activity-log entries for this project — the 活动 chip. */
  activityLog: number;
};

export async function listProjects(): Promise<ProjectListItem[]> {
  const res = await apiFetch<{ projects: ProjectListItem[] }>("/api/v1/projects");
  return res.projects;
}

// A3: the project terminal — 完成回顾 kicks off the flagship process assessment
// (which can take minutes) in the background and returns immediately with the
// project's new status ("evaluating"). The report is polled for via listProjects
// (status → "done") and read from the growth report, not returned here.
export async function finishProject(id: string): Promise<{ status: ProjectStatus }> {
  const raw = await apiFetch<{ status: ProjectStatus }>(`/api/v1/projects/${id}/finish`, { method: "POST" });
  return { status: ProjectStatus.parse(raw.status) };
}

// #20 · the 完成写作 milestone, now per-document (Phase B: `doc` selects the
// proposal or essay). Locks that document read-only; the essay's finish unlocks
// the 回顾 room. Idempotent server-side; 422 draft_empty when the draft is empty.
export async function finishWriting(id: string, doc: "proposal" | "essay" = "essay"): Promise<{ writingFinished: boolean }> {
  return apiFetch<{ writingFinished: boolean }>(`/api/v1/projects/${id}/finish-writing?doc=${doc}`, { method: "POST" });
}

// #20 (铁律②) · 重新打开写作 — reversible: clears the requested document's milestone
// so it is editable again. 409 already_finalizing once the project is evaluating/done.
export async function reopenWriting(id: string, doc: "proposal" | "essay" = "essay"): Promise<{ writingFinished: boolean }> {
  return apiFetch<{ writingFinished: boolean }>(`/api/v1/projects/${id}/reopen-writing?doc=${doc}`, { method: "POST" });
}

// slice 3a (Finding 2) · non-blocking skip for the 反例 prompt — sets
// counterpointsWaived so the framework can generate the plan without an
// articulated counterexample (铁律②).
export async function waiveCounterpoints(id: string): Promise<{ counterpointsWaived: boolean }> {
  return apiFetch<{ counterpointsWaived: boolean }>(`/api/v1/projects/${id}/framework/waive-counterpoints`, { method: "POST" });
}

export async function createProject(body: {
  title?: string;
  prompt: string;
  projectType?: string;
  writingLanguage?: "en" | "zh" | "bilingual";
  cover?: string;
}): Promise<{ id: string }> {
  const raw = await apiFetch<unknown>(`/api/v1/projects`, { method: "POST", body: JSON.stringify(body) });
  return CreateProjectResult.parse(raw);
}

// Rename a project (the student renaming their own workspace). Returns the
// trimmed/clamped title the server stored.
export async function renameProject(id: string, title: string): Promise<{ title: string }> {
  return apiFetch<{ title: string }>(`/api/v1/projects/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ title }),
  });
}

// Task 3: the create-drawer's cover picker — every pre-uploaded photo cover
// ("img:<n>") with a signed CDN URL. Resilient like listProjects: trust the
// shape, fall back to an empty list rather than throw on a missing field.
export async function getProjectCovers(): Promise<{ key: string; url: string }[]> {
  const raw = await apiFetch<{ covers?: { key: string; url: string }[] }>(`/api/v1/project-covers`);
  return raw.covers ?? [];
}

