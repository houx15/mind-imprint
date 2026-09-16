import { apiFetch } from "./client";

/**
 * api/interest.ts — 她的兴趣树，从客户端这一侧看。
 *
 * 形状逐字段抄自 `apps/api/internal/api/interest.go` 的 `interestKeywordDTO`
 * / `interestDisciplineDTO` / `interestFieldDTO`，对着那份 Go 源码核过，不是
 * 照 spec 猜的。和 `api/reports.ts` 一样，每个读取都带 `?? []` / `?? ""`，
 * 少一个字段绝不让一次渲染崩掉。
 *
 * 两件事值得在类型这一层说清楚：
 *
 * - **`fields` 永远是七条，包括一个词都没有的那几根。** 空枝不是缺数据，它是
 *   这张图上最有用的一条信息：她还没走过的方向。所以这里不要「过滤掉空的」。
 * - **`sources[].evidence` 永远非空。** 服务端在解析、落库两处都挡过没有原话
 *   的词。界面可以放心地把它当作「她自己说过的话」直接显示——这句话是整棵树
 *   可信度的来源。
 */

/** 树的七根主枝。与 `packages/contracts/src/discipline.ts` 的 `FIELD_IDS` 一致。 */
export type InterestFieldId =
  | "formal"
  | "science"
  | "making"
  | "society"
  | "humanities"
  | "arts"
  | "self";

export type KeywordSourceKind = "reading" | "writing" | "project" | "news" | "quiz";

/** 这条「词 → 学科」的边是怎么来的。界面要说得出凭什么这么放。 */
export type RouteHow = "alias" | "cooccur" | "llm" | "student";

export interface InterestSource {
  kind: KeywordSourceKind;
  /** 对应的 atom id。news / quiz 没有 atom，所以可能是空串。 */
  refId: string;
  label: string;
  /** 她自己的那句话。永远非空。 */
  evidence: string;
  happenedAt: string;
}

export interface SyllabusRef {
  board: "IB" | "ALevel" | "AP" | "IGCSE" | "University";
  code: string;
  label: string;
  level?: string;
}

export interface InterestDiscipline {
  id: string;
  zh: string;
  en: string;
  /** 它研究什么——一句问题。 */
  asks: string;
  method: string;
  exemplar: string;
  syllabus: SyllabusRef[];
  confidence: number;
  how: RouteHow;
  rationale: string;
}

export interface InterestKeyword {
  id: string;
  /** interests.json 的 id。探索地图算推荐时靠它排除她已经有的词。 */
  interestId: string;
  textZh: string;
  textEn: string;
  field: InterestFieldId;
  /** 1..5 的读数，由来源条数推出来，不是等级。 */
  strength: number;
  note: string;
  firstSeenAt: string;
  sources: InterestSource[];
  disciplines: InterestDiscipline[];
}

export interface InterestField {
  id: InterestFieldId;
  label: string;
  keywordCount: number;
}

export interface InterestTree {
  fields: InterestField[];
  keywords: InterestKeyword[];
}

interface RawTree {
  fields?: Partial<InterestField>[];
  keywords?: Partial<InterestKeyword>[];
}

function normalize(raw: RawTree): InterestTree {
  return {
    fields: (raw.fields ?? []).map((f) => ({
      id: (f.id ?? "self") as InterestFieldId,
      label: f.label ?? "",
      keywordCount: f.keywordCount ?? 0,
    })),
    keywords: (raw.keywords ?? []).map((k) => ({
      id: k.id ?? "",
      interestId: k.interestId ?? "",
      textZh: k.textZh ?? "",
      textEn: k.textEn ?? "",
      field: (k.field ?? "self") as InterestFieldId,
      strength: k.strength ?? 1,
      note: k.note ?? "",
      firstSeenAt: k.firstSeenAt ?? "",
      sources: (k.sources ?? []) as InterestSource[],
      disciplines: (k.disciplines ?? []) as InterestDiscipline[],
    })),
  };
}

/**
 * 取她的树。
 *
 * 这是一次纯读（2026-09-04 之后）：采集在后台队列里跑，不再挂在这个请求上。
 */
export async function fetchInterestTree(): Promise<InterestTree> {
  return normalize(await apiFetch<RawTree>("/api/v1/interest/tree"));
}

/** 取任意一棵树的同一套整形。教师端看学生的树时用它，路径不同、形状相同。 */
export async function fetchInterestTreeFrom(path: string): Promise<InterestTree> {
  return normalize(await apiFetch<RawTree>(path));
}

/* ── 继续深挖 ───────────────────────────────────────────────────────────── */

export type DigKind = "think" | "read" | "write" | "make";

export interface DigSeed {
  kind: DigKind;
  /**
   * 会被当作标题 / 立意直接送进创建接口，所以它是一句能独立成立的话。
   *
   * 🚨 `read` 那一颗例外：它是**分级阅读库里那篇文章的真标题**，由服务端填，
   * 不是模型写的（2026-09-16）。模型只在服务端给的名单里挑一个 slug；挑了
   * 名单以外的，那一颗整个不存在。
   */
  text: string;
  /** 一句「为什么是你」，把这颗种子和她这个词的来源连起来。 */
  why: string;
  /**
   * 分级阅读库里的 slug 和难度档，只有 `read` 那一颗有。
   *
   * 空 = 这一颗没有落到库上（也包括那篇文章后来从库里下架了）。界面据此不给
   * 「在阅读室打开」那颗按钮 —— 一颗指向不存在的文章的按钮比没有按钮更糟。
   */
  slug?: string;
  tier?: number;
}

export interface KeywordDig {
  keywordId: string;
  seeds: DigSeed[];
  /**
   * 生成失败时的后台原话。**有 note 就一定没有种子** —— 服务端绝不摆四个通用
   * 动词顶上，界面也不许自己补。
   */
  note: string;
}

/**
 * 取这个关键词的四颗种子。
 *
 * ⏳ 第一次会慢：服务端要发一次调用，用**她在这个词上留下的原话**生成。生成
 * 一次就存着 —— 每次打开都换一批建议的教练，说明它对你没有看法。
 */
export async function fetchKeywordDig(keywordId: string): Promise<KeywordDig> {
  const raw = await apiFetch<Partial<KeywordDig>>(
    `/api/v1/interest/keywords/${encodeURIComponent(keywordId)}/dig`,
  );
  return {
    keywordId: raw.keywordId ?? keywordId,
    seeds: (raw.seeds ?? [])
      .filter((s): s is DigSeed => Boolean(s?.kind && s?.text))
      // 「去读」必须指向库里真的有的那一篇。服务端已经挡过一道（名单外的 slug
      // 整颗丢掉、下架的那篇发出来时不带 slug），这里再挡一道 —— 一颗没有
      // slug 的 read 种子在界面上仍然长得像一篇文章，而它点不开。
      .filter((s) => s.kind !== "read" || Boolean(s.slug && s.tier)),
    note: raw.note ?? "",
  };
}

/* ── 候选词：读完一篇之后等她认的那几个 ─────────────────────────────────── */

export interface InterestProposal {
  interestId: string;
  zh: string;
  en: string;
  field: InterestFieldId;
  /** 印记对这个词之于她的一句话。 */
  note: string;
  /** 她自己写的、让这个词被提出来的那一句。 */
  evidence: string;
  decided: boolean;
  accepted: boolean;
}

export interface InterestProposals {
  proposals: InterestProposal[];
  /**
   * 采集还没跑完。**不是**「还有没决定的候选」—— 界面要分清这两件事，否则一篇
   * 采不出词的阅读会永远转圈。
   */
  pending: boolean;
}

/** GET /api/v1/interest/proposals/{atomId} */
export async function fetchInterestProposals(atomId: string): Promise<InterestProposals> {
  const raw = await apiFetch<Partial<InterestProposals>>(
    `/api/v1/interest/proposals/${encodeURIComponent(atomId)}`,
  );
  return {
    proposals: (raw.proposals ?? []).map((p) => ({
      interestId: p.interestId ?? "",
      zh: p.zh ?? "",
      en: p.en ?? "",
      field: (p.field ?? "self") as InterestFieldId,
      note: p.note ?? "",
      evidence: p.evidence ?? "",
      decided: p.decided ?? false,
      accepted: p.accepted ?? false,
    })),
    pending: raw.pending ?? false,
  };
}

/**
 * POST /api/v1/interest/proposals/{atomId}/{interestId}
 *
 * 认下去会真的往树上种一个词，**没有撤销**。不认也落库 —— 她拒绝了什么和她认下
 * 了什么一样是过程数据（铁律④）。
 */
export async function decideInterestProposal(
  atomId: string,
  interestId: string,
  accept: boolean,
): Promise<void> {
  await apiFetch<void>(
    `/api/v1/interest/proposals/${encodeURIComponent(atomId)}/${encodeURIComponent(interestId)}`,
    { method: "POST", body: JSON.stringify({ accept }) },
  );
}
