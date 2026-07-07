/**
 * AudioPlayer wraps a single shared HTMLAudioElement so at most one
 * narration clip plays at a time across the whole app.
 *
 * The underlying `Audio()` element is constructed lazily (inside `play`)
 * so this module is safe to import in SSR / test environments where
 * `Audio` may not exist as a global.
 */
export class AudioPlayer {
  private el: HTMLAudioElement | null = null;
  private currentUrl: string | null = null;

  /**
   * `onEnded` is an optional extra param (natural playback completion callback) —
   * additive to the base `play(url): Promise<void>` interface, so existing callers
   * that only pass `url` are unaffected.
   */
  async play(url: string, onEnded?: () => void): Promise<void> {
    this.stop();
    if (typeof Audio === "undefined") return;
    const el = new Audio();
    this.el = el;
    this.currentUrl = url;
    el.src = url;
    if (onEnded) el.addEventListener("ended", onEnded, { once: true });
    await el.play();
  }

  stop(): void {
    if (!this.el) return;
    this.el.pause();
    this.el.currentTime = 0;
    this.el = null;
    this.revokeCurrentUrl();
  }

  private revokeCurrentUrl(): void {
    if (this.currentUrl && typeof URL !== "undefined" && typeof URL.revokeObjectURL === "function") {
      URL.revokeObjectURL(this.currentUrl);
    }
    this.currentUrl = null;
  }
}

export const player = new AudioPlayer();
