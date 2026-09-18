import { useCallback, useEffect, useState } from "react";

/**
 * 右边那一栏的宽度，她自己拖。
 *
 * 🚨 产品负责人 2026-09-03：「we have the right sidebar adjustable like in
 * cowork right? sometimes we need to see a larger thing or file, chat area
 * would be narrow」。
 *
 * 固定 360px 是个折中，而折中在两头都不成立：审核一份文档时它太窄，随手看一眼
 * 计划时它又白占着地方。一个「铺开」按钮只是把折中换成两个折中——她要的是自己
 * 定，而且不同的工具、不同的时刻要的宽度本来就不一样。
 *
 * 宽度存在 localStorage：她调好一次就该一直是那样，而不是每开一个项目重调一遍。
 */

const KEY = "pbl:paneWidth";
export const PANE_MIN = 300;
/** 对话至少留这么宽。再窄下去左边就不是对话了，是一条缝。 */
export const CHAT_MIN = 380;
export const PANE_DEFAULT = 360;

/** 夹在能用的范围里。窗口变窄时也要重夹一次，否则栏会顶出屏幕。 */
export function clampPaneWidth(width: number, viewport: number): number {
  const max = Math.max(PANE_MIN, viewport - CHAT_MIN);
  return Math.min(Math.max(Math.round(width), PANE_MIN), max);
}

function readStored(key: string, fallback: number): number {
  try {
    const raw = window.localStorage.getItem(key);
    const n = raw ? Number(raw) : NaN;
    return Number.isFinite(n) ? n : fallback;
  } catch {
    // 无痕窗口、禁了站点数据——记不住就用默认值，不该因此崩掉整个房间。
    return fallback;
  }
}

/** `key` / `fallback`：写作房间也用这一个（2026-09-18「ai sidebar right side,
 *  can adjust width」），宽度各存各的，默认值也不同。 */
export function usePaneWidth(key: string = KEY, fallback: number = PANE_DEFAULT) {
  const [width, setWidthRaw] = useState<number>(fallback);
  // 窄屏上工具是整屏浮层，宽度没有意义，拖把手也不出现。
  const [desktop, setDesktop] = useState(false);

  useEffect(() => {
    // jsdom（房间的逻辑测试）没有 matchMedia：没有就当窄屏，不拖。
    if (typeof window.matchMedia !== "function") return;
    const mq = window.matchMedia("(min-width: 1024px)");
    const sync = () => setDesktop(mq.matches);
    sync();
    mq.addEventListener("change", sync);
    return () => mq.removeEventListener("change", sync);
  }, []);

  useEffect(() => {
    setWidthRaw(clampPaneWidth(readStored(key, fallback), window.innerWidth));
    const onResize = () => setWidthRaw((w) => clampPaneWidth(w, window.innerWidth));
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, [key, fallback]);

  const setWidth = useCallback((next: number) => {
    const w = clampPaneWidth(next, window.innerWidth);
    setWidthRaw(w);
    try {
      window.localStorage.setItem(key, String(w));
    } catch {
      // 存不下就只是这一次会话有效，不影响拖动本身。
    }
  }, [key]);

  return { width, setWidth, desktop };
}
