import { apiFetch } from "./client";

/**
 * api/explore — 今日新闻星图。
 *
 * 形状逐字段抄自 `apps/api/internal/api/explore.go`，对着那份 Go 源码核过。
 * 每个读取都带 `?? []` / `?? ""`：少一个字段绝不让一次渲染崩掉。
 *
 * ⏳ `fetchToday` **可能很慢**：今天这一屏还没生成时，第一个打开的学生会触发
 * 一次抓取（十二个源，并行，实测 2-3 秒）加一次模型调用。界面必须给一个
 * 「正在生成」的状态。见 explore.go 顶部关于「为什么生成是惰性的」。
 */

export interface ExploreDiscipline {
  id: string;
  zh: string;
  en: string;
  asks: string;
}

export interface ExplorePlanet {
  id: string;
  /** 1..5，1 是今天最重要的那条。决定星球大小与位置。 */
  rank: number;
  titleZh: string;
  titleEn: string;
  summary: string;
  /** 她能自己追问的那个问题。永远非空 —— 这是这一屏存在的理由。 */
  hook: string;
  url: string;
  source: string;
  /** 七根主枝之一。星球的颜色 = 它会长在树的哪根枝上。 */
  field: string;
  /** 她收藏这颗星时，种进树的那个词。 */
  keyword: string;
  discipline: ExploreDiscipline | null;
  saved: boolean;
  publishedAt: string;
}

export interface ExploreToday {
  day: string;
  planets: ExplorePlanet[];
  /**
   * 生成失败时的后台原话。非空表示今天这一屏是空的，而且我们知道为什么。
   * 照原样显示（界面文案 §8）—— **绝不用昨天的顶上**。
   */
  note: string;
  /**
   * 还要等几秒才能再试一次生成。0 = 现在就能试，-1 = 今天不再试了。
   *
   * 🚨 空状态里那个「重试」按钮靠这个数决定要不要出现。上一版服务端失败一次
   * 就锁死一整天，那个按钮按下去什么都不会发生 —— 一个按不动的按钮教她的是
   * 「这里的按钮不作数」。
   */
  retryAfter: number;
}

function normalizePlanet(p: Partial<ExplorePlanet>): ExplorePlanet {
  return {
    id: p.id ?? "",
    rank: p.rank ?? 1,
    titleZh: p.titleZh ?? "",
    titleEn: p.titleEn ?? "",
    summary: p.summary ?? "",
    hook: p.hook ?? "",
    url: p.url ?? "",
    source: p.source ?? "",
    field: p.field ?? "science",
    keyword: p.keyword ?? "",
    discipline: p.discipline ?? null,
    saved: p.saved ?? false,
    publishedAt: p.publishedAt ?? "",
  };
}

export async function fetchToday(): Promise<ExploreToday> {
  const raw = await apiFetch<Partial<ExploreToday>>("/api/v1/explore/today");
  return {
    day: raw.day ?? "",
    planets: (raw.planets ?? []).map(normalizePlanet),
    note: raw.note ?? "",
    retryAfter: raw.retryAfter ?? -1,
  };
}

/**
 * 收藏一颗星 = **把它加到我的树上**。
 *
 * 服务端会用这颗星的钩子作为 evidence 种一个词。这不是一个书签开关：没有
 * 「取消收藏」，因为那个词来自一件真的发生过的事。
 */
export async function savePlanet(id: string): Promise<ExplorePlanet> {
  return normalizePlanet(
    await apiFetch<Partial<ExplorePlanet>>(`/api/v1/explore/planets/${encodeURIComponent(id)}/save`, {
      method: "POST",
    }),
  );
}
