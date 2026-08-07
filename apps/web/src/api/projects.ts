import { CreateProjectResult, ProjectStatus } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

export type ProjectListItem = {
  id: string;
  title: string;
  qualLabel: string;
  activeStation: string;
  status: ProjectStatus;
  /** Raw stored cover value ("img:<n>" / "grad:<name>" / ""). */
  cover: string;
  /** Signed CDN URL for an "img:" cover; "" for "grad:"/unset covers. */
  coverUrl: string;
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

// #20 · the 完成写作 milestone. Locks the draft read-only and unlocks the 回顾
// room. Idempotent server-side; 422 draft_empty when the draft has no content.
export async function finishWriting(id: string): Promise<{ writingFinished: boolean }> {
  return apiFetch<{ writingFinished: boolean }>(`/api/v1/projects/${id}/finish-writing`, { method: "POST" });
}

// #20 (铁律②) · 重新打开写作 — reversible: clears the milestone so the draft is
// editable again. 409 already_finalizing once the project is evaluating/done.
export async function reopenWriting(id: string): Promise<{ writingFinished: boolean }> {
  return apiFetch<{ writingFinished: boolean }>(`/api/v1/projects/${id}/reopen-writing`, { method: "POST" });
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

// Task 3: the create-drawer's cover picker — every pre-uploaded photo cover
// ("img:<n>") with a signed CDN URL. Resilient like listProjects: trust the
// shape, fall back to an empty list rather than throw on a missing field.
export async function getProjectCovers(): Promise<{ key: string; url: string }[]> {
  const raw = await apiFetch<{ covers?: { key: string; url: string }[] }>(`/api/v1/project-covers`);
  return raw.covers ?? [];
}

export async function submitOnboarding(projectId: string, body: { restate: string; weakPicks: number[] }): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/onboarding`, { method: "POST", body: JSON.stringify(body) });
}

export async function submitSelfScore(projectId: string, body: { scores: { code: string; band: number }[] }): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/self-score`, { method: "POST", body: JSON.stringify(body) });
}

export async function submitReflection(projectId: string, body: { text: string }): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/reflection`, { method: "POST", body: JSON.stringify(body) });
}

// N3d Task 9: S1 立题's whole-panel submit — mirrors submitOnboarding's shape
// (a plain DB write, not autosave); terms/answers/searchPlan travel verbatim.
export async function submitFraming(
  projectId: string,
  body: { terms: { term: string; definition: string }[]; answers: string[]; searchPlan: string[] },
): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/framing`, { method: "POST", body: JSON.stringify(body) });
}

// N3d Task 9: S2 视角与素材's whole-panel submit — level is one of
// national/global_for/global_against (the binding design's own three levels,
// dc.html:2158), kept as a plain string here (not the Zod enum) so a
// perspective-matrix-minted row with no level can still round-trip.
export async function submitPerspectives(
  projectId: string,
  body: { perspectives: { text: string; level: string }[] },
): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/perspectives`, { method: "POST", body: JSON.stringify(body) });
}

// N6-E Task 6: re-opens a `waived` station (the journey composer skipped it
// for this student) so she can walk it after all — mirrors submitOnboarding/
// submitPerspectives above (a plain POST, no body, no response to parse).
export async function reopenStation(projectId: string, code: string): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/journey/reopen/${code}`, { method: "POST" });
}
