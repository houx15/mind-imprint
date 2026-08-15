import type { BlockDefinition, BlockSessionState, SliceDefinition, SliceSessionState } from "@mind-imprint/course-contract";
import type { WorkflowEffect } from "./workflowRuntime";
import { WorkflowRuntimeError } from "./workflowRuntime";

/** Blocks that carry attempts + answer state (§12.1 assessment seeding). */
const ASSESSMENT_TYPES: ReadonlySet<BlockDefinition["type"]> = new Set(["singleChoice", "fillBlank"]);

const isAssessment = (block: BlockDefinition): boolean => ASSESSMENT_TYPES.has(block.type);

/**
 * §12.1 defaults. Builds the initial SliceSessionState for a slice:
 * - no `initialState` → every block visible + enabled;
 * - `visibleBlockIds` present → listed visible, unlisted hidden;
 * - `enabledBlockIds` present → listed enabled, unlisted disabled;
 * - omitting either list preserves that list's default.
 * Every block gets `completed:false`; assessment blocks additionally seed `attempts:0`.
 * Slice starts `status:"not-started"`, `elapsedSeconds:0`.
 */
export function initSliceState(slice: SliceDefinition): SliceSessionState {
  const initial = slice.workflow.initialState;
  const visibleList = initial?.visibleBlockIds;
  const enabledList = initial?.enabledBlockIds;

  const blockStates: Record<string, BlockSessionState> = {};
  for (const block of slice.blocks) {
    const visible = visibleList === undefined ? true : visibleList.includes(block.id);
    const enabled = enabledList === undefined ? true : enabledList.includes(block.id);
    const state: BlockSessionState = { visible, enabled, completed: false };
    if (isAssessment(block)) state.attempts = 0;
    blockStates[block.id] = state;
  }

  return { status: "not-started", elapsedSeconds: 0, blockStates };
}

/** Returns a new SliceSessionState with `blockId`'s block state replaced by `next`. */
function withBlock(state: SliceSessionState, blockId: string, next: BlockSessionState): SliceSessionState {
  return { ...state, blockStates: { ...state.blockStates, [blockId]: next } };
}

function requireBlock(state: SliceSessionState, blockId: string): BlockSessionState {
  const block = state.blockStates[blockId];
  if (!block) throw new WorkflowRuntimeError(`sessionState: effect targets unknown block '${blockId}'`);
  return block;
}

/**
 * Folds one WorkflowEffect onto persisted block/slice state. Visibility and
 * interactivity effects flip the target block; `resetBlock` clears its progress;
 * `completeSlice` marks the slice completed. Transient effects (focus, narration,
 * timer, media play/pause) carry no persisted state and return `state` unchanged.
 * Never mutates; always returns a new object when something changes.
 *
 * Throws {@link WorkflowRuntimeError} on a block-targeting effect whose target
 * does not exist in the slice — an impossible state Slice 1 validation rules out.
 */
export function applyEffect(state: SliceSessionState, effect: WorkflowEffect): SliceSessionState {
  switch (effect.type) {
    case "show":
      return withBlock(state, effect.targetId, { ...requireBlock(state, effect.targetId), visible: true });
    case "hide":
      return withBlock(state, effect.targetId, { ...requireBlock(state, effect.targetId), visible: false });
    case "enable":
      return withBlock(state, effect.targetId, { ...requireBlock(state, effect.targetId), enabled: true });
    case "disable":
      return withBlock(state, effect.targetId, { ...requireBlock(state, effect.targetId), enabled: false });
    case "resetBlock":
      return withBlock(state, effect.targetId, {
        ...requireBlock(state, effect.targetId),
        completed: false,
        attempts: 0,
        answer: undefined,
      });
    case "completeSlice":
      return { ...state, status: "completed" };
    // Transient / non-persisted effects — no block-state change.
    case "focus":
    case "clearFocus":
    case "playNarration":
    case "pauseNarration":
    case "stopNarration":
    case "playBlock":
    case "pauseBlock":
    case "startTimer":
    case "cancelTimer":
    case "navigate":
      return state;
    default: {
      // Exhaustiveness guard: a new action type must be handled explicitly.
      const _never: never = effect;
      return _never;
    }
  }
}

/** A minimal runtime-event shape the reducer folds; matches CourseRuntimeEvent's relevant fields. */
export interface SessionStateEvent {
  type: string;
  sourceId: string;
  payload?: unknown;
}

const asRecord = (payload: unknown): Record<string, unknown> | undefined =>
  payload !== null && typeof payload === "object" ? (payload as Record<string, unknown>) : undefined;

/**
 * Folds one runtime event onto persisted block state (spec-anchored, small map):
 * - `answer.submitted` → `attempts += 1`, stores `payload.answer`;
 * - `answer.correct` / `block.completed` → marks the source block `completed:true`;
 * - `video.paused` / `video.ended` → records `payload.positionSeconds` as `mediaPositionSeconds`.
 * Events whose source is not a block in this slice (e.g. `narration.ended`, whose
 * source is a narration id) are ignored. Never mutates.
 */
export function applyEvent(state: SliceSessionState, event: SessionStateEvent): SliceSessionState {
  const block = state.blockStates[event.sourceId];

  switch (event.type) {
    case "answer.submitted": {
      if (!block) return state;
      const payload = asRecord(event.payload);
      return withBlock(state, event.sourceId, {
        ...block,
        attempts: (block.attempts ?? 0) + 1,
        answer: payload ? payload.answer : event.payload,
      });
    }
    case "answer.correct":
    case "block.completed": {
      if (!block) return state;
      return withBlock(state, event.sourceId, { ...block, completed: true });
    }
    case "video.paused":
    case "video.ended": {
      if (!block) return state;
      const payload = asRecord(event.payload);
      const position = payload && typeof payload.positionSeconds === "number" ? payload.positionSeconds : block.mediaPositionSeconds;
      return withBlock(state, event.sourceId, { ...block, mediaPositionSeconds: position });
    }
    default:
      return state;
  }
}
