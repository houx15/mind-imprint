import type { AudioEngine } from "../../src/narration/audioEngine";

/**
 * Deterministic {@link AudioEngine} for tests: records the call order and lets a
 * test fire `ended` synchronously. `calls` captures play/pause/stop in order so
 * single-track ordering (stop-before-play) can be asserted.
 */
export class FakeAudioEngine implements AudioEngine {
  readonly calls: Array<{ op: "play" | "pause" | "stop"; url?: string }> = [];
  private readonly endedListeners = new Set<() => void>();

  play(url: string): void {
    this.calls.push({ op: "play", url });
  }

  pause(): void {
    this.calls.push({ op: "pause" });
  }

  stop(): void {
    this.calls.push({ op: "stop" });
  }

  onEnded(cb: () => void): () => void {
    this.endedListeners.add(cb);
    return () => {
      this.endedListeners.delete(cb);
    };
  }

  /** Fires `ended` to every currently-registered listener. */
  fireEnded(): void {
    for (const listener of this.endedListeners) listener();
  }
}
