import { useMemo, useEffect } from "react";
import { api } from "../api";
import { synthesize } from "../api/voice";
import { player } from "../audio/player";
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
      conversation: createConversation({
        api,
        store,
        taskId,
        speak: (text) => { void synthesize(text).then((url) => player.play(url)).catch(() => {}); },
      }),
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

  useEffect(() => {
    const POLL_MS = 30_000;
    let cancelled = false;
    const id = setInterval(() => {
      void (async () => {
        try {
          const latest = await api.getEvaluation(taskId);
          if (cancelled || !latest || latest.status !== "done") return;
          const known = store.getLatestEvaluation(taskId);
          if (!known || latest.created_at > known.created_at) store.putEvaluation(latest);
        } catch {
          // best-effort: a failed poll is silently ignored
        }
      })();
    }, POLL_MS);
    return () => { cancelled = true; clearInterval(id); };
  }, [taskId, store]);

  return <WorkspaceView store={store} conversation={conversation} evaluator={evaluator} taskId={taskId} onBack={onBack} />;
}
