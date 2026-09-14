import { useCallback, useEffect, useMemo, useState } from "react";
import { fetchInterestTree, type InterestField, type InterestTree } from "../api/interest";
import { toTreeKeywords } from "./liveTree";
import type { Keyword } from "./types";

/**
 * useInterestTree — 那棵树的真数据。
 *
 * # 没有 mock 兜底，这是有意的
 *
 * 最省事的写法是「请求失败就回退到 `data/tree.ts` 的示例关键词」，这样任何时候
 * 打开都有一棵好看的树。**绝不这么做。** 那意味着一个学生会看到十六个不属于她
 * 的词，挂在一张标着「这就是你的模型」的图上，而且她没有任何办法看出来。见
 * memory: ai-errors-must-surface-never-fake —— 一个像模像样的假答案，比一条明白
 * 的错误糟得多。
 *
 * 所以四个状态是分开的，界面必须把它们说清楚：
 *
 *   loading  正在长（服务端可能正在补采，见 interest_harvest.go，会慢几秒）
 *   error    出错了，说出来
 *   empty    她还没有词 —— 这是邀请她去做兴趣测试的地方（P3）
 *   ready    有树
 */
export type TreeStatus = "loading" | "error" | "empty" | "ready";

export interface LiveTree {
  status: TreeStatus;
  keywords: Keyword[];
  fields: InterestField[];
  /** 后台的原话，直接显示给她和我们看（AGENTS.md 界面文案 §8）。 */
  error: string;
  /** 她做完兴趣测试之后要重新拉一次。 */
  reload: () => void;
}

/**
 * `fetcher` defaults to the student's own tree (`fetchInterestTree`) so
 * every existing caller is unaffected. The lite teacher end passes its own
 * fetcher (`getStudentTree(classId, userId)`, wrapped through
 * `fetchInterestTreeFrom`) to read a STUDENT's tree instead.
 *
 * 🚨 Pass an inline arrow and it must be memoised (`useCallback`): it sits
 * in this hook's effect deps below, and a fresh function identity every
 * render would refetch on every render.
 */
export function useInterestTree(fetcher: () => Promise<InterestTree> = fetchInterestTree): LiveTree {
  const [tree, setTree] = useState<InterestTree | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    setError("");
    fetcher()
      .then((t) => {
        if (alive) setTree(t);
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
  }, [nonce, fetcher]);

  const reload = useCallback(() => setNonce((n) => n + 1), []);

  // 位置只在数据变化时重算。每次渲染都重算，会让一次悬停就把整棵树重排一遍。
  const keywords = useMemo(() => (tree ? toTreeKeywords(tree) : []), [tree]);

  const status: TreeStatus = loading
    ? "loading"
    : error
      ? "error"
      : keywords.length === 0
        ? "empty"
        : "ready";

  return {
    status,
    keywords,
    fields: tree?.fields ?? [],
    error,
    reload,
  };
}

/**
 * 成果数 —— 这些词是从几件**做完的**事情上长出来的。
 *
 * 按 (类型, id) 去重：一篇阅读长出三个词，它仍然是一件事。这个数字的定义就是
 * 「几件做完的事」，而不是「几次活动」——后者是一个参与度指标，而参与度指标会
 * 让一个反复打开同一篇文章的学生看起来很努力。
 */
export function outputCount(keywords: Keyword[]): number {
  const seen = new Set<string>();
  for (const k of keywords) {
    for (const s of k.sources) {
      if (s.id) seen.add(`${s.kind}:${s.id}`);
    }
  }
  return seen.size;
}
