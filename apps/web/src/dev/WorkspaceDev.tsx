/**
 * WorkspaceDev — assembled dev entry for the workspace.
 *
 * Creates a module-level store (backed by real window.localStorage) and a
 * Conversation, seeded once with the Phoebe demo task, then renders
 * <WorkspaceView />.
 *
 * Guard: only creates the Phoebe task if the store currently has no tasks so
 * that reloads / re-renders in development reuse the existing task.
 */

import { useState } from "react";
import { createStore } from "../store";
import { chat, loadConfig } from "../llm";
import { createConversation } from "../agent";
import { CARD_REGISTRY, deriveCatalog } from "@mind-imprint/contracts";
import { demoCatalog } from "../agent/prompt";
import { WorkspaceView } from "../workspace";

// ── Module-level singletons (created once per module load) ────────────────

const store = createStore({ storage: window.localStorage });

// Seed the Phoebe task only if the store is empty
let taskId: string;
const existingTasks = store.listTasks();
if (existingTasks.length > 0) {
  taskId = existingTasks[0]!.id;
} else {
  const task = store.createTask({
    title: "中国是否让地球变得更可持续？",
    seed:
      "我看到一篇说『中国让地球变绿』的公众号文章，想用它写中国让地球更可持续。文章链接：https://mp.weixin.qq.com/s/china-greening-earth-example",
  });
  taskId = task.id;
}

const catalog = demoCatalog(deriveCatalog(CARD_REGISTRY));

const conversation = createConversation({
  store,
  chat,
  config: loadConfig(),
  registry: CARD_REGISTRY,
  catalog,
  taskId,
});

// ── Component ─────────────────────────────────────────────────────────────

export function WorkspaceDev() {
  // onBack in the dev entry is a no-op (no task list view here)
  const [, setDummy] = useState(0);

  return (
    <div style={{ height: "calc(100vh - 53px)", display: "flex", flexDirection: "column" }}>
      <WorkspaceView
        store={store}
        conversation={conversation}
        taskId={taskId}
        onBack={() => setDummy((n) => n + 1)}
      />
    </div>
  );
}
