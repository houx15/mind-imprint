import { useCallback, useRef, useState } from "react";
import type { WorkspaceCard } from "../../api/teacherWorkspace";
import { useAlive } from "../../shared/useAlive";
import {
  beginTurn,
  initialThread,
  isPendingCurrent,
  resetThread,
  setComposer,
  settleFailure,
  settleStale,
  settleSuccess,
  type ThreadInput,
  type ThreadState,
} from "./threadLogic";
import { applyPatch, type Choice, type Turn } from "./workspaceLogic";

// teacher/workspace/useWorkspaceThread.ts — one teacher-workspace
// conversation (homework card, class home, parent report). The transitions
// are in `threadLogic.ts`; this file wires them to a request and to the
// caller's artifact.

export type { ThreadInput } from "./threadLogic";

/** What one surface's endpoint returns for a turn. */
export interface ThreadReply<A> {
  reply: string;
  /** Only the artifact fields the turn wrote. */
  patch: Partial<A>;
  choices: Choice[];
  cards: WorkspaceCard[];
}

export interface WorkspaceThreadOptions<A extends object> {
  /** The artifact as it is in this render. The hook keeps it in a ref
   *  assigned during render, so a response that lands later is applied
   *  against what she has NOW, not what she had when she sent. */
  artifact: A;
  setArtifact: (next: A) => void;
  /** The key a turn is scoped to on the server (the class id today). A
   *  response is dropped when the live artifact's scope no longer matches
   *  the one it was sent under. */
  scopeOf: (artifact: A) => string;
  /** Sends one turn. `artifact` is the snapshot taken when the turn started;
   *  `turns` is already windowed (`trimTurns`) and includes this turn. */
  post: (req: { artifact: A; turns: Turn[]; input: ThreadInput }) => Promise<ThreadReply<A>>;
  /** The error line for a failed turn, e.g. `failText("对话", e)`. */
  describeError: (e: unknown) => string;
}

export interface WorkspaceThread<A extends object> {
  turns: Turn[];
  busy: boolean;
  error: string | null;
  /** Clamped; pass straight to `WorkspacePanel`. */
  choices: Choice[];
  cards: WorkspaceCard[];
  /** Fields where her own edit beat the last turn's patch. */
  kept: (keyof A)[];
  /** The composer's text (controlled; pass to `WorkspacePanel`). A failed
   *  typed sentence is written back here when the composer is empty. */
  composer: string;
  setComposer: (text: string) => void;
  /** The input of the last failed turn in this conversation, or null. */
  failed: ThreadInput | null;
  /** Sends one turn. Returns false (and changes nothing) while a turn is in
   *  flight. On acceptance, a composer holding exactly this text is cleared. */
  run: (input: ThreadInput) => boolean;
  /** Sends `failed` again. No-op when there is none or a turn is in flight. */
  retry: () => void;
  /** Clears the conversation. Any turn in flight is dropped when it lands. */
  reset: () => void;
}

/**
 * One workspace conversation, owned by whichever component must outlive the
 * panel (the homework page owns it above its 传统/AI toggle, so switching
 * modes keeps the thread).
 *
 * Rules it keeps for every surface:
 * - the request carries `trimTurns` of the history, never all of it;
 * - the artifact is snapshotted before sending, and the patch is applied
 *   with `applyPatch(live, snapshot, patch)` when the response lands, so a
 *   field she edited meanwhile is kept (reported in `kept`);
 * - a response is applied only if the generation and scope it was sent under
 *   are still current — this holds for failures too; `reset()` bumps the
 *   generation. A scope change that did not go through `reset()` resets the
 *   thread when the in-flight response lands (`settleStale`), so `busy` is
 *   never stuck; callers should still call `reset()` on every scope change;
 * - a current failure removes her optimistic bubble (`rollbackTurn`), sets
 *   `error` and `failed`, and puts a typed sentence back into an empty
 *   composer;
 * - choices are clamped (`clampChoices`).
 *
 * State lives in a ref that is the source of truth and is mirrored into React
 * state on every change, so `busy` and `gen` are read synchronously (a double
 * click cannot start two turns; `reset()` takes effect before the next
 * `await` resolves).
 */
export function useWorkspaceThread<A extends object>(opts: WorkspaceThreadOptions<A>): WorkspaceThread<A> {
  const alive = useAlive();
  const ref = useRef<ThreadState<keyof A>>(initialThread<keyof A>());
  const [state, setState] = useState(ref.current);

  const optsRef = useRef(opts);
  optsRef.current = opts;

  const commit = useCallback((next: ThreadState<keyof A>) => {
    if (next === ref.current) return;
    ref.current = next;
    if (alive.current) setState(next);
  }, [alive]);

  const run = useCallback(
    (input: ThreadInput) => {
      const o = optsRef.current;
      const snapshot = o.artifact;
      const started = beginTurn(ref.current, input, o.scopeOf(snapshot));
      if (!started) return false;
      commit(started.state);
      const { pending } = started;
      o.post({ artifact: snapshot, turns: started.wireTurns, input }).then(
        (res) => {
          if (!alive.current) return;
          const live = optsRef.current;
          const scopeNow = live.scopeOf(live.artifact);
          // Checked before `setArtifact`: a stale patch must never reach the
          // artifact. `settleStale` still runs, so a same-generation scope
          // change releases `busy`.
          if (!isPendingCurrent(ref.current, pending, scopeNow)) {
            commit(settleStale(ref.current, pending));
            return;
          }
          const { next, kept } = applyPatch(live.artifact, snapshot, res.patch);
          live.setArtifact(next);
          commit(settleSuccess(ref.current, pending, scopeNow, { ...res, kept }));
        },
        (e: unknown) => {
          if (!alive.current) return;
          const live = optsRef.current;
          const scopeNow = live.scopeOf(live.artifact);
          commit(settleFailure(ref.current, pending, scopeNow, input, live.describeError(e)));
        },
      );
      return true;
    },
    [alive, commit],
  );

  const retry = useCallback(() => {
    const failed = ref.current.failed;
    if (failed && !ref.current.busy) run(failed);
  }, [run]);

  const reset = useCallback(() => commit(resetThread(ref.current)), [commit]);
  const setText = useCallback((text: string) => commit(setComposer(ref.current, text)), [commit]);

  return {
    turns: state.turns,
    busy: state.busy,
    error: state.error,
    choices: state.choices,
    cards: state.cards,
    kept: state.kept,
    composer: state.composer,
    setComposer: setText,
    failed: state.failed,
    run,
    retry,
    reset,
  };
}
