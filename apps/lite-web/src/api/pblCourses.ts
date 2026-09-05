import { apiFetch } from "./client";

// api/pblCourses.ts —— 项目里的「去上一课」。
// 形状读自 apps/api/internal/api/pbl_course.go · pblCourseDTO。

const base = (id: string) => `/api/v1/pbl/projects/${id}`;

export interface PblCourse {
  id: string;
  slug: string;
  /** 印记为什么这时候递这一课，用「你」跟她说的那句话。 */
  why: string;
  /** 她上完回来写下的那句：这一课对我这个项目有什么用。 */
  takeaway: string;
  finishedAt: string | null;
  createdAt: string;

  title: string;
  blurb: string;
  timeLabel: string;
  stepCount: number;
  coverUrl: string;
  /** 这门课现在还看得见吗。false = 下线了，或者不给这个版本了。 */
  available: boolean;
  /** 她在课程里的进度：""（没开始）/ "in-progress" / "completed"。
   *  和 finishedAt 是两件事：上完课 ≠ 写下了这一课有什么用。 */
  courseStatus: string;
  completedSteps: number;
}

export function listPblCourses(projectId: string): Promise<PblCourse[]> {
  return apiFetch<PblCourse[]>(`${base(projectId)}/courses`);
}

/**
 * 她上完回来，写下这一课对这个项目有什么用。
 *
 * 服务端拒绝空的 takeaway：回灌读的就是这一句，没有它，这次上课在对话里等于
 * 没发生。返回的是整份列表，省一次重拉。
 */
export function finishPblCourse(
  projectId: string,
  courseId: string,
  takeaway: string,
): Promise<PblCourse[]> {
  return apiFetch<PblCourse[]>(`${base(projectId)}/courses/${courseId}/finish`, {
    method: "POST",
    body: JSON.stringify({ takeaway }),
  });
}
