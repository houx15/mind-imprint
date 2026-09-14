import type { Reading } from "../api/readings";
import type { Writing } from "../api/writings";
import { PROJECT_STATUS_LABELS, type Project } from "../api/projects";
import { projectPath, readingPath, writingPath } from "../routing";

export interface RecentLearning {
  key: string;
  title: string;
  kind: string;
  status: string;
  path: string;
  time: string;
  detail?: string;
}

// Compare actual activity, not metadata edits; completed work stays in its
// existing history screen. Prefix IDs because three collections share a list.
export function recentLearning(
  readings: Reading[],
  writings: Writing[],
  projects: Project[],
): RecentLearning[] {
  return [
    ...readings
      .filter((r) => r.status !== "finished" && !r.finishedAt)
      .map((r) => ({
        key: `reading:${r.id}`,
        title: r.title || "未命名阅读",
        kind: "阅读",
        status: "阅读中",
        path: readingPath(r.id),
        time: r.lastActivityAt || r.updatedAt,
      })),
    ...writings
      .filter((w) => w.status !== "finished" && !w.finishedAt)
      .map((w) => ({
        key: `writing:${w.id}`,
        title: w.title || "未命名写作",
        kind: "写作",
        status: "写作中",
        path: writingPath(w.id),
        time: w.lastActivityAt || w.updatedAt,
      })),
    ...projects
      .filter((p) => p.status !== "archived")
      .map((p) => ({
        key: `project:${p.id}`,
        title: p.name || p.idea || "未命名项目",
        kind: "项目",
        status: PROJECT_STATUS_LABELS[p.status],
        path: projectPath(p.id),
        time: p.lastActivityAt || p.createdAt,
        detail: p.currentStep,
      })),
  ]
    .sort(
      (a, b) =>
        (Date.parse(b.time) || 0) - (Date.parse(a.time) || 0) ||
        a.key.localeCompare(b.key),
    )
    .slice(0, 3);
}
