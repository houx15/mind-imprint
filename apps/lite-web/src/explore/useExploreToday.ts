import { useCallback, useEffect, useState } from "react";
import { fetchToday, type ExplorePlanet, type ExploreToday } from "../api/explore";

/**
 * 今天的星图。
 *
 * 四个状态，和兴趣树一样分开，理由也一样：**没有 mock 兜底，也绝不拿昨天的
 * 冒充今天的。**
 *
 *   loading  正在生成（第一个打开的人会触发一次抓取 + 一次模型调用，要几秒）
 *   error    请求失败，说出后台原话
 *   empty    今天没有星图 —— `note` 里有服务端给的原因
 *   ready    有五颗星
 *
 * `empty` 和 `error` 是两回事：前者是「今天生成失败了，这是原因」，后者是
 * 「这次请求没成功」。学生看到的句子不一样。
 */
export type ExploreStatus = "loading" | "error" | "empty" | "ready";

export interface LiveExplore {
  status: ExploreStatus;
  day: string;
  planets: ExplorePlanet[];
  /** 服务端说的「今天为什么没有星图」。 */
  note: string;
  /** 请求本身失败时的原话。 */
  error: string;
  reload: () => void;
  /** 收藏成功后原地更新那一颗，不必重新拉整屏。 */
  applySaved: (p: ExplorePlanet) => void;
}

export function useExploreToday(): LiveExplore {
  const [data, setData] = useState<ExploreToday | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    setError("");
    fetchToday()
      .then((t) => {
        if (alive) setData(t);
      })
      .catch((e: unknown) => {
        if (alive) setError(e instanceof Error ? e.message : String(e));
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [nonce]);

  const reload = useCallback(() => setNonce((n) => n + 1), []);

  const applySaved = useCallback((saved: ExplorePlanet) => {
    setData((cur) =>
      cur ? { ...cur, planets: cur.planets.map((p) => (p.id === saved.id ? saved : p)) } : cur,
    );
  }, []);

  const status: ExploreStatus = loading
    ? "loading"
    : error
      ? "error"
      : (data?.planets.length ?? 0) === 0
        ? "empty"
        : "ready";

  return {
    status,
    day: data?.day ?? "",
    planets: data?.planets ?? [],
    note: data?.note ?? "",
    error,
    reload,
    applySaved,
  };
}
