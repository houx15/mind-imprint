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
 * ⏳ 这个请求**可能会慢**：服务端在返回之前会把她已完成、还没采过的东西补采
 * 一遍（最多三个，并行），所以第一次打开一棵积压的树可能要几秒。界面必须给
 * 一个「正在长」的状态，而不是一片空白——见 `interest_harvest.go` 顶部。
 */
export async function fetchInterestTree(): Promise<InterestTree> {
  return normalize(await apiFetch<RawTree>("/api/v1/interest/tree"));
}
