import { apiFetch } from "./client";

// api/library.ts — 分级阅读库的客户端。形状照抄 Go 的 handler
// （apps/api/internal/api/library.go 的 libraryShelfDTO），不是猜的。
//
// 目录一次全发（二十篇的元数据，不含正文），所以筛选和搜索都在前端完成，敲一
// 个字不打一次请求，「查看全部」那一页也没有加载态。

export interface LibraryTag {
  id: string;
  zh: string;
  field: string;
}

export interface LibraryLevel {
  /** 1..5 */
  tier: number;
  /** 入门 / 基础 / 进阶 / 高阶 / 原文 */
  name: string;
  /** 原始 Lexile 值；0 表示这一档是没有简写过的原文，分级不适用。 */
  lexile: number;
  words: number;
  minutes: number;
}

export interface LibraryArticle {
  slug: string;
  title: string;
  zhTitle: string;
  reason: string;
  /** 主枝 id（science / society / …），决定卡片颜色。 */
  field: string;
  tags: LibraryTag[];
  /** 已经签好的 CDN 链接。空串表示这次签不出来，别渲染这张图。 */
  coverUrl: string;
  levels: LibraryLevel[];
  /** 非空表示她已经从库里开过这一篇 —— 卡片给「继续读」而不是「开始读」。 */
  readingId?: string;
  readTier?: number;
  finished: boolean;
}

export interface LibraryRecommendation {
  slug: string;
  tier: number;
  /** 命中的学科中文名。空表示这条不是按她的兴趣挑的。 */
  why: string[];
}

export interface LibraryShelf {
  articles: LibraryArticle[];
  recommended: LibraryRecommendation[];
  fields: LibraryTag[];
  /** 默认难度（1..5）。 */
  tier: number;
}

/** GET /api/v1/library */
export function getLibraryShelf(): Promise<LibraryShelf> {
  return apiFetch<LibraryShelf>("/api/v1/library");
}

/** POST /api/v1/library/{slug}/levels/{tier} —— 开一篇，返回阅读 id。 */
export async function startLibraryReading(slug: string, tier: number): Promise<string> {
  const raw = await apiFetch<{ id: string }>(
    `/api/v1/library/${encodeURIComponent(slug)}/levels/${tier}`,
    { method: "POST" },
  );
  return raw.id;
}
