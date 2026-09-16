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

// 产品负责人 2026-09-02 定的说法。原来那套（在聊 / 在做 / 在养着 / 收起来了）
// 太口语，读起来不像一个正经的项目状态。
export const PROJECT_STATUS_LABELS: Record<ProjectStatus, string> = {
  talking: "构思中",
  running: "进行中",
  review: "待复盘",
  keeping: "已落地",
  archived: "已归档",
};

/**
 * 项目类别。
 *
 * 🚨 由她自己选，不由 AI 判（产品负责人 2026-09-02）。刚建出来的项目类别是
 * 空的——那不是缺数据，是她还没想好。
 *
 * 存的就是这里的中文本身（迁移 0112 把 kind 放开成自由字符串），所以加一个
 * 类别就是这张单子上多一行，不用改数据库、也不用发一次版。
 */
export const PROJECT_KINDS = [
  "网站搭建",
  "内容设计",
  "田野调查",
  "数据分析",
  "产品原型",
  "活动策划",
  "研究写作",
] as const;
export type ProjectKind = (typeof PROJECT_KINDS)[number];

/** 分类器时代留下的英文值，照旧显示，不改写历史数据。 */
const LEGACY_KIND_LABELS: Record<string, string> = {
  website: "网站搭建",
  research: "研究写作",
  design: "内容设计",
  making: "产品原型",
  investigation: "田野调查",
};

/** 界面上显示的类别名。空 = 还没定。 */
export function kindLabel(kind: string): string {
  const k = kind.trim();
  if (!k) return "";
  return LEGACY_KIND_LABELS[k] ?? k;
}

export interface Project {
  planPending?: boolean;
  id: string;
  /** Her own opening sentence, stored verbatim. Shown on the kanban card until
   *  she names the project, and kept afterwards — it is the only record of how
   *  she first put it. */
  idea: string;
  /** 她选的类别；空 = 还没定。自由字符串，见 PROJECT_KINDS。 */
  kind: string;
  /** 便签板的坐标视图开着没有。见 migration 0122。 */
  boardAxes: boolean;
  name: string;
  coverGround: string;
  coverGlyph: string;
  status: ProjectStatus;
  createdAt: string;
  lastActivityAt: string;
  /** 现在走到的那一步。没有计划时是空串。 */
  currentStep: string;
  stepsDone: number;
  stepsTotal: number;
  /** The project came from an assignment: `idea` is the teacher's driving
   *  question, not her words. Missing (an older server) = her own project. */
  assigned?: boolean;
}

/** Whether `idea` is the teacher's driving question. The room uses this to
 *  decide not to post `idea` as her first turn. Only an explicit `true` counts. */
export function isAssignedProject(p: Pick<Project, "assigned">): boolean {
  return p.assigned === true;
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

/**
 * 开/关便签板的坐标视图。
 *
 * 🚨 这一位存在服务端而不是本地：印记要靠它判断板上的 x/y 是不是一句判断。
 * 没开过坐标视图的板子，位置是系统派的座位，读成「她觉得这条不要紧」是在编造。
 */
export function setBoardAxes(projectId: string, on: boolean): Promise<Project> {
  return apiFetch<Project>(`/api/v1/pbl/projects/${projectId}`, {
    method: "PATCH",
    body: JSON.stringify({ boardAxes: on }),
  });
}
