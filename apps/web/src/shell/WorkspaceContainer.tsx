import { useMemo, useEffect } from "react";
import { api } from "../api";
import { createConversation } from "../agent/createConversation";
import { createEvaluator } from "../agent/createEvaluator";
import { WorkspaceView } from "../workspace";
import type { Store } from "../store/createStore";

interface Props {
  store: Store;
  taskId: string;
  onBack: () => void;
  openingMessage?: string;
}

export function WorkspaceContainer({ store, taskId, onBack, openingMessage }: Props) {
  const { conversation, evaluator } = useMemo(
    () => ({
      conversation: createConversation({ api, store, taskId }),
      evaluator: createEvaluator({ api, store, taskId }),
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [taskId],
  );

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const detail = await api.getTask(taskId);
        if (cancelled) return;
        const evaluation = await api.getEvaluation(taskId);
        if (cancelled) return;
        store.hydrateTask(taskId, { ...detail, evaluation: evaluation ?? undefined });
      } catch {
        // hydration failure leaves the cache as-is; a fresh task simply has none
      }
      if (cancelled) return;
      // Fresh task created from the directory: no server messages yet — send the opening line.
      if (openingMessage && store.listMessages(taskId).length === 0) {
        void conversation.send(openingMessage);
      }
    })();
    return () => { cancelled = true; };
  }, [taskId, conversation, openingMessage, store]);

  return <WorkspaceView store={store} conversation={conversation} evaluator={evaluator} taskId={taskId} onBack={onBack} />;
}
