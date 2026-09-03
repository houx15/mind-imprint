import { apiFetch } from "./client";

// api/reframe.ts —— 把问题说清楚。形状读自 apps/api/internal/api/pbl_reframe.go。

const base = (id: string) => `/api/v1/pbl/projects/${id}`;

export interface Reframe {
  id: string;
  who: string;
  needs: string;
  why: string;
  hmw: string;
  /** 它替掉的上一版。留着旧的，她之后才看得见"我当时以为问题是这个"。 */
  supersedes: string | null;
  confirmedAt: string | null;
  createdAt: string;
}

export function listReframes(projectId: string): Promise<Reframe[]> {
  return apiFetch<Reframe[]>(`${base(projectId)}/reframes`);
}

export function createReframe(
  projectId: string,
  body: { who?: string; needs?: string; why?: string; hmw?: string; supersedes?: string },
): Promise<Reframe> {
  return apiFetch<Reframe>(`${base(projectId)}/reframes`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function updateReframe(
  projectId: string,
  id: string,
  patch: { who?: string; needs?: string; why?: string; hmw?: string },
): Promise<Reframe> {
  return apiFetch<Reframe>(`${base(projectId)}/reframes/${id}`, {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
}

export function confirmReframe(projectId: string, id: string): Promise<Reframe> {
  return apiFetch<Reframe>(`${base(projectId)}/reframes/${id}/confirm`, { method: "POST" });
}

/**
 * 当前这一版：已确认、而且没有被后来的版本替掉。
 *
 * 前端也算一遍（后端有 CurrentPblReframe），因为界面上要同时拿到"现在这版"和
 * "上一版"来对照，而对照正是重新框定问题时最该看见的东西。
 */
export function currentReframe(all: Reframe[]): Reframe | null {
  // 🚨 只有**已确认**的那一版才算替掉了上一版。写到一半的草稿一开局就指向了
  // 当前这版（界面一打开就建了它），如果让它也算数，"现在的问题"会在她刚开
  // 始改写的那一刻凭空消失——而那正是最需要看见上一版的时候。
  const superseded = new Set(
    all.filter((r) => r.confirmedAt).map((r) => r.supersedes).filter(Boolean) as string[],
  );
  const live = all.filter((r) => r.confirmedAt && !superseded.has(r.id));
  return live.length ? (live[live.length - 1] ?? null) : null;
}

/** 还没定下来的那一版，如果有。同一时刻最多只该有一版在写。 */
export function draftReframe(all: Reframe[]): Reframe | null {
  const drafts = all.filter((r) => !r.confirmedAt);
  return drafts.length ? (drafts[drafts.length - 1] ?? null) : null;
}

/**
 * 从她摆好的格子里，起一句 How might we 的头。
 *
 * 🚨 这不是 AI 代笔（铁律①）。填进去的每一个字都是她自己写在便签上的话，
 * 模板只负责把它们摆成一个问句——AGENTS.md 说得很清楚：「从学生已陈述的
 * 研究问题派生大纲」是确定性的系统步骤，不需要为它加一道确认门槛。
 * 她拿到这句之后可以整句改写，改过的那一版才是存进 hmw 的东西。
 *
 * 空着的格子留「……」而不是留空：一句缺了词的话看得出缺在哪儿，一句被删干净
 * 的话看起来只是没写。
 */
export function seedHmw(who: string, needs: string): string {
  const w = who.trim() || "……";
  const n = needs.trim() || "……";
  return `我们可以怎样帮助${w}，让他能够${n}？`;
}

/** 拼成一句人话，给她看的。 */
export function reframeSentence(r: {
  who: string;
  needs: string;
  why: string;
}): string {
  if (!r.who && !r.needs && !r.why) return "";
  // 🚨 问的是「为什么这对他重要？」，所以她的答案几乎总是「因为…」开头，
  // 模板再补一个「因为」就成了「因为 因为课间只有十分钟」。空格也去掉：
  // 中文句子里不留西文空格。
  const why = (r.why || "").replace(/^[，,、\s]*因为[，,：:\s]*/, "");
  return `${r.who || "……"}需要${r.needs || "……"}，因为${why || "……"}。`;
}
