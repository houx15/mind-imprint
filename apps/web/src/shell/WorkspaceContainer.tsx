import { useMemo, useEffect } from "react";
import type { Store } from "../store/createStore";
import type { ChatFn } from "../agent/runEvaluation";
import type { LlmConfig } from "../llm/types";
import { chat as realChat, loadConfig } from "../llm";
import { createConversation, createEvaluator } from "../agent";
import { demoCatalog } from "../agent/prompt";
import { CARD_REGISTRY, deriveCatalog } from "@mind-imprint/contracts";
import { WorkspaceView } from "../workspace";

const CATALOG = demoCatalog(deriveCatalog(CARD_REGISTRY));

interface Props {
  store: Store;
  taskId: string;
  onBack: () => void;
  chat?: ChatFn;
  config?: Partial<LlmConfig>;
}

export function WorkspaceContainer({ store, taskId, onBack, chat, config }: Props) {
  const cfg = config ?? loadConfig();
  const chatFn = chat ?? realChat;
  const { conversation, evaluator } = useMemo(() => ({
    conversation: createConversation({ store, chat: chatFn, config: cfg, registry: CARD_REGISTRY, catalog: CATALOG, taskId }),
    evaluator: createEvaluator({ store, chat: chatFn, config: cfg, registry: CARD_REGISTRY, taskId }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }), [taskId]);

  // A task opened from the directory carries its opening message but no reply
  // yet — kick off the first LLM turn. kickoff() is idempotent (no-ops once the
  // assistant has replied), so this is safe across remounts and reloads.
  useEffect(() => {
    void conversation.kickoff();
  }, [conversation]);

  return (
    <WorkspaceView store={store} conversation={conversation} evaluator={evaluator} taskId={taskId} onBack={onBack} />
  );
}
