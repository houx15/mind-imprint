import type { Reading } from "../api/readings";
import type { Writing } from "../api/writings";
import { readingPath, writingPath } from "../routing";

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
// existing history screen. Prefix IDs because two collections share a list.
//
// 🚨 2026-09-21 项目那一格从底栏藏掉了（还没做完），所以这里也不再收项目条 ——
// 底栏进不去、首页却还列着它，点下去是一个她不该看到的半成品。
// 藏的是入口，不是数据：项目本身一条没动，路由也还在。
export function recentLearning(
  readings: Reading[],
  writings: Writing[],
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
  ]
    .sort(
      (a, b) =>
        (Date.parse(b.time) || 0) - (Date.parse(a.time) || 0) ||
        a.key.localeCompare(b.key),
    )
    .slice(0, 3);
}
