import { apiFetch } from "./client";
import type { NoteKind } from "./notes";

/**
 * 出门前的观察清单。
 *
 * 🚨 「观察日记」一直没有 before-state：她带着一段话出门，回来面对几个空白框，
 * 于是写下的是「大家好像都挺忙的」——那是印象，不是观察。
 *
 * 清单是那个 before-state：印记按学生的时间和条件安排可执行的观察任务。
 * 当前版本中已勾选的任务会成为填好类别的便签底稿；历史任务不重新生成底稿。
 *
 * 没点掉的那几条同样进回灌——「第 2 条（听一句原话）没做到」是铁律④要的信号。
 */

const base = (projectId: string) => `/api/v1/pbl/projects/${projectId}`;

export interface MissionItem {
	version: string;
	editedByStudent: boolean;
  id: string;
  prompt: string;
  /** 这一条要带回哪一类便签。空 = 她自己判断。 */
  wantKind: NoteKind | "";
  ordinal: number;
  /** 她在现场点掉的时刻。null = 还没做到。 */
  doneAt: string | null;
  supersededAt: string | null;
}

export function editMission(projectId: string, item: MissionItem, prompt: string): Promise<MissionItem[]> {
  return apiFetch(`${base(projectId)}/mission/${item.id}`, {method: "PUT", body: JSON.stringify({prompt, version: item.version})});
}

export function listMission(projectId: string, toolId: string): Promise<MissionItem[]> {
  return apiFetch<MissionItem[]>(`${base(projectId)}/tools/${toolId}/mission`);
}

/** 点掉一条；再点一下是取消——现场点错了不该没法反悔。 */
export function tickMission(
  projectId: string,
  itemId: string,
  done: boolean,
): Promise<MissionItem> {
  return apiFetch<MissionItem>(`${base(projectId)}/mission/${itemId}`, {
    method: "PATCH",
    body: JSON.stringify({ done }),
  });
}
