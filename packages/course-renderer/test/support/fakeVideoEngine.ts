import type { VideoEngine } from "../../src/media/videoEngine";

/**
 * Deterministic {@link VideoEngine} for tests: records call order and lets a
 * test drive playback time and the `ended` event synchronously. jsdom has no
 * media playback, so cue timing is driven purely through {@link advanceTo}.
 */
export class FakeVideoEngine implements VideoEngine {
  readonly calls: Array<"play" | "pause" | "reset" | `seek:${number}`> = [];
  private time = 0;
  private readonly timeListeners = new Set<(t: number) => void>();
  private readonly endedListeners = new Set<() => void>();

  play(): void {
    this.calls.push("play");
  }

  pause(): void {
    this.calls.push("pause");
  }

  reset(): void {
    this.calls.push("reset");
    this.time = 0;
  }

  currentTime(): number {
    return this.time;
  }

  seek(seconds: number): void {
    this.calls.push(`seek:${seconds}`);
    this.time = seconds;
    for (const l of this.timeListeners) l(this.time);
  }

  onTimeUpdate(cb: (t: number) => void): () => void {
    this.timeListeners.add(cb);
    return () => this.timeListeners.delete(cb);
  }

  onEnded(cb: () => void): () => void {
    this.endedListeners.add(cb);
    return () => this.endedListeners.delete(cb);
  }

  /** Advance reported time to `seconds` and notify time listeners. */
  advanceTo(seconds: number): void {
    this.time = seconds;
    for (const l of this.timeListeners) l(this.time);
  }

  /** Fire the `ended` event to every registered listener. */
  fireEnded(): void {
    for (const l of this.endedListeners) l();
  }

  factory = (): VideoEngine => this;
}
