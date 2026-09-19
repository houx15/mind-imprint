import { useCallback, useEffect, useRef, useState } from "react";

import { isMuted, setMuted } from "./assets";

/**
 * useVoice —— 这个房间的声音。
 *
 * 三条规矩，每一条都是为了让声音**不可能挡住她**：
 *
 *  1. **默认关。** 教室里默认外放是灾难。
 *  2. **任何一屏都不等它。** `play()` 是 fire-and-forget：404、编解码不支持、
 *     浏览器拦截自动播放，全部吞掉。一句没播出来的旁白，和一屏卡住，差别很大。
 *  3. **换一屏就停上一句。** 否则她快速翻过三屏，会同时听见三个人说话。
 */
export function useVoice(): {
  muted: boolean;
  toggle: () => void;
  /** 播一句。静音时什么都不做。 */
  play: (url: string) => void;
  /** 停。换屏时调。 */
  stop: () => void;
} {
  const [muted, setMutedState] = useState(true);
  const audioRef = useRef<HTMLAudioElement | null>(null);

  // 读 localStorage 放在 effect 里而不是 useState 的初值里：服务端渲染和
  // 预览环境里 localStorage 可能不存在，而初值会在渲染期间就被求值。
  useEffect(() => {
    setMutedState(isMuted());
  }, []);

  const stop = useCallback(() => {
    const a = audioRef.current;
    if (!a) return;
    a.pause();
    a.currentTime = 0;
  }, []);

  const play = useCallback(
    (url: string) => {
      if (muted || !url) return;
      stop();
      try {
        const a = new Audio(url);
        audioRef.current = a;
        // 浏览器在没有用户交互时会拒绝自动播放，返回一个 rejected promise。
        // 那不是错误，是它该有的行为 —— 吞掉。
        void a.play().catch(() => undefined);
      } catch {
        // 连 Audio 都构造不出来的环境（某些内嵌 webview）。不出声，照常往下走。
      }
    },
    [muted, stop],
  );

  const toggle = useCallback(() => {
    setMutedState((m) => {
      const next = !m;
      setMuted(next);
      if (next) {
        const a = audioRef.current;
        if (a) {
          a.pause();
          a.currentTime = 0;
        }
      }
      return next;
    });
  }, []);

  // 离开房间时闭嘴。
  useEffect(() => () => {
    const a = audioRef.current;
    if (a) a.pause();
  }, []);

  return { muted, toggle, play, stop };
}
