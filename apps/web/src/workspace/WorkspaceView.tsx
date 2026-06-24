import { useState } from "react";
import { useSyncExternalStore } from "react";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { Store } from "../store/createStore";
import type { Conversation } from "../agent/createConversation";
import type { Evaluator } from "../agent/createEvaluator";
import { useEvaluator } from "../agent/useEvaluator";
import { ChatLog } from "./ChatLog";
import { Composer } from "./Composer";
import { TreePanel } from "./TreePanel";
import { CardSheetHost } from "./CardSheetHost";
import { EvalLoading } from "./EvalLoading";
import { EvalModal } from "./EvalModal";
import { messagesToItems } from "./viewModel";
import { deriveProcessTree } from "./processTree";

type Props = {
  store: Store;
  conversation: Conversation;
  taskId: string;
  onBack: () => void;
  evaluator?: Evaluator;
};

// A no-op evaluator used when no evaluator prop is provided (e.g. in older tests)
const NOOP_EVAL_STATE: import("../agent/createEvaluator").EvalState = { phase: "idle" };
const NOOP_EVALUATOR: Evaluator = {
  getSnapshot: () => NOOP_EVAL_STATE,
  subscribe: () => () => {},
  run: async () => {},
};

export function WorkspaceView({ store, conversation, taskId, onBack, evaluator = NOOP_EVALUATOR }: Props) {
  const [treeOpen, setTreeOpen] = useState(true);
  const [showEvalModal, setShowEvalModal] = useState(false);

  // Subscribe to store state
  const storeState = useSyncExternalStore(store.subscribe, store.getSnapshot);

  // Subscribe to conversation state
  const convState = useSyncExternalStore(conversation.subscribe, conversation.getSnapshot);

  // Subscribe to evaluator state
  const evalState = useEvaluator(evaluator);

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
            gap: "12px",
          }}
        >
          <div
            style={{
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

          <button
            type="button"
            disabled={evalState.phase === "running"}
            onClick={() => {
              const hasEval = !!store.getLatestEvaluation(taskId);
              if (hasEval && !window.confirm("重新评估会覆盖上一次的思维印记，确定吗？")) {
                return;
              }
              setShowEvalModal(true);
              void evaluator.run();
            }}
            style={{
              fontSize: "13px",
              fontWeight: 700,
              color: evalState.phase === "running" ? "#9AA1B0" : "#2A3B7A",
              background: evalState.phase === "running" ? "#F0F1F5" : "#EBF0FF",
              border: "none",
              padding: "7px 14px",
              borderRadius: "10px",
              cursor: evalState.phase === "running" ? "not-allowed" : "pointer",
              fontFamily: "inherit",
            }}
          >
            {store.getLatestEvaluation(taskId) ? "重新评估" : "生成思维印记"}
          </button>
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
            thinking={phase === "awaiting_llm"}
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
            onClose={(id) => conversation.closeCard(id)}
            onSkip={(id) => void conversation.skipCard(id)}
          />
        )}

        {/* Eval loading overlay */}
        {evalState.phase === "running" && <EvalLoading />}

        {/* Eval error card */}
        {evalState.phase === "error" && (
          <div
            style={{
              position: "absolute",
              bottom: "80px",
              left: "50%",
              transform: "translateX(-50%)",
              zIndex: 40,
              background: "#FFF3F3",
              border: "1px solid #F8C8C8",
              borderRadius: "12px",
              padding: "16px 20px",
              display: "flex",
              alignItems: "center",
              gap: "12px",
              boxShadow: "0 4px 16px rgba(200,60,60,.12)",
              minWidth: "280px",
            }}
          >
            <div style={{ flex: 1 }}>
              <div style={{ fontWeight: 700, color: "#C0392B", fontSize: "14px" }}>评估失败</div>
              {evalState.error && (
                <div style={{ fontSize: "13px", color: "#8A4040", marginTop: "4px" }}>
                  {evalState.error}
                </div>
              )}
            </div>
            <button
              type="button"
              onClick={() => void evaluator.run()}
              style={{
                background: "#C0392B",
                color: "#fff",
                border: "none",
                padding: "7px 14px",
                borderRadius: "8px",
                fontSize: "13px",
                fontWeight: 700,
                cursor: "pointer",
                fontFamily: "inherit",
                flexShrink: 0,
              }}
            >
              重试
            </button>
          </div>
        )}

        {/* Eval modal — shown when done and not dismissed */}
        {evalState.phase === "done" && evalState.evaluation && showEvalModal && (
          <EvalModal
            evaluation={evalState.evaluation}
            onClose={() => setShowEvalModal(false)}
          />
        )}
      </div>
    </div>
  );
}
