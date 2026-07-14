import type { Anchor, ActiveCard, CardSpec, TraceEvent } from "@mind-imprint/contracts";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { CoachMessage } from "./state";
import { studioTurn as defaultStudioTurn, postDisposition as defaultPostDisposition, type StudioTurnEvent } from "../api/studioTurn";
import {
  activateProjectCard as defaultActivateProjectCard,
  submitProjectCard as defaultSubmitProjectCard,
  skipProjectCard as defaultSkipProjectCard,
} from "../api/projectCards";

export type CardState = { cardInstanceId: string; cardId: string; spec: CardSpec; status: "proposed" | "active"; anchors: Anchor[]; materialId?: string } | null;

// activeCardToState maps the server-projected StudioProjection.activeCard
// into the same local CardState shape a "card" SSE event produces (the spec
// is resolved from CARD_REGISTRY either way, never sent over the wire twice).
// null when there is no open card, or its cardId names a card this build
// doesn't recognize (defensive — never crash the rehydration path).
export function activeCardToState(ac: ActiveCard | null): CardState {
  if (!ac) return null;
  const spec = CARD_REGISTRY[ac.cardId];
  if (!spec) return null;
  return {
    cardInstanceId: ac.cardInstanceId,
    cardId: ac.cardId,
    spec,
    status: ac.status,
    anchors: ac.anchors,
    materialId: ac.materialId || undefined,
  };
}

type Snapshot = {
  messages: CoachMessage[];
  sending: boolean;
  error: string | null;
  disposableInterventionId: string | null;
  card: CardState;
};

type Deps = {
  projectId: string;
  // The card the server projected as already proposed/active (StudioProjection
  // .activeCard) — seeds the conversation's local card state so a page reload
  // rehydrates the open card instead of losing the client's only reference to
  // it (whole-branch review CRITICAL: FIX-D's suppression would otherwise
  // block every future card from surfacing, forever, once that happens).
  initialCard?: CardState;
  api?: {
    studioTurn: typeof defaultStudioTurn;
    postDisposition: typeof defaultPostDisposition;
    activateProjectCard: typeof defaultActivateProjectCard;
    submitProjectCard: typeof defaultSubmitProjectCard;
    skipProjectCard: typeof defaultSkipProjectCard;
  };
};

export function createStudioConversation({ projectId, api, initialCard }: Deps) {
  const turn = api?.studioTurn ?? defaultStudioTurn;
  const dispose = api?.postDisposition ?? defaultPostDisposition;
  const activateCard = api?.activateProjectCard ?? defaultActivateProjectCard;
  const submitCardTurn = api?.submitProjectCard ?? defaultSubmitProjectCard;
  const skipCardApi = api?.skipProjectCard ?? defaultSkipProjectCard;
  let state: Snapshot = { messages: [], sending: false, error: null, disposableInterventionId: null, card: initialCard ?? null };
  const listeners = new Set<() => void>();
  const emit = () => listeners.forEach((l) => l());
  const set = (p: Partial<Snapshot>) => { state = { ...state, ...p }; emit(); };

  // allowSurfaceOverActive: false (the default, used by an ordinary send()
  // turn) refuses to let an incoming "card" frame overwrite a DIFFERENT card
  // that is currently "active" (open, being filled) — whole-branch review
  // CRITICAL 1's defense in depth. FIX-D already suppresses a competing
  // surface_card server-side while any card_instance is proposed/active, but
  // this function must not itself be the kind of thing that discards a
  // student's open, half-filled card just because a frame arrived. submitCard
  // passes true: its own post-submit refeed legitimately introduces the NEXT
  // card in the same round trip, arriving BEFORE this function has even seen
  // the "done" event that will retire the just-submitted one — at that
  // instant local state.card is still the old id/status, so the naive check
  // would otherwise misfire on a completely legitimate transition.
  function applyEvent(e: StudioTurnEvent, opts?: { allowSurfaceOverActive?: boolean }) {
    if (e.type === "intervention") {
      set({
        messages: [...state.messages, { kind: "ai", body: e.body, tag: e.criterion || undefined, anchor: e.anchor || undefined }],
        disposableInterventionId: e.interventionId,
      });
    } else if (e.type === "card") {
      if (!opts?.allowSurfaceOverActive && state.card?.status === "active" && state.card.cardInstanceId !== e.cardInstanceId) {
        return;
      }
      const spec = CARD_REGISTRY[e.cardId];
      if (spec) set({ card: { cardInstanceId: e.cardInstanceId, cardId: e.cardId, spec, status: "proposed", anchors: e.anchors, materialId: e.materialId } });
    } else if (e.type === "error") {
      set({ error: e.message });
    }
  }

  async function send(text: string) {
    set({ messages: [...state.messages, { kind: "student", body: text }], sending: true, error: null });
    try {
      for await (const e of turn(projectId, text) as AsyncGenerator<StudioTurnEvent>) {
        applyEvent(e);
      }
    } catch {
      set({ error: "对话失败，请重试" });
    } finally {
      set({ sending: false });
    }
  }

  async function disposeIntervention(action: "accept" | "rewrite" | "reject", reason: string) {
    if (!state.disposableInterventionId) return;
    await dispose(projectId, state.disposableInterventionId, action, reason);
  }

  async function openCard() {
    if (!state.card) return;
    const activatedId = state.card.cardInstanceId;
    await activateCard(projectId, activatedId);
    if (state.card?.cardInstanceId === activatedId) {
      set({ card: { ...state.card, status: "active" } });
    }
  }

  async function submitCard(finalEnvelope: { field_values: Record<string, unknown>; event_trace: TraceEvent[]; anchors: Anchor[] }) {
    if (!state.card) return;
    const cardInstanceId = state.card.cardInstanceId;
    const submittedId = cardInstanceId;
    set({ sending: true, error: null });
    try {
      for await (const e of submitCardTurn(projectId, cardInstanceId, finalEnvelope) as AsyncGenerator<StudioTurnEvent>) {
        applyEvent(e, { allowSurfaceOverActive: true });
        if (e.type === "done") {
          if (state.card?.cardInstanceId === submittedId) {
            set({ card: null });
          }
        }
      }
    } catch {
      set({ error: "对话失败，请重试" });
    } finally {
      set({ sending: false });
    }
  }

  async function skipCard(eventTrace: TraceEvent[]) {
    if (!state.card) return;
    await skipCardApi(projectId, state.card.cardInstanceId, { event_trace: eventTrace });
    set({ card: null });
  }

  // dropFirst removes only the first `n` messages from this session's local
  // turn buffer — the projection (fetched via refetchProject) is now the
  // single source of truth for that prefix (it's what the GET snapshot
  // actually contains), so keeping them here too would render each one
  // twice (CoachRail is keyed by array index with no dedupe). `n` MUST be
  // captured by the caller before the GET is issued: any message that enters
  // the buffer during the round trip is not yet in that snapshot and must
  // survive — dropping it would silently delete the student's own words
  // (Slice 6b fix-wave bug [A]). card/sending/error/disposableInterventionId
  // are untouched: a refetch reconciles history, not this session's live
  // interaction state.
  function dropFirst(n: number) {
    set({ messages: state.messages.slice(n) });
  }

  return {
    getSnapshot: () => state,
    subscribe: (l: () => void) => { listeners.add(l); return () => listeners.delete(l); },
    send,
    dispose: disposeIntervention,
    openCard,
    submitCard,
    skipCard,
    dropFirst,
  };
}
