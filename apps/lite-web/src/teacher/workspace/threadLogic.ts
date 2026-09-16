import type { WorkspaceCard } from "../../api/teacherWorkspace";
import { clampChoices, isCurrentTurn, rollbackTurn, trimTurns, type Choice, type Turn } from "./workspaceLogic";

// teacher/workspace/threadLogic.ts — the state transitions of one workspace
// conversation, as pure functions. `useWorkspaceThread.ts` is the only
// caller; it holds a `ThreadState` in a ref and commits each result to React.
// Nothing here knows which surface (homework, class home, parent report) the
// conversation belongs to.

/** What she sent this turn: a typed sentence, or a tapped choice. `label` is
 *  what appears in her bubble; `slug` is echoed back as `choiceSlug`. */
export type ThreadInput = { text: string } | { choiceId: string; label: string; slug?: string };

/** The text her bubble shows for an input. */
export function inputText(input: ThreadInput): string {
  return "text" in input ? input.text : input.label;
}

export interface ThreadState<K extends PropertyKey = string> {
  /** Bumped by `resetThread`. A turn records the value it started under; a
   *  response carrying an older value belongs to a cleared conversation. */
  gen: number;
  turns: Turn[];
  busy: boolean;
  error: string | null;
  /** Already clamped. */
  choices: Choice[];
  cards: WorkspaceCard[];
  /** Artifact keys whose patched value was dropped because she edited them
   *  while the turn was in flight (`applyPatch`'s `kept`). */
  kept: K[];
  /** The composer's text. Held here, not in the panel, so it survives the
   *  panel unmounting and so a failed turn can put her sentence back. */
  composer: string;
  /** The input of the last turn that failed in this generation. 重试 sends it
   *  again; cleared when a new turn starts or the thread resets. */
  failed: ThreadInput | null;
}

/** Identifies one in-flight turn: the generation and scope (e.g. class id) it
 *  was sent under, and where its optimistic bubble sits. */
export interface PendingTurn {
  gen: number;
  scope: string;
  index: number;
  text: string;
}

export function initialThread<K extends PropertyKey = string>(): ThreadState<K> {
  return { gen: 0, turns: [], busy: false, error: null, choices: [], cards: [], kept: [], composer: "", failed: null };
}

export function setComposer<K extends PropertyKey>(state: ThreadState<K>, composer: string): ThreadState<K> {
  return state.composer === composer ? state : { ...state, composer };
}

/** Whether a failed sentence should be written back into the composer. Only
 *  when the composer is empty (whitespace counts as empty): anything she has
 *  typed since is newer than the failed sentence and is kept. */
export function shouldRestore(composer: string): boolean {
  return composer.trim() === "";
}

/** Starts a turn. Returns null while another turn is in flight. `wireTurns`
 *  is the windowed history to send (`trimTurns`), including the new turn.
 *
 *  If the composer still holds exactly this sentence (a failed sentence was
 *  restored and she pressed 重试 instead of 发送), it is cleared so the same
 *  sentence is not left there to be sent a second time. */
export function beginTurn<K extends PropertyKey>(
  state: ThreadState<K>,
  input: ThreadInput,
  scope: string,
): { state: ThreadState<K>; pending: PendingTurn; wireTurns: Turn[] } | null {
  if (state.busy) return null;
  const text = inputText(input);
  const turns = [...state.turns, { role: "teacher" as const, text }];
  const composer = "text" in input && state.composer.trim() === input.text.trim() ? "" : state.composer;
  return {
    state: { ...state, turns, busy: true, error: null, kept: [], failed: null, composer },
    pending: { gen: state.gen, scope, index: turns.length - 1, text },
    wireTurns: trimTurns(turns),
  };
}

/** Whether `pending`'s response still belongs to the conversation on screen.
 *  `scopeNow` is read from the LIVE artifact when the response lands. */
export function isPendingCurrent<K extends PropertyKey>(
  state: ThreadState<K>,
  pending: PendingTurn,
  scopeNow: string,
): boolean {
  return isCurrentTurn({ gen: pending.gen, classId: pending.scope }, { gen: state.gen, classId: scopeNow });
}

/** The transition for a response that is not current.
 *
 *  - Older generation: `state` unchanged (same object), including `busy`.
 *    A reset already cleared that conversation, and `busy` now belongs to
 *    whatever the new generation is doing.
 *  - Same generation, different scope: the scope moved without anyone
 *    calling `reset()`. The conversation on screen belongs to the old scope
 *    and this turn is the one holding `busy`, so leaving `state` unchanged
 *    would keep `busy` true until remount. It is treated as the reset that
 *    should have happened — the transition a class change uses — which drops
 *    the old scope's turns, clears `busy`, and bumps the generation. */
export function settleStale<K extends PropertyKey>(state: ThreadState<K>, pending: PendingTurn): ThreadState<K> {
  return pending.gen === state.gen ? resetThread(state) : state;
}

/** A response arrived. A stale one goes through `settleStale`. `kept` is
 *  computed by the caller with `applyPatch` against the live artifact. */
export function settleSuccess<K extends PropertyKey>(
  state: ThreadState<K>,
  pending: PendingTurn,
  scopeNow: string,
  res: { reply: string; choices: Choice[]; cards: WorkspaceCard[]; kept: K[] },
): ThreadState<K> {
  if (!isPendingCurrent(state, pending, scopeNow)) return settleStale(state, pending);
  return {
    ...state,
    turns: [...state.turns, { role: "ai", text: res.reply }],
    busy: false,
    error: null,
    choices: clampChoices(res.choices),
    cards: res.cards,
    kept: res.kept,
  };
}

/** A request failed. A stale failure goes through `settleStale`. A current one
 *  removes her optimistic bubble (`rollbackTurn`), records `input` for 重试,
 *  and puts her sentence back into the composer if the composer is empty. */
export function settleFailure<K extends PropertyKey>(
  state: ThreadState<K>,
  pending: PendingTurn,
  scopeNow: string,
  input: ThreadInput,
  message: string,
): ThreadState<K> {
  if (!isPendingCurrent(state, pending, scopeNow)) return settleStale(state, pending);
  const restore = "text" in input && shouldRestore(state.composer);
  return {
    ...state,
    turns: rollbackTurn(state.turns, pending.index, pending.text),
    busy: false,
    error: message,
    failed: input,
    composer: restore ? input.text : state.composer,
  };
}

/** Clears the conversation and bumps the generation, so a turn already in
 *  flight is dropped when it lands (success or failure). The composer is kept:
 *  it is what she is typing now, not part of the old conversation. */
export function resetThread<K extends PropertyKey>(state: ThreadState<K>): ThreadState<K> {
  return { ...initialThread<K>(), gen: state.gen + 1, composer: state.composer };
}
