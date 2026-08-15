import { useSyncExternalStore } from "react";
import type { NarrationDefinition } from "@mind-imprint/course-contract";
import type { SliceEmitter } from "@mind-imprint/course-runtime";
import type { AudioEngine } from "./audioEngine";

/** The narration currently mounted in the player (post asset-resolution). */
export interface ActiveNarration {
  id: string;
  text: string;
  audioUrl: string;
}

/**
 * §11 / §17.13 — drives one {@link AudioEngine} on behalf of the SlicePlayer.
 * Enforces single-track playback (a new `play` stops the previous track first)
 * and emits `narration.ended` (sourced by the narration id) when the engine
 * reports the track finished. Framework-agnostic: exposes a `subscribe`/
 * `getSnapshot` pair so {@link NarrationPlayer} can render via
 * `useSyncExternalStore`.
 */
export class NarrationController {
  private readonly engine: AudioEngine;
  private readonly listeners = new Set<() => void>();
  private active: ActiveNarration | null = null;
  private unsubEnded: (() => void) | null = null;

  constructor(engine: AudioEngine) {
    this.engine = engine;
    this.subscribe = this.subscribe.bind(this);
    this.getSnapshot = this.getSnapshot.bind(this);
  }

  /** Starts a narration track, stopping any current one first (single-track). */
  play(narration: NarrationDefinition, audioUrl: string, emit: SliceEmitter): void {
    if (this.active) this.detach();
    this.active = { id: narration.id, text: narration.text, audioUrl };
    this.unsubEnded = this.engine.onEnded(() => {
      emit(narration.id, "narration.ended");
    });
    this.engine.play(audioUrl);
    this.notify();
  }

  pause(): void {
    this.engine.pause();
  }

  stop(): void {
    this.engine.stop();
    this.detach();
    this.active = null;
    this.notify();
  }

  /** Replays the active track from the start (§11 replay control). */
  replay(emit: SliceEmitter): void {
    if (this.active) this.play({ id: this.active.id, text: this.active.text, audio: "" }, this.active.audioUrl, emit);
  }

  private detach(): void {
    this.engine.stop();
    this.unsubEnded?.();
    this.unsubEnded = null;
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  getSnapshot(): ActiveNarration | null {
    return this.active;
  }

  private notify(): void {
    for (const listener of this.listeners) listener();
  }
}

export interface NarrationPlayerProps {
  controller: NarrationController;
  /** Slice-scoped emitter, used by the manual replay/pause/stop controls. */
  emit: SliceEmitter;
}

/**
 * Renders the accessible transcript + transport controls for the currently
 * active narration. Renders nothing when no narration is playing. Only one
 * narration is ever active (the controller guarantees single-track).
 */
export function NarrationPlayer({ controller, emit }: NarrationPlayerProps) {
  const active = useSyncExternalStore(controller.subscribe, controller.getSnapshot);
  if (!active) return null;
  return (
    <section className="course-narration" data-narration-id={active.id} aria-label="讲解">
      <div className="course-narration__controls">
        <button type="button" data-narration-control="replay" onClick={() => controller.replay(emit)}>
          重播
        </button>
        <button type="button" data-narration-control="pause" onClick={() => controller.pause()}>
          暂停
        </button>
        <button type="button" data-narration-control="stop" onClick={() => controller.stop()}>
          停止
        </button>
      </div>
      <p className="course-narration__transcript" data-narration-transcript>
        {active.text}
      </p>
    </section>
  );
}
