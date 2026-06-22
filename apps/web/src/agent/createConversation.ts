import type { CardInstance, CardSpec, Catalog } from "@mind-imprint/contracts";
import { SummonCardArgs } from "@mind-imprint/contracts";
import type { Store } from "../store/createStore";
import type { ChatRequest, ChatResult, LlmConfig } from "../llm/types";
import { LlmError } from "../llm/LlmError";
import { newEnvelope, envelopeReducer } from "../cards/envelopeReducer";
import { buildSystemPrompt, summonCardTool } from "./prompt";
import { buildLlmMessages } from "./messageMapping";

export type ConvPhase = "idle" | "awaiting_llm" | "proposal_pending" | "card_active" | "error";

export interface ConvState {
  taskId: string;
  phase: ConvPhase;
  pendingCardId?: string;
  error?: string;
}

type ChatFn = (config: Partial<LlmConfig>, req: ChatRequest) => Promise<ChatResult>;

export interface ConversationDeps {
  store: Store;
  chat: ChatFn;
  config: Partial<LlmConfig>;
  registry: Record<string, CardSpec>;
  catalog: Catalog;
  taskId: string;
  now?: () => string;
  genId?: () => string;
}

export interface Conversation {
  getSnapshot(): ConvState;
  subscribe(listener: () => void): () => void;
  send(text: string): Promise<void>;
  /**
   * Answer an opening message that is already in the store but has no reply yet
   * (e.g. the task was created from the directory with its first message seeded).
   * No-op unless the latest message is an unanswered user message and we are idle,
   * so it is safe to call on every workspace mount and after a reload.
   */
  kickoff(): Promise<void>;
  openCard(cardInstanceId: string): void;
  submitCard(cardInstanceId: string, finalInstance: CardInstance): Promise<void>;
  skipCard(cardInstanceId: string): Promise<void>;
}

export function createConversation(deps: ConversationDeps): Conversation {
  const { store, chat, config, registry, catalog, taskId } = deps;
  const now = deps.now ?? (() => new Date().toISOString());
  const genId = deps.genId ?? (() => crypto.randomUUID());

  if (!store.getTask(taskId)) throw new Error(`[createConversation] task "${taskId}" not in store`);

  let state: ConvState = { taskId, phase: "idle" };
  const listeners = new Set<() => void>();

  function setState(next: Partial<ConvState>): void {
    const nextState = { ...state, ...next };
    const changed = (Object.keys(nextState) as (keyof ConvState)[]).some(
      (k) => nextState[k] !== state[k],
    );
    if (changed) {
      state = nextState;
      listeners.forEach((l) => l());
    }
  }

  function buildMessages() {
    const systemPrompt = buildSystemPrompt(catalog);
    return buildLlmMessages({
      systemPrompt,
      messages: store.listMessages(taskId),
      cardById: (id) => store.getCard(id),
      specById: (id) => registry[id],
    });
  }

  async function callChat(): Promise<ChatResult> {
    const messages = buildMessages();
    const tools = [summonCardTool(catalog)];
    return chat(config, { messages, tools });
  }

  async function handleLlmError(err: unknown): Promise<void> {
    if (err instanceof LlmError) {
      setState({ phase: "error", error: err.message });
    } else {
      setState({ phase: "error", error: String(err) });
    }
  }

  /**
   * Run one LLM turn against the current store transcript: call the model, and
   * either record a card proposal or append a plain-text reply. Assumes the
   * triggering user message is already in the store (send() appends it first;
   * kickoff() answers a pre-seeded one).
   */
  async function runTurn(): Promise<void> {
    setState({ phase: "awaiting_llm", error: undefined, pendingCardId: undefined });

    let result: ChatResult;
    try {
      result = await callChat();
    } catch (err) {
      await handleLlmError(err);
      return;
    }

    // Look for summon_card tool calls — take first, warn about extras
    const summonCalls = (result.toolCalls ?? []).filter((tc) => tc.name === "summon_card");

    if (summonCalls.length > 0) {
      if (summonCalls.length > 1) {
        console.warn(
          `[createConversation] LLM returned ${summonCalls.length} summon_card calls; using only the first`,
        );
      }

      const toolCall = summonCalls[0]!;

      // Validate args
      const parsedArgs = SummonCardArgs.safeParse(toolCall.args);
      if (parsedArgs.success) {
        const { card_id, nudge_text } = parsedArgs.data;

        // Validate card_id ∈ registry
        if (registry[card_id]) {
          // Create proposed CardInstance
          const ci = newEnvelope(card_id, taskId, now, genId);
          store.putCard(ci);

          // Append assistant message with tool_call (SummonCardCall shape)
          const summonCardCall = {
            id: toolCall.id,
            name: "summon_card" as const,
            args: parsedArgs.data,
            card_instance_id: ci.id,
          };

          store.appendMessage({
            task_id: taskId,
            role: "assistant",
            content: nudge_text,
            tool_call: summonCardCall,
          });

          setState({ phase: "proposal_pending", pendingCardId: ci.id });
          return;
        }
        // else: unknown card_id — fall through to plain text
      }
      // Invalid args or unknown card_id — fall through to plain text
    }

    // Plain text reply
    store.appendMessage({ task_id: taskId, role: "assistant", content: result.text });
    setState({ phase: "idle", pendingCardId: undefined });
  }

  return {
    getSnapshot(): ConvState {
      return state;
    },

    subscribe(listener: () => void): () => void {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },

    async send(text: string): Promise<void> {
      store.appendMessage({ task_id: taskId, role: "user", content: text });
      await runTurn();
    },

    async kickoff(): Promise<void> {
      // Only answer a genuinely pending opening message: we must be idle, and
      // the latest message must be an unanswered user message. This keeps the
      // call idempotent across remounts/StrictMode and reloads — once the model
      // has replied, the latest message is an assistant message and this no-ops.
      if (state.phase !== "idle") return;
      const messages = store.listMessages(taskId);
      const last = messages[messages.length - 1];
      if (!last || last.role !== "user") return;
      await runTurn();
    },

    openCard(cardInstanceId: string): void {
      const ci = store.getCard(cardInstanceId);
      if (!ci) throw new Error(`[createConversation] unknown card instance "${cardInstanceId}"`);
      const activated = envelopeReducer(ci, { type: "activate" }, now);
      store.putCard(activated);
      setState({ phase: "card_active" });
    },

    async submitCard(cardInstanceId: string, finalInstance: CardInstance): Promise<void> {
      store.putCard(finalInstance);
      setState({ phase: "awaiting_llm", pendingCardId: undefined });

      let result: ChatResult;
      try {
        result = await callChat();
      } catch (err) {
        await handleLlmError(err);
        return;
      }

      store.appendMessage({ task_id: taskId, role: "assistant", content: result.text });
      setState({ phase: "idle", pendingCardId: undefined });
    },

    async skipCard(cardInstanceId: string): Promise<void> {
      const ci = store.getCard(cardInstanceId);
      if (!ci) throw new Error(`[createConversation] unknown card instance "${cardInstanceId}"`);
      const skipped = envelopeReducer(ci, { type: "skip" }, now);
      store.putCard(skipped);
      setState({ phase: "awaiting_llm", pendingCardId: undefined });

      let result: ChatResult;
      try {
        result = await callChat();
      } catch (err) {
        await handleLlmError(err);
        return;
      }

      store.appendMessage({ task_id: taskId, role: "assistant", content: result.text });
      setState({ phase: "idle", pendingCardId: undefined });
    },
  };
}
