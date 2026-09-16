import { apiFetch } from "./client";
import type { ArtifactTrialDraft } from "./artifactTrial";
import type { KeepEntry } from "./lookback";
const base = (project: string, artifact: string) => `/api/v1/pbl/projects/${project}/artifacts/${artifact}/trial-draft`;
export const getArtifactTrialDraft = (project: string, artifact: string) => apiFetch<ArtifactTrialDraft>(base(project, artifact));
export const saveArtifactTrialDraft = (project: string, artifact: string, draft: ArtifactTrialDraft) => apiFetch<ArtifactTrialDraft>(base(project, artifact), { method: "PUT", body: JSON.stringify(draft) });
export const submitArtifactTrialDraft = (project: string, artifact: string, revision: number) => apiFetch<{entry: KeepEntry; draft: ArtifactTrialDraft}>(`${base(project, artifact)}/submit`, { method: "POST", body: JSON.stringify({revision}) });
