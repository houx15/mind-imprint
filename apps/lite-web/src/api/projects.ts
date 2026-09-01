import { apiFetch } from "./client";

// api/projects.ts — the 项目 tab's client. Shapes read straight off
// apps/api/internal/api/pbl_projects.go's pblProjectDTO, not guessed.
//
// 🚨 The paths are `/api/v1/pbl/projects`, NOT `/api/v1/projects`. That second
// one is pro's route and a lite student gets 404 on it by design — there is a
// test in edition_test.go asserting exactly that.

/** Mirrors pbl_project's status CHECK (migration 0108). The order here is the
 *  kanban's column order, left to right: it runs from "still just talking" to
 *  "put away", which is the direction a project actually travels. */
export const PROJECT_STATUSES = ["talking", "running", "review", "keeping", "archived"] as const;
export type ProjectStatus = (typeof PROJECT_STATUSES)[number];

export const PROJECT_STATUS_LABELS: Record<ProjectStatus, string> = {
  talking: "在聊",
  running: "在做",
  review: "复盘",
  keeping: "在养着",
  archived: "收起来了",
};

/** Mirrors pbl_project's kind CHECK, and pbl.ProjectKinds in Go. */
export const PROJECT_KINDS = ["website", "research", "design", "making", "investigation"] as const;
export type ProjectKind = (typeof PROJECT_KINDS)[number];

export const PROJECT_KIND_LABELS: Record<ProjectKind, string> = {
  website: "做个网站",
  research: "弄明白一个问题",
  design: "做个设计",
  making: "动手做出来",
  investigation: "去真实世界里看看",
};

export interface Project {
  id: string;
  /** Her own opening sentence, stored verbatim. Shown on the kanban card until
   *  she names the project, and kept afterwards — it is the only record of how
   *  she first put it. */
  idea: string;
  kind: ProjectKind;
  name: string;
  coverGround: string;
  coverGlyph: string;
  status: ProjectStatus;
  createdAt: string;
  lastActivityAt: string;
}

export function listProjects(): Promise<Project[]> {
  return apiFetch<Project[]>("/api/v1/pbl/projects");
}

/** Creates the project. The server classifies what she wrote and may take a
 *  model round-trip, so callers should show that something is happening. */
export function createProject(idea: string): Promise<Project> {
  return apiFetch<Project>("/api/v1/pbl/projects", {
    method: "POST",
    body: JSON.stringify({ idea }),
  });
}

export function updateProject(
  id: string,
  patch: Partial<Pick<Project, "name" | "coverGround" | "coverGlyph" | "status">>,
): Promise<Project> {
  return apiFetch<Project>(`/api/v1/pbl/projects/${id}`, {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
}

/** What the card shows as its title. An unnamed project is not "未命名" — it is
 *  the sentence she actually wrote, which says far more than a placeholder. */
export function projectTitle(p: Project): string {
  return p.name.trim() || p.idea.trim();
}

/**
 * Bucket projects into kanban columns.
 *
 * Every column is present even when empty, so the board keeps its shape as
 * projects move between columns — a column that disappears when it empties
 * makes the board jump under her hand.
 *
 * A status the server does not know is dropped rather than thrown on. This is
 * the forward-compatibility case: a future status must never blank the whole
 * board for someone on an older bundle.
 */
export function groupByStatus(projects: Project[]): Record<ProjectStatus, Project[]> {
  const out = Object.fromEntries(PROJECT_STATUSES.map((s) => [s, [] as Project[]])) as Record<
    ProjectStatus,
    Project[]
  >;
  for (const p of projects) {
    if (p.status in out) out[p.status].push(p);
  }
  return out;
}
