import { createContext, useContext } from "react";

/**
 * §11 / §17.13 — the minimal audio surface the narration layer needs. Injected
 * so tests (and non-DOM hosts) can substitute a deterministic fake. Only ONE
 * track ever plays: the {@link NarrationController} calls `stop` before starting
 * a new track.
 */
export interface AudioEngine {
  play(url: string): void;
  pause(): void;
  stop(): void;
  /** Registers an `ended` listener; returns an unsubscribe fn. */
  onEnded(cb: () => void): () => void;
}

/**
 * Default engine: wraps a single lazily-created {@link HTMLAudioElement}. The
 * element is not constructed until the first `play`, so merely instantiating the
 * engine (e.g. as a context default) is safe under jsdom.
 */
export class HtmlAudioEngine implements AudioEngine {
  private audio: HTMLAudioElement | null = null;
  private readonly endedListeners = new Set<() => void>();

  private ensure(): HTMLAudioElement {
    if (!this.audio) {
      const audio = new Audio();
      audio.addEventListener("ended", () => {
        for (const listener of this.endedListeners) listener();
      });
      this.audio = audio;
    }
    return this.audio;
  }

  play(url: string): void {
    const audio = this.ensure();
    if (audio.src !== url) audio.src = url;
    audio.currentTime = 0;
    // Ignore the play() promise: autoplay policy rejection is not fatal here.
    void audio.play?.();
  }

  pause(): void {
    this.audio?.pause();
  }

  stop(): void {
    if (this.audio) {
      this.audio.pause();
      this.audio.currentTime = 0;
    }
  }

  onEnded(cb: () => void): () => void {
    this.endedListeners.add(cb);
    return () => {
      this.endedListeners.delete(cb);
    };
  }
}

/** Module-level default so every consumer without a provider shares one engine. */
let defaultEngine: AudioEngine | null = null;
function getDefaultEngine(): AudioEngine {
  if (!defaultEngine) defaultEngine = new HtmlAudioEngine();
  return defaultEngine;
}

const AudioEngineContext = createContext<AudioEngine | null>(null);

export const AudioEngineProvider = AudioEngineContext.Provider;

/** Reads the injected engine, falling back to the shared {@link HtmlAudioEngine}. */
export function useAudioEngine(): AudioEngine {
  return useContext(AudioEngineContext) ?? getDefaultEngine();
}
