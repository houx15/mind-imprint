import { useEffect, useRef, useState } from "react";
import type { ProseResult } from "../api/weekly";
import { useAlive } from "../shared/useAlive";
import { errorText } from "./assignmentLogic";

export type ProseState = { status: "idle" } | { status: "pending" } | { status: "error"; message: string };

const IDLE: ProseState = { status: "idle" };
const PENDING: ProseState = { status: "pending" };

/**
 * useWeekly — one week of a weekly summary (a student's or a class's), with
 * the week navigation and the prose request. Mirrors pro `ClassWeeklyView`'s
 * lifecycle (load, then ask for prose once), with three additions:
 *
 * 1. **The prose request is a spend, so it is latched.** `asked` holds every
 *    `${scope}:${weekStart}` this mount has already POSTed for. A re-render,
 *    a StrictMode remount, a 加载失败 重试 or coming back to a week never
 *    POSTs a second time on its own; only the 重试 button does. A GET never
 *    asks for prose.
 * 2. **Every outcome is remembered per week.** `outcome` keeps pending or the
 *    failure text per key, so going to another week and back shows
 *    总结生成中 or 总结生成失败 again instead of an empty block.
 * 3. **Late answers are dropped by key.** The GET effect has a `cancelled`
 *    flag for the week it loaded. A POST can outlive that effect, so its
 *    answer is applied only while `shown` still names the same week, and
 *    only while the component is mounted (`useAlive`, not a `cancelled`
 *    flag: see useAlive.ts for the StrictMode trap a latch plus a
 *    `cancelled` flag causes).
 *
 * Callers key the component by student or class, so `scope` never changes
 * within one mount.
 */
export function useWeekly<T extends { weekStart: string; proseReady: boolean }>({
  scope,
  load,
  post,
  wantsProse,
}: {
  scope: string;
  /** `weekStart` null = the latest completed week. */
  load: (weekStart: string | null) => Promise<T>;
  post: (weekStart: string) => Promise<ProseResult<T>>;
  /** false skips the prose request (a week with nothing in it). */
  wantsProse: (week: T) => boolean;
}) {
  const [weekStart, setWeekStart] = useState<string | null>(null);
  const [data, setData] = useState<T | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [prose, setProse] = useState<ProseState>(IDLE);
  const [nonce, setNonce] = useState(0);

  const alive = useAlive();
  const asked = useRef(new Set<string>());
  const outcome = useRef(new Map<string, ProseState>());
  const shown = useRef("");

  // The callers' functions change identity on every render; the effect reads
  // the latest ones without re-running.
  const fns = useRef({ load, post, wantsProse });
  fns.current = { load, post, wantsProse };

  function requestProse(ws: string) {
    const key = `${scope}:${ws}`;
    asked.current.add(key);
    outcome.current.set(key, PENDING);
    if (shown.current === key) setProse(PENDING);
    fns.current
      .post(ws)
      .then((r) => {
        const next: ProseState = r.proseError ? { status: "error", message: r.proseError } : IDLE;
        outcome.current.set(key, next);
        if (!alive.current || shown.current !== key) return;
        setData(r.week);
        setProse(next);
      })
      .catch((e: unknown) => {
        const next: ProseState = { status: "error", message: errorText(e) };
        outcome.current.set(key, next);
        if (!alive.current || shown.current !== key) return;
        setProse(next);
      });
  }

  useEffect(() => {
    let cancelled = false;
    shown.current = "";
    setData(null);
    setLoadError(null);
    setProse(IDLE);
    fns.current
      .load(weekStart)
      .then((w) => {
        if (cancelled) return;
        const key = `${scope}:${w.weekStart}`;
        shown.current = key;
        setData(w);
        if (w.proseReady || !fns.current.wantsProse(w)) return;
        if (asked.current.has(key)) {
          setProse(outcome.current.get(key) ?? IDLE);
          return;
        }
        requestProse(w.weekStart);
      })
      .catch((e: unknown) => {
        if (!cancelled) setLoadError(errorText(e));
      });
    return () => {
      cancelled = true;
    };
    // requestProse reads refs only; scope is fixed per mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [scope, weekStart, nonce]);

  return {
    data,
    loadError,
    prose,
    /** The week the navigation steps from: the one shown, else the one asked for. */
    currentWeek: data?.weekStart ?? weekStart,
    goToWeek: (ws: string) => setWeekStart(ws),
    reload: () => setNonce((x) => x + 1),
    retryProse: () => {
      if (data) requestProse(data.weekStart);
    },
  };
}
