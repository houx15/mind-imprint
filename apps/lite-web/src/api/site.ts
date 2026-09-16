import { API_BASE, apiFetch } from "./client";
import type { SiteContent, SiteDraft, SiteLayout, SitePalette } from "../site/types";

export function putSiteSectionImage(key: string, objectKey: string): Promise<SiteState> {
  return apiFetch(`/api/v1/pbl/site/sections/${encodeURIComponent(key)}/image`, {
    method: "PUT", body: JSON.stringify({ objectKey }),
  });
}

/**
 * api/site.ts — 她的主页那一组端点。形状照着
 * apps/api/internal/api/pbl_site.go 的 pblSiteDTO 抄，不是猜的。
 */

export interface SiteState {
  layout: SiteLayout;
  /** 第三关定下的配色。三个颜色都空 = 还没定。 */
  palette: SitePalette;
  /** 第三关生成的头图。空 = 她没要头图。 */
  heroUrl: string;
  draft: SiteDraft;
  /** 服务端合成好的整页：她写的字 + 她真做过的事。 */
  content: SiteContent;
  /** 还缺哪些她自己的字。非空时发布会被服务端拒掉。 */
  missing: string[];
  publishMissing?: string[];
  published: boolean;
  publishedVersionId?: string;
  /** 已发布时的公开链接；没发布时是空串。 */
  url: string;
  /** 建起它的那个主页项目。 */
  projectId: string;
  /** 她已经发布出去的作品，和访客在 `/p/:token` 上看到的是同一份。 */
  works?: PublishedWork[];
}

export function getSite(): Promise<SiteState> {
  return apiFetch<SiteState>("/api/v1/pbl/site");
}

export function putSiteContent(draft: SiteDraft, expectedDraft?: SiteDraft): Promise<SiteState> {
  return apiFetch<SiteState>("/api/v1/pbl/site/content", {
    method: "PUT",
    body: JSON.stringify({ ...draft, expectedDraft }),
  });
}

/**
 * 第三关：风格 + 配色。
 *
 * 🚨 取代了 putSiteLayout。那一条要她**写一句理由**才落定——那是 SiteStudio 那个
 * 表单里的一格，而整个表单已经退役。理由没有消失，它换了个地方：配色是从她第一
 * 关留下的关键词派生的，每一组都写着「它为什么配那几个词」。
 */
export function putSiteLook(layout: SiteLayout, palette: SitePalette): Promise<SiteState> {
  return apiFetch<SiteState>("/api/v1/pbl/site/look", {
    method: "PUT",
    body: JSON.stringify({ layout, palette }),
  });
}

/** 从她的关键词派生三组配色。 */
export function generatePalettes(): Promise<SitePalette[]> {
  return apiFetch<SitePalette[]>("/api/v1/pbl/site/palettes", { method: "POST" });
}

/** 画一张头图。实测一次约 70 秒。 */
export function drawSiteHero(): Promise<SiteState> {
  return apiFetch<SiteState>("/api/v1/pbl/site/hero", { method: "POST" });
}

/** 不要头图了。这是一个合法的选择，不是一件没做完的事。 */
export function clearSiteHero(): Promise<SiteState> {
  return apiFetch<SiteState>("/api/v1/pbl/site/hero", { method: "DELETE" });
}

export function publishSite(versionId?: string): Promise<{ url: string; published: boolean }> {
  return apiFetch("/api/v1/pbl/site/publish", { method: "POST", body: JSON.stringify({versionId}) });
}

export function revokeSite(): Promise<void> {
  return apiFetch("/api/v1/pbl/site/publish", { method: "DELETE" });
}

/** 开始（或回到）主页项目。幂等：已经有一个就返回原来那个。 */
export function startSiteProject(): Promise<{ id: string }> {
  return apiFetch("/api/v1/pbl/site/project", { method: "POST" });
}

/** 访客那一面。没有 session 也能读——这是整个轻量版第二个这样的端点。 */
/** 她发布过的一件作品。`publicPath` 是相对路径（`/s/<token>`）—— 绝对地址由
 *  浏览器用自己的 origin 拼，理由见 SharePanel。 */
export interface PublishedWork {
  title: string;
  kind: string;
  publicPath: string;
}

export function getPublicSite(
  token: string,
): Promise<
  | {generated: true; renderKey: string; comparison?: {feedback:string;observation:string}|null; works?: PublishedWork[]}
  | {generated?: false; layout: SiteLayout; palette: SitePalette; heroUrl: string; content: SiteContent; works?: PublishedWork[] }
> {
  return apiFetch(`/api/v1/public/sites/${encodeURIComponent(token)}`);
}

export function applySiteStructure(projectId: string): Promise<SiteState> {
 return apiFetch<SiteState>(`/api/v1/pbl/projects/${projectId}/site-structure`, { method: "POST" });
}

export const publicCodeSiteURL=(token:string)=>`${API_BASE}/api/v1/public/sites/${encodeURIComponent(token)}/render`;
