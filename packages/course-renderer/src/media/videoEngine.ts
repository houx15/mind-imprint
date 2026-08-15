import { createContext, useContext, useRef } from "react";

/**
 * §17.10 — the minimal media surface {@link VideoRenderer} and the cue timeline
 * need. Injected so tests (and non-DOM hosts) can substitute a deterministic
 * fake: jsdom does not implement `HTMLMediaElement` playback, so all timing is
 * driven through the engine's reported `currentTime`, never the wall clock.
 */
export interface VideoEngine {
  play(): void;
  pause(): void;
  /** Rewind to the start and stop. */
  reset(): void;
  /** The engine's reported playback position, in seconds. */
  currentTime(): number;
  /** Seek to `seconds`. */
  seek(seconds: number): void;
  /** Registers a time-update listener (called with the new currentTime); returns an unsubscribe fn. */
  onTimeUpdate(cb: (seconds: number) => void): () => void;
  /** Registers an `ended` listener; returns an unsubscribe fn. */
  onEnded(cb: () => void): () => void;
}

/**
 * Default engine: binds to the React-rendered `<video>` element via a getter, so
 * poster/captions/track live in the accessible DOM element while playback state
 * flows through this seam. The getter defers element access until after mount.
 */
export class HtmlVideoEngine implements VideoEngine {
  constructor(private readonly getEl: () => HTMLVideoElement | null) {}

  play(): void {
    // Ignore the play() promise: autoplay-policy rejection is not fatal here.
    void this.getEl()?.play?.();
  }

  pause(): void {
    this.getEl()?.pause();
  }

  reset(): void {
    const el = this.getEl();
    if (el) {
      el.pause();
      el.currentTime = 0;
    }
  }

  currentTime(): number {
    return this.getEl()?.currentTime ?? 0;
  }

  seek(seconds: number): void {
    const el = this.getEl();
    if (el) el.currentTime = seconds;
  }

  onTimeUpdate(cb: (seconds: number) => void): () => void {
    const el = this.getEl();
    if (!el) return () => {};
    const handler = () => cb(el.currentTime);
    el.addEventListener("timeupdate", handler);
    return () => el.removeEventListener("timeupdate", handler);
  }

  onEnded(cb: () => void): () => void {
    const el = this.getEl();
    if (!el) return () => {};
    el.addEventListener("ended", cb);
    return () => el.removeEventListener("ended", cb);
  }
}

/** A factory so each mounted video gets an engine bound to its own element. */
export type VideoEngineFactory = (getEl: () => HTMLVideoElement | null) => VideoEngine;

const VideoEngineContext = createContext<VideoEngineFactory | null>(null);

export const VideoEngineProvider = VideoEngineContext.Provider;

/**
 * Returns a stable {@link VideoEngine} for one mounted video, bound to `getEl`.
 * Uses the injected factory when a provider is present (tests inject a fake),
 * otherwise a per-instance {@link HtmlVideoEngine}. Created once per mount.
 */
export function useVideoEngine(getEl: () => HTMLVideoElement | null): VideoEngine {
  const factory = useContext(VideoEngineContext);
  const ref = useRef<VideoEngine | null>(null);
  if (ref.current === null) ref.current = factory ? factory(getEl) : new HtmlVideoEngine(getEl);
  return ref.current;
}
