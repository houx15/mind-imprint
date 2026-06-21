import { useState } from "react";
import { useSyncExternalStore } from "react";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { Store } from "../store/createStore";
import type { Conversation } from "../agent/createConversation";
import { ChatLog } from "./ChatLog";
import { Composer } from "./Composer";
import { TreePanel } from "./TreePanel";
import { CardSheetHost } from "./CardSheetHost";
import { messagesToItems } from "./viewModel";
import { deriveProcessTree } from "./processTree";

type Props = {
  store: Store;
  conversation: Conversation;
  taskId: string;
  onBack: () => void;
};

export function WorkspaceView({ store, conversation, taskId, onBack }: Props) {
  const [treeOpen, setTreeOpen] = useState(true);

  // Subscribe to store state
  const storeState = useSyncExternalStore(store.subscribe, store.getSnapshot);

  // Subscribe to conversation state
  const convState = useSyncExternalStore(conversation.subscribe, conversation.getSnapshot);

  const task = store.getTask(taskId);
  const phase = convState.phase;
  const pendingCardId = convState.pendingCardId;

  // Completed card count for breadcrumb
  const allCards = storeState.cards.filter((c) => c.task_id === taskId);
  const completedCount = allCards.filter((c) => c.status === "completed").length;

  // Chat items
  const messages = storeState.messages.filter((m) => m.task_id === taskId);
  const chatItems = messagesToItems(
    messages,
    (id) => store.getCard(id),
    (cardId) => CARD_REGISTRY[cardId],
  );

  // Derive process tree from current store snapshot (no state — re-derives on every store update)
  const treeNodes = task
    ? deriveProcessTree({ task, cards: store.listCards(taskId), registry: CARD_REGISTRY })
    : [];

  // Active card for the bottom sheet
  const activeCard =
    phase === "card_active" && pendingCardId
      ? store.getCard(pendingCardId)
      : undefined;
  const activeSpec = activeCard ? CARD_REGISTRY[activeCard.card_id] : undefined;

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%", width: "100%" }}>
      {/* ── Breadcrumb ────────────────────────────────────────────────────── */}
      <div
        style={{
          height: "62px",
          flex: "none",
          background: "#FFFFFF",
          borderBottom: "1px solid #EAECF2",
          display: "flex",
          alignItems: "center",
          padding: "0 26px",
          gap: "18px",
        }}
      >
        <button
          type="button"
          onClick={onBack}
          style={{
            display: "flex",
            alignItems: "center",
            gap: "7px",
            color: "#6B7384",
            fontSize: "13.5px",
            fontWeight: 600,
            cursor: "pointer",
            padding: "7px 12px",
            borderRadius: "9px",
            background: "none",
            border: "none",
            fontFamily: "inherit",
          }}
        >
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2.2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M15 18l-6-6 6-6" />
          </svg>
          返回所有任务
        </button>

        <div style={{ width: "1px", height: "22px", background: "#EAECF2" }} />

        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "10px",
            minWidth: 0,
          }}
        >
          <span
            style={{
              fontSize: "15.5px",
              fontWeight: 700,
              color: "#1C2333",
              whiteSpace: "nowrap",
              overflow: "hidden",
              textOverflow: "ellipsis",
            }}
          >
            {task?.title ?? taskId}
          </span>
          <span
            style={{
              flex: "none",
              fontSize: "11.5px",
              fontWeight: 600,
              color: "#4C9A82",
              background: "#E7F3EE",
              padding: "3px 10px",
              borderRadius: "999px",
            }}
          >
            进行中
          </span>
        </div>

        <div
          style={{
            marginLeft: "auto",
            display: "flex",
            alignItems: "center",
            gap: "8px",
            color: "#9AA1B0",
            fontSize: "12.5px",
            fontWeight: 500,
          }}
        >
          <svg
            width="15"
            height="15"
            viewBox="0 0 24 24"
            fill="none"
            stroke="#9AA1B0"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <rect x="3" y="6" width="18" height="13" rx="2.5" />
          </svg>
          已用 {completedCount} 张工具卡
        </div>
      </div>

      {/* ── Main content area ─────────────────────────────────────────────── */}
      <div style={{ flex: 1, minHeight: 0, display: "flex", position: "relative" }}>
        {/* Chat column */}
        <div
          style={{
            flex: 1,
            minWidth: 0,
            display: "flex",
            flexDirection: "column",
            background: "#F3F4F8",
          }}
        >
          <ChatLog
            items={chatItems}
            onOpenCard={(id) => conversation.openCard(id)}
            onSkipCard={(id) => void conversation.skipCard(id)}
          />

          <Composer
            onSend={(text) => void conversation.send(text)}
            disabled={phase === "awaiting_llm"}
          />
        </div>

        {/* Tree panel */}
        <TreePanel open={treeOpen} onToggle={() => setTreeOpen((v) => !v)} nodes={treeNodes} />

        {/* Bottom sheet — only when card_active */}
        {phase === "card_active" && activeCard && activeSpec && (
          <CardSheetHost
            cardInstance={activeCard}
            spec={activeSpec}
            onSubmit={(id, final) => void conversation.submitCard(id, final)}
            onClose={(id) => void conversation.skipCard(id)}
          />
        )}
      </div>
    </div>
  );
}
