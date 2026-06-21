import { EvalLlmOutput, Evaluation, DEMO_RUBRIC } from "@mind-imprint/contracts";
import type { CardSpec } from "@mind-imprint/contracts";
import type { Store } from "../store/createStore";
import type { ChatRequest, ChatResult, LlmConfig } from "../llm/types";
import { assembleEvalInput } from "./evalInput";
import { buildEvalPrompt } from "./evalPrompt";

export type ChatFn = (config: Partial<LlmConfig>, req: ChatRequest) => Promise<ChatResult>;

export interface RunEvaluationDeps {
  store: Store;
  chat: ChatFn;
  config: Partial<LlmConfig>;
  registry: Record<string, CardSpec>;
  taskId: string;
  now?: () => string;
}

/**
 * Parse the raw LLM output text into a validated EvalLlmOutput.
 * Strips optional ```json fences before parsing.
 * Throws on any failure (JSON parse error or Zod validation error).
 */
function parseEvalOutput(text: string): EvalLlmOutput {
  // Strip leading/trailing ```json fences if present
  let cleaned = text.trim();
  if (cleaned.startsWith("```json")) {
    cleaned = cleaned.slice("```json".length).trimStart();
  } else if (cleaned.startsWith("```")) {
    cleaned = cleaned.slice(3).trimStart();
  }
  if (cleaned.endsWith("```")) {
    cleaned = cleaned.slice(0, -3).trimEnd();
  }

  const parsed = JSON.parse(cleaned);
  return EvalLlmOutput.parse(parsed);
}

/**
 * Runs a full evaluation cycle:
 * 1. Assembles the eval input from store data (messages + cards)
 * 2. Builds the system prompt from DEMO_RUBRIC
 * 3. Calls the flagship LLM with up to 1 retry on parse failure
 * 4. Persists and returns the Evaluation
 */
export async function runEvaluation(deps: RunEvaluationDeps): Promise<Evaluation> {
  const { store, chat, config, registry, taskId } = deps;
  const now = deps.now ?? (() => new Date().toISOString());

  const system = buildEvalPrompt(DEMO_RUBRIC);
  const user = assembleEvalInput({
    messages: store.listMessages(taskId),
    cards: store.listCards(taskId),
    registry,
  });

  const model = config.evalModel ?? config.model;
  const chatConfig = { ...config, model };
  const chatRequest: ChatRequest = {
    messages: [
      { role: "system", content: system },
      { role: "user", content: user },
    ],
    maxTokens: 1500,
  };

  // First attempt
  const firstResult = await chat(chatConfig, chatRequest);

  let out: EvalLlmOutput;
  try {
    out = parseEvalOutput(firstResult.text);
  } catch {
    // Retry once on parse/validation failure
    const retryResult = await chat(chatConfig, chatRequest);
    try {
      out = parseEvalOutput(retryResult.text);
    } catch {
      throw new Error("评估输出解析失败");
    }
  }

  const evaluation: Evaluation = {
    task_id: taskId,
    scores: out.scores,
    narrative: out.narrative,
    created_at: now(),
  };

  store.putEvaluation(evaluation);
  return evaluation;
}
