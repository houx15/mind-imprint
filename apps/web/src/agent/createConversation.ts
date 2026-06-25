import type { CardInstance } from "@mind-imprint/contracts";
import type { ApiClient } from "../api";
import type { Store } from "../store/createStore";
import { newEnvelope, envelopeReducer } from "../cards/envelopeReducer";

export type ConvPhase = "idle" | "awaiting_llm" | "proposal_pending" | "card_active" | "error";
export interface ConvState { taskId: string; phase: ConvPhase; pendingCardId?: string; error?: string }

export interface ConversationDeps {
  api: Pick<ApiClient, "runTurn" | "activateCard" | "submitCard" | "skipCard">;
  store: Store;
  taskId: string;
  now?: () => string;
  genId?: () => string;
}

export interface Conversation {
  getSnapshot(): ConvState;
  subscribe(listener: () => void): () => void;
  send(text: string): Promise<void>;
  openCard(cardInstanceId: string): void;
  closeCard(cardInstanceId: string): void;
  submitCard(cardInstanceId: string, finalInstance: CardInstance): Promise<void>;
  skipCard(cardInstanceId: string): Promise<void>;
}

export function createConversation(deps: ConversationDeps): Conversation {
  const { api, store, taskId } = deps;
  const now = deps.now ?? (() => new Date().toISOString());
  const genId = deps.genId ?? (() => crypto.randomUUID());

  let state: ConvState = { taskId, phase: "idle" };
  const listeners = new Set<() => void>();
  function setState(next: Partial<ConvState>): void {
    const merged = { ...state, ...next };
    if ((Object.keys(merged) as (keyof ConvState)[]).some((k) => merged[k] !== state[k])) {
      state = merged;
      listeners.forEach((l) => l());
    }
  }

  // Stream one turn; userInput undefined ⇒ continuation turn (after a card).
  async function streamTurn(userInput?: string): Promise<void> {
    setState({ phase: "awaiting_llm", error: undefined, pendingCardId: undefined });
    const assistantId = genId();
    let text = "";
    let assistantCreated = false;
    try {
      for await (const ev of api.runTurn(taskId, userInput)) {
        if (ev.type === "text") {
          text += ev.delta;
          store.putMessage({ id: assistantId, task_id: taskId, role: "assistant", content: text, tool_call: null, created_at: now() });
          assistantCreated = true;
        } else if (ev.type === "card") {
          const ci = newEnvelope(ev.cardId, taskId, now, () => ev.cardInstanceId);
          store.putCard(ci);
          store.putMessage({
            id: assistantId, task_id: taskId, role: "assistant", content: text, created_at: now(),
            tool_call: { id: ev.cardInstanceId, name: "summon_card", args: { card_id: ev.cardId, reason: "", nudge_text: ev.nudgeText }, card_instance_id: ev.cardInstanceId },
          });
          setState({ phase: "proposal_pending", pendingCardId: ev.cardInstanceId });
          return; // one card per turn — the card ends the turn
        } else if (ev.type === "done") {
          store.putMessage({ id: assistantId, task_id: taskId, role: "assistant", content: text, tool_call: null, created_at: now() });
          setState({ phase: "idle", pendingCardId: undefined });
          return;
        } else if (ev.type === "error") {
          if (assistantCreated) store.removeMessage(assistantId);
          setState({ phase: "error", error: ev.message });
          return;
        }
      }
      setState({ phase: "idle" });
    } catch (err) {
      if (assistantCreated) store.removeMessage(assistantId);
      setState({ phase: "error", error: err instanceof Error ? err.message : String(err) });
    }
  }

  return {
    getSnapshot: () => state,
    subscribe(listener) { listeners.add(listener); return () => { listeners.delete(listener); }; },

    async send(text) {
      store.appendMessage({ task_id: taskId, role: "user", content: text });
      await streamTurn(text);
    },

    openCard(cardInstanceId) {
      const ci = store.getCard(cardInstanceId);
      if (!ci) throw new Error(`[conversation] unknown card "${cardInstanceId}"`);
      store.putCard(envelopeReducer(ci, { type: "activate" }, now));
      setState({ phase: "card_active", pendingCardId: cardInstanceId });
      // Mark opened on the server so the live tree reflects it (non-blocking).
      void api.activateCard(taskId, cardInstanceId).then((c) => store.putCard(c)).catch(() => {});
    },

    closeCard() {
      setState({ phase: "idle", pendingCardId: undefined }); // close ≠ skip
    },

    async submitCard(cardInstanceId, finalInstance) {
      store.putCard(finalInstance); // optimistic completed
      setState({ phase: "awaiting_llm", pendingCardId: undefined });
      try {
        const server = await api.submitCard(taskId, cardInstanceId, finalInstance);
        store.putCard(server);
      } catch (err) {
        setState({ phase: "error", error: err instanceof Error ? err.message : String(err) });
        return;
      }
      await streamTurn(); // continuation turn
    },

    async skipCard(cardInstanceId) {
      const ci = store.getCard(cardInstanceId);
      if (!ci) throw new Error(`[conversation] unknown card "${cardInstanceId}"`);
      const skipped = envelopeReducer(ci, { type: "skip" }, now);
      store.putCard(skipped);
      setState({ phase: "awaiting_llm", pendingCardId: undefined });
      try {
        const server = await api.skipCard(taskId, cardInstanceId, skipped.event_trace);
        store.putCard(server);
      } catch (err) {
        setState({ phase: "error", error: err instanceof Error ? err.message : String(err) });
        return;
      }
      await streamTurn(); // continuation turn
    },
  };
}
