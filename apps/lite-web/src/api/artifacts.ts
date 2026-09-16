import { apiFetch } from "./client";

// api/artifacts.ts —— 印记交出来的东西。形状读自 apps/api/internal/api/pbl_artifacts.go。

const base = (id: string) => `/api/v1/pbl/projects/${id}`;

export type ArtifactKind = "options" | "draft" | "spec" | "image" | "site" | "html";

export interface Artifact {
  superseded?: boolean;
	stale?: boolean;
  id: string;
  kind: ArtifactKind;
  title: string;
  /** 文档放 body，图片和网站放 url。二进制一律在 OSS，这里只有 key/URL。 */
  payload: { body?: string; url?: string; [k: string]: unknown };
  /** 印记自己说的：我猜了什么。 */
  guessed: string[];
  /** 印记自己说的：这一版还有什么不对。 */
  admits: string[];
  verdict: "kept" | "revise" | "dropped" | null;
  why: string;
  settledAt: string | null;
  createdAt: string;
}

export function listArtifacts(projectId: string): Promise<Artifact[]> {
  return apiFetch<Artifact[]>(`${base(projectId)}/artifacts`);
}

export function editArtifactText(projectId: string, artifactId: string, body: string): Promise<Artifact> {
  return apiFetch<Artifact>(`${base(projectId)}/artifacts/${artifactId}/text`, {
    method: "PUT", body: JSON.stringify({body}),
  });
}

export function refreshSiteReview(projectId: string, artifactId: string): Promise<Artifact[]> {
  return apiFetch<Artifact[]>(`${base(projectId)}/artifacts/${artifactId}/refresh-site`, { method: "POST" });
}

export function settleArtifact(
  projectId: string,
  artifactId: string,
  verdict: "kept" | "revise" | "dropped",
  why: string,
): Promise<Artifact> {
  return apiFetch<Artifact>(`${base(projectId)}/artifacts/${artifactId}/settle`, {
    method: "POST",
    body: JSON.stringify({ verdict, why }),
  });
}

/** 读得出字的那几种。图片和网站直接看，没有句子可划。 */
export function isDocument(a: Artifact): boolean {
  return a.kind === "draft" || a.kind === "spec" || a.kind === "options" || (a.kind === "site" && Boolean(a.payload.body));
}

/** 还没定的，最新的排最前——审阅默认审她手边这一件。 */
export function pendingArtifacts(all: Artifact[]): Artifact[] {
  return all.filter((a) => !a.settledAt && !a.superseded).slice().reverse();
}
