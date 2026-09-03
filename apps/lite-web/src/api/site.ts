import { apiFetch } from "./client";
import type { SiteContent, SiteDraft, SiteLayout } from "../site/types";

/**
 * api/site.ts — 她的主页那一组端点。形状照着
 * apps/api/internal/api/pbl_site.go 的 pblSiteDTO 抄，不是猜的。
 */

export interface SiteState {
  layout: SiteLayout;
  layoutWhy: string;
  draft: SiteDraft;
  /** 服务端合成好的整页：她写的字 + 她真做过的事。 */
  content: SiteContent;
  /** 还缺哪些她自己的字。非空时发布会被服务端拒掉。 */
  missing: string[];
  published: boolean;
  /** 已发布时的公开链接；没发布时是空串。 */
  url: string;
  /** 建起它的那个主页项目。 */
  projectId: string;
}

export function getSite(): Promise<SiteState> {
  return apiFetch<SiteState>("/api/v1/pbl/site");
}

export function putSiteContent(draft: SiteDraft): Promise<SiteState> {
  return apiFetch<SiteState>("/api/v1/pbl/site/content", {
    method: "PUT",
    body: JSON.stringify(draft),
  });
}

/** 挑版式。`why` 为空时服务端返回 400——没有理由，什么都不落定。 */
export function putSiteLayout(layout: SiteLayout, why: string): Promise<SiteState> {
  return apiFetch<SiteState>("/api/v1/pbl/site/layout", {
    method: "PUT",
    body: JSON.stringify({ layout, why }),
  });
}

export function publishSite(): Promise<{ url: string; published: boolean }> {
  return apiFetch("/api/v1/pbl/site/publish", { method: "POST" });
}

export function revokeSite(): Promise<void> {
  return apiFetch("/api/v1/pbl/site/publish", { method: "DELETE" });
}

/** 开始（或回到）主页项目。幂等：已经有一个就返回原来那个。 */
export function startSiteProject(): Promise<{ id: string }> {
  return apiFetch("/api/v1/pbl/site/project", { method: "POST" });
}

/** 访客那一面。没有 session 也能读——这是整个轻量版第二个这样的端点。 */
export function getPublicSite(token: string): Promise<{ layout: SiteLayout; content: SiteContent }> {
  return apiFetch(`/api/v1/public/sites/${encodeURIComponent(token)}`);
}
