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
  now?: () => string;
  genId?: () => string;
}

export interface Conversation {
  getSnapshot(): ConvState;
  subscribe(listener: () => void): () => void;
  send(text: string): Promise<void>;
  openCard(cardInstanceId: string): void;
  submitCard(cardInstanceId: string, finalInstance: CardInstance): Promise<void>;
  skipCard(cardInstanceId: string): Promise<void>;
}

export function createConversation(deps: ConversationDeps): Conversation {
  const { store, chat, config, registry, catalog } = deps;
  const now = deps.now ?? (() => new Date().toISOString());
  const genId = deps.genId ?? (() => crypto.randomUUID());

  // Use the most recently created task in the store
  const tasks = store.listTasks();
  if (tasks.length === 0) throw new Error("[createConversation] no task in store");
  const taskId = tasks[tasks.length - 1]!.id;

  let state: ConvState = { taskId, phase: "idle" };
  const listeners = new Set<() => void>();

  function setState(next: Partial<ConvState>): void {
    const nextState = { ...state, ...next };
    const changed = (Object.keys(nextState) as (keyof ConvState)[]).some(
      (k) => nextState[k] !== state[k],
    );
    state = nextState;
    if (changed) {
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
      setState({ phase: "awaiting_llm" });

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
      setState({ phase: "awaiting_llm" });

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
