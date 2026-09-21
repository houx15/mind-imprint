import { apiFetch } from "./client";

/**
 * 写作题库 —— 705 道 2024–2026 的真题与官方题。
 *
 * 🚨 **筛、搜、翻页都在服务端**。整库发过来是 900KB，而且「筛完之后每一维
 * 还剩哪些值」这件事只有看得见全库的人算得出来。这里只负责把参数拼成
 * query string，再把回来的一页画出来。
 */

export interface WritingPrompt {
  id: string;
  category: string;
  lang: "zh" | "en";
  year: number;
  type: string;
  source: string;
  region?: string;
  taskType: string;
  text: string;
  wordLimit?: string;
  minutes?: number;
  fullScore?: number;
  sourceUrl?: string;
  topics: string[];
  difficulty: number;
  diffLabel: string;
}

export interface WritingPromptDetail extends WritingPrompt {
  requirements?: string;
  notes?: string;
}

export interface PromptFacet {
  value: string;
  label: string;
  count: number;
}

export interface WritingPromptPage {
  items: WritingPrompt[];
  total: number;
  page: number;
  pageSize: number;
  pages: number;
  facets: {
    langs: PromptFacet[];
    categories: PromptFacet[];
    difficulties: PromptFacet[];
    topics: PromptFacet[];
    years: PromptFacet[];
  };
  recommended: { prompt: WritingPrompt; why: string[] }[];
}

export interface PromptQuery {
  lang?: string;
  category?: string;
  difficulty?: number;
  topic?: string;
  year?: number;
  q?: string;
  page?: number;
  pageSize?: number;
}

export function promptQueryString(q: PromptQuery): string {
  const p = new URLSearchParams();
  if (q.lang) p.set("lang", q.lang);
  if (q.category) p.set("category", q.category);
  if (q.difficulty) p.set("difficulty", String(q.difficulty));
  if (q.topic) p.set("topic", q.topic);
  if (q.year) p.set("year", String(q.year));
  if (q.q?.trim()) p.set("q", q.q.trim());
  if (q.page && q.page > 1) p.set("page", String(q.page));
  if (q.pageSize) p.set("pageSize", String(q.pageSize));
  const s = p.toString();
  return s ? `?${s}` : "";
}

export async function listWritingPrompts(
  q: PromptQuery = {},
): Promise<WritingPromptPage> {
  const res = await apiFetch<WritingPromptPage>(
    `/api/v1/writing-prompts${promptQueryString(q)}`,
  );
  // 🚨 服务端的空切片会 marshal 成 null（[[go-nil-slice-becomes-null]]）。
  // 逐层兜底，别让一次 `.map()` 把整屏打白。
  return {
    ...res,
    items: (res.items ?? []).map((it) => ({ ...it, topics: it.topics ?? [] })),
    recommended: (res.recommended ?? []).map((r) => ({
      prompt: { ...r.prompt, topics: r.prompt.topics ?? [] },
      why: r.why ?? [],
    })),
    facets: {
      langs: res.facets?.langs ?? [],
      categories: res.facets?.categories ?? [],
      difficulties: res.facets?.difficulties ?? [],
      topics: res.facets?.topics ?? [],
      years: res.facets?.years ?? [],
    },
  };
}

export function getWritingPrompt(id: string): Promise<WritingPromptDetail> {
  return apiFetch<WritingPromptDetail>(
    `/api/v1/writing-prompts/${encodeURIComponent(id)}`,
  );
}

export async function startWritingFromPrompt(id: string): Promise<string> {
  const res = await apiFetch<{ id: string }>(
    `/api/v1/writing-prompts/${encodeURIComponent(id)}/start`,
    { method: "POST" },
  );
  return res.id;
}
