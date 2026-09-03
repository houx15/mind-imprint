import { apiFetch } from "./client";

/**
 * api/siteRefs.ts —— 主页项目第二关：她自己找到的那几个个人网站。
 *
 * 形状照着 apps/api/internal/api/pbl_sites.go 的 pblSiteRefDTO 抄，不是猜的。
 */

export interface SiteRef {
  id: string;
  url: string;
  title: string;
  /** 这一站在做什么。印记读完那一页说的。 */
  what: string;
  /** 它由哪几块组成。她搭自己结构时照着这个看。 */
  structure: string;
  /** 最值得学的一处。 */
  best: string;
  /** 她自己在这一站上补的一句。 */
  sheSaid: string;
}

/** 第二关要几个站才算够。和服务端 pbl.SiteRefsWanted 是同一个数。 */
export const SITE_REFS_WANTED = 3;

const base = (id: string) => `/api/v1/pbl/projects/${id}/sites`;

export function listSiteRefs(projectId: string): Promise<SiteRef[]> {
  return apiFetch<SiteRef[]>(base(projectId));
}

/** 粘一个网址进来。服务端去读那一页，回一张卡。 */
export function addSiteRef(projectId: string, url: string): Promise<SiteRef> {
  return apiFetch<SiteRef>(base(projectId), {
    method: "POST",
    body: JSON.stringify({ url }),
  });
}

export function setSiteRefSaid(projectId: string, id: string, sheSaid: string): Promise<SiteRef> {
  return apiFetch<SiteRef>(`${base(projectId)}/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ sheSaid }),
  });
}

export function deleteSiteRef(projectId: string, id: string): Promise<void> {
  return apiFetch<void>(`${base(projectId)}/${id}`, { method: "DELETE" });
}
