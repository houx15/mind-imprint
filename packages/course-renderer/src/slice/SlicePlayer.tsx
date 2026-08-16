import { useEffect, useMemo, useReducer, useRef, useState } from "react";
import type {
  BlockDefinition,
  CourseRuntimeEvent,
  NarrationDefinition,
  SliceDefinition,
  SliceSessionState,
  TargetRef,
  WorkflowEventType,
} from "@mind-imprint/course-contract";
import {
  applyEffect,
  applyEvent,
  initSliceState,
  setCurrentWorkflowStep,
  type CourseRuntimeAdapters,
  type RuntimeEventBus,
  type SliceEmitter,
  type WorkflowEffect,
  type WorkflowInputEvent,
  WorkflowRuntime,
} from "@mind-imprint/course-runtime";
import { LayoutRenderer } from "../layout/LayoutRenderer";
import { getBlockRenderer } from "../blocks/registry";
import { FocusProvider, FocusTarget, focusedItemIdFor } from "../focus/FocusManager";
import { NarrationController, NarrationPlayer } from "../narration/NarrationPlayer";
import { useAudioEngine } from "../narration/audioEngine";
import { MediaHandleRegistry, MediaHandleRegistryProvider } from "../media/mediaRegistry";

/** Injectable timer surface so tests can fire timers deterministically. */
export interface Scheduler {
  setTimeout(handler: () => void, ms: number): unknown;
  clearTimeout(handle: unknown): void;
}

const defaultScheduler: Scheduler = {
  setTimeout: (handler, ms) => setTimeout(handler, ms),
  clearTimeout: (handle) => clearTimeout(handle as ReturnType<typeof setTimeout>),
};

export interface SlicePlayerProps {
  slice: SliceDefinition;
  /** Part this slice belongs to — stamped onto emitted events (event provenance). */
  partId: string;
  /** Session the slice state is persisted under. */
  sessionId: string;
  adapters: CourseRuntimeAdapters;
  bus: RuntimeEventBus;
  onSliceComplete: () => void;
  onNavigateNext: () => void;
  /** Restore a workflow position (revisit). */
  restoreStepId?: string;
  /** Restore persisted block/slice state (revisit). */
  restoreState?: SliceSessionState;
  scheduler?: Scheduler;
  /** Media-handle registry for play/pause/reset effects; created internally when omitted. */
  mediaRegistry?: MediaHandleRegistry;
}

/** CourseRuntimeEvent → the standardized fields a workflow transition matches (§12.4). */
function toWorkflowInput(event: CourseRuntimeEvent): WorkflowInputEvent {
  const payload = event.payload && typeof event.payload === "object" ? (event.payload as Record<string, unknown>) : undefined;
  return {
    type: event.type as WorkflowEventType,
    sourceId: event.sourceId,
    interactionId: typeof payload?.interactionId === "string" ? payload.interactionId : undefined,
    timerId: typeof payload?.timerId === "string" ? payload.timerId : undefined,
  };
}

/**
 * §17.4 / §17.14 / §12 — the wiring hub. Seeds block state (`initSliceState`),
 * starts a `WorkflowRuntime`, applies the runtime's ordered effect list to block
 * state / narration / focus / timers, routes bus Events back into the runtime,
 * folds them onto persisted state, and persists via the session adapter. Renders
 * the slice's layout with each block mounted through the registry.
 *
 * Effects are data; this component is the only place that performs them. Block
 * renderers stay pure and are selected purely by `type`.
 */
export function SlicePlayer({
  slice,
  partId,
  sessionId,
  adapters,
  bus,
  onSliceComplete,
  onNavigateNext,
  restoreStepId,
  restoreState,
  scheduler = defaultScheduler,
  mediaRegistry,
}: SlicePlayerProps) {
  const engine = useAudioEngine();

  // The SlicePlayer owns one media-handle registry (§17.10): media renderers
  // register their imperative handle on mount; the effect interpreter drives them.
  const mediaRegistryRef = useRef<MediaHandleRegistry | null>(mediaRegistry ?? null);
  if (mediaRegistryRef.current === null) mediaRegistryRef.current = new MediaHandleRegistry();
  const registry = mediaRegistryRef.current;

  const blockById = useMemo(() => new Map<string, BlockDefinition>(slice.blocks.map((b) => [b.id, b])), [slice]);
  const narrationById = useMemo(
    () => new Map<string, NarrationDefinition>(slice.narrations.map((n) => [n.id, n])),
    [slice],
  );

  // Authoritative slice state lives in a ref so the (non-React) bus handler can
  // read/write it synchronously; a reducer bump forces the re-render.
  const stateRef = useRef<SliceSessionState>(restoreState ?? initSliceState(slice));
  const [, forceRender] = useReducer((n: number) => n + 1, 0);
  const [focus, setFocus] = useState<TargetRef | null>(slice.workflow.initialState?.focusedTarget ?? null);

  const controllerRef = useRef<NarrationController | null>(null);
  if (controllerRef.current === null) controllerRef.current = new NarrationController(engine);
  const controller = controllerRef.current;

  const timers = useRef(new Map<string, unknown>());
  const emitRef = useRef<SliceEmitter | null>(null);
  // The active WorkflowRuntime, so the effect interpreter can stamp the resume
  // position (`currentWorkflowStepId`) after every start()/send() without
  // re-subscribing. Set once in the mount effect, before it's first read.
  const runtimeRef = useRef<WorkflowRuntime | null>(null);

  // The effect interpreter. Stable via ref so the mount effect and bus handler
  // share one implementation without re-subscribing. `occurredAt`, when passed,
  // is the triggering event's envelope timestamp (NEVER a fresh clock read) —
  // threaded into `completeSlice` so `completedAt`/`elapsedSeconds` freeze correctly.
  const applyEffectsRef = useRef<(effects: WorkflowEffect[], occurredAt?: string) => void>(() => {});
  applyEffectsRef.current = (effects: WorkflowEffect[], occurredAt?: string) => {
    const emit = emitRef.current!;
    let next = stateRef.current;
    let completed = false;
    let navigated = false;

    for (const effect of effects) {
      switch (effect.type) {
        case "show":
        case "hide":
        case "enable":
        case "disable":
          next = applyEffect(next, effect);
          break;
        case "resetBlock":
          // Clears persisted progress AND rewinds the live media element.
          next = applyEffect(next, effect);
          registry.get(effect.targetId)?.reset();
          break;
        case "completeSlice":
          next = applyEffect(next, effect, occurredAt);
          completed = true;
          break;
        case "navigate":
          navigated = true;
          break;
        case "focus":
          setFocus(effect.target);
          break;
        case "clearFocus":
          setFocus(null);
          break;
        case "playNarration": {
          const narration = narrationById.get(effect.narrationId);
          if (narration) controller.play(narration, adapters.assetResolver.resolve(narration.audio), emit);
          break;
        }
        case "pauseNarration":
          // §P2-04 target semantics: only act if this IS the active track —
          // a stale/other narration id must never pause the current one.
          controller.pause(effect.narrationId);
          break;
        case "stopNarration":
          controller.stop(effect.narrationId);
          break;
        case "startTimer": {
          const handle = scheduler.setTimeout(
            () => emit(effect.timerId, "timer.elapsed", { timerId: effect.timerId }),
            effect.durationSeconds * 1000,
          );
          timers.current.set(effect.timerId, handle);
          break;
        }
        case "cancelTimer": {
          const handle = timers.current.get(effect.timerId);
          if (handle !== undefined) {
            scheduler.clearTimeout(handle);
            timers.current.delete(effect.timerId);
          }
          break;
        }
        case "playBlock":
          registry.get(effect.targetId)?.play();
          break;
        case "pauseBlock":
          registry.get(effect.targetId)?.pause();
          break;
      }
    }

    // Stamp the resume position (§16) from the runtime's current step — a no-op
    // (identity-guarded by setCurrentWorkflowStep) when send() didn't transition.
    const runtime = runtimeRef.current;
    if (runtime) next = setCurrentWorkflowStep(next, runtime.currentStepId);

    stateRef.current = next;
    forceRender();
    void adapters.sessionAdapter.saveSliceState(sessionId, slice.id, next);
    if (completed) onSliceComplete();
    if (navigated) onNavigateNext();
  };

  // Mount: activate the slice, bind the emitter, start the runtime, subscribe.
  useEffect(() => {
    bus.setActiveSlice(slice.id);
    const emit = bus.bindSlice(partId, slice.id);
    emitRef.current = emit;

    const runtime = new WorkflowRuntime(slice.workflow, restoreStepId ? { restoreStepId } : undefined);
    runtimeRef.current = runtime;

    const unsubscribe = bus.subscribe((event: CourseRuntimeEvent) => {
      // Persist every accepted event (the bus already dropped anything not for
      // this active slice before delivering it here).
      void adapters.sessionAdapter.appendEvent(sessionId, event);
      // Fold the raw event onto persisted state (occurredAt stamps slice timing
      // from the event envelope — never a fresh clock read), then advance the workflow.
      const folded = applyEvent(stateRef.current, {
        type: String(event.type),
        sourceId: event.sourceId,
        payload: event.payload,
        occurredAt: event.occurredAt,
      });
      stateRef.current = folded;
      applyEffectsRef.current(runtime.send(toWorkflowInput(event)), event.occurredAt);
    });

    applyEffectsRef.current(runtime.start());

    const pending = timers.current;
    return () => {
      unsubscribe();
      for (const handle of pending.values()) scheduler.clearTimeout(handle);
      pending.clear();
      controller.stop();
    };
    // Mount-once wiring; slice identity is stable for a mounted SlicePlayer.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const state = stateRef.current;

  return (
    <MediaHandleRegistryProvider value={registry}>
      <FocusProvider value={focus}>
        <div className="course-slice" data-slice-id={slice.id}>
        <LayoutRenderer
          layout={slice.layout}
          renderSlot={(_slotId, blockIds) =>
            blockIds.map((id) => {
              const block = blockById.get(id);
              if (!block) return null;
              const Renderer = getBlockRenderer(block.type);
              const blockState = state.blockStates[id]!;
              return (
                <FocusTarget key={id} blockId={id} className="course-slot-block">
                  <Renderer
                    block={block}
                    assetResolver={adapters.assetResolver}
                    state={blockState}
                    visible={blockState.visible}
                    enabled={blockState.enabled}
                    focusedItemId={focusedItemIdFor(focus, id)}
                    emit={emitRef.current ?? (() => {})}
                  />
                </FocusTarget>
              );
            })
          }
        />
          <NarrationPlayer controller={controller} emit={emitRef.current ?? (() => {})} />
        </div>
      </FocusProvider>
    </MediaHandleRegistryProvider>
  );
}
