import type { ChatCardOffer, ChatThread, CardInstance } from "@mind-imprint/contracts";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import { Bean } from "../../studio/Bean";
import { StudioCardSheet } from "../../studio/StudioCardSheet";

const FONT = "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif";

// A single turn in the on-screen log. Historical turns (loaded via
// getMessages) never carry an offer — only a turn built live from this
// session's chatTurn stream can (keystone honesty: one CRAAP offer per
// thread, nothing reconstructed after the fact).
export type ChatEntry = {
  id: string;
  role: "user" | "assistant";
  text: string;
  offer?: ChatCardOffer;
  offerPhase?: "offered" | "accepted" | "resolved";
};

export type ChatSurfaceProps = {
  threads: ChatThread[];
  activeThreadId: string | null;
  entries: ChatEntry[];
  draft: string;
  sending: boolean;
  onDraftChange: (v: string) => void;
  onSend: () => void;
  onNewConversation: () => void;
  onSelectThread: (id: string) => void;
  onAcceptOffer: (entryId: string) => void;
  onDismissOffer: (entryId: string) => void;
  onCardSubmit: (entryId: string, env: CardInstance) => void;
  onCardSkip: (entryId: string) => void;
};

// ===== inline icons (dc.html 思维印记_工作区.dc.html lines 527-666 — the
// binding CHAT surface); never lucide-react, matching the rest of the shell. =====

function PlusIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}

function GrowthPillIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="#4C9A82" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M12 20V10M6 20v-5M18 20V6" />
      <path d="M3 20h18" />
    </svg>
  );
}

function AttachIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M21.4 11.05l-9.19 9.19a5 5 0 01-7.07-7.07l9.19-9.19a3 3 0 014.24 4.24l-9.2 9.19a1 1 0 01-1.41-1.41l8.49-8.49" />
    </svg>
  );
}

function ImageIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="3" y="3" width="18" height="18" rx="2" />
      <circle cx="8.5" cy="8.5" r="1.5" />
      <path d="M21 15l-5-5L5 21" />
    </svg>
  );
}

function MicIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="9" y="2" width="6" height="12" rx="3" />
      <path d="M5 10a7 7 0 0014 0M12 17v4" />
    </svg>
  );
}

function SendIcon() {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M22 2L11 13M22 2l-7 20-4-9-9-4z" />
    </svg>
  );
}

function QuestionCircleIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#7C6BB5" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="9" />
      <path d="M9.1 9a3 3 0 015.8 1c0 2-3 3-3 3" />
      <path d="M12 17h.01" />
    </svg>
  );
}

// The in-thread card offer — dc.html's `hasCard` preview block (lines
// 589-608: tag/title/desc/steps) plus an explicit 接受/跳过 pair. Product
// rule #2 (不操纵): summon is automatic, but OPENING a card is always the
// student's confirmed choice — so the offer never auto-mounts the runtime.
function CardOfferBlock({
  entryId,
  offer,
  phase,
  onAccept,
  onDismiss,
  onSubmit,
  onSkip,
}: {
  entryId: string;
  offer: ChatCardOffer;
  phase: "offered" | "accepted" | "resolved";
  onAccept: (entryId: string) => void;
  onDismiss: (entryId: string) => void;
  onSubmit: (entryId: string, env: CardInstance) => void;
  onSkip: (entryId: string) => void;
}) {
  const spec = CARD_REGISTRY[offer.cardId];
  if (!spec || phase === "resolved") return null;

  if (phase === "accepted") {
    return (
      <div style={{ marginTop: 10, maxWidth: 560 }}>
        <StudioCardSheet
          spec={spec}
          onSubmit={(env) => onSubmit(entryId, env)}
          onSkip={() => onSkip(entryId)}
        />
      </div>
    );
  }

  return (
    <div style={{ marginTop: 10, maxWidth: 560, background: "#fff", border: "1px solid #E4E8F5", borderRadius: 14, overflow: "hidden", boxShadow: "0 4px 16px rgba(20,30,60,.06)" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "11px 16px", background: "#F0ECF8", borderBottom: "1px solid #EAE4F3" }}>
        <QuestionCircleIcon />
        <span style={{ fontSize: 11.5, fontWeight: 700, color: "#7C6BB5", letterSpacing: ".02em" }}>{spec.category}</span>
      </div>
      <div style={{ padding: "14px 16px 16px" }}>
        <div style={{ fontSize: 14.5, fontWeight: 800, color: "#1C2333" }}>{spec.name}</div>
        <div style={{ fontSize: 12.5, color: "#8A92A3", lineHeight: 1.6, marginTop: 5 }}>{spec.purpose}</div>
        <div style={{ display: "flex", flexDirection: "column", gap: 9, marginTop: 14 }}>
          {spec.steps.map((st, i) => (
            <div key={st.key} style={{ display: "flex", gap: 11, alignItems: "flex-start" }}>
              <span style={{ flex: "none", width: 20, height: 20, borderRadius: "50%", background: "#EDEFF9", color: "#2A3B7A", fontSize: 11, fontWeight: 800, display: "flex", alignItems: "center", justifyContent: "center" }}>
                {i + 1}
              </span>
              <span style={{ fontSize: 13, color: "#2B3346", lineHeight: 1.55 }}>{st.title}</span>
            </div>
          ))}
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 14, marginTop: 16 }}>
          <button
            type="button"
            onClick={() => onAccept(entryId)}
            style={{ background: "#2A3B7A", color: "#fff", border: "none", padding: "9px 16px", borderRadius: 10, fontSize: 12.5, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}
          >
            接受
          </button>
          <button
            type="button"
            onClick={() => onDismiss(entryId)}
            style={{ background: "none", border: "none", color: "#9AA1B0", fontSize: 12, fontWeight: 600, cursor: "pointer", padding: 0, fontFamily: "inherit" }}
          >
            暂不需要
          </button>
        </div>
      </div>
    </div>
  );
}

export function ChatSurface({
  threads,
  activeThreadId,
  entries,
  draft,
  sending,
  onDraftChange,
  onSend,
  onNewConversation,
  onSelectThread,
  onAcceptOffer,
  onDismissOffer,
  onCardSubmit,
  onCardSkip,
}: ChatSurfaceProps) {
  const activeThread = threads.find((t) => t.id === activeThreadId) ?? null;
  const activeTitle = activeThread?.title?.trim() ? activeThread.title : "新对话";
  const canSend = draft.trim().length > 0 && !sending;

  return (
    <div style={{ flex: 1, minHeight: 0, height: "100%", display: "flex", overflow: "hidden", fontFamily: FONT }}>
      {/* history sidebar — dc.html lines 532-549 */}
      <div style={{ width: 288, flex: "none", background: "#fff", borderRight: "1px solid #EAECF2", display: "flex", flexDirection: "column" }}>
        <div style={{ padding: "18px 18px 12px" }}>
          <button
            type="button"
            onClick={onNewConversation}
            style={{ width: "100%", display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 8, background: "#2A3B7A", color: "#fff", border: "none", padding: 11, borderRadius: 12, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}
          >
            <PlusIcon />
            新对话
          </button>
        </div>
        <div style={{ padding: "0 18px 8px", fontSize: 11.5, fontWeight: 700, color: "#9AA1B0", letterSpacing: ".02em" }}>对话历史</div>
        <div style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "0 12px 16px" }}>
          {threads.map((t) => {
            const isActive = t.id === activeThreadId;
            return (
              <div
                key={t.id}
                onClick={() => onSelectThread(t.id)}
                style={{
                  cursor: "pointer",
                  borderRadius: 10,
                  padding: "10px 12px",
                  marginBottom: 2,
                  background: isActive ? "#EDEFF9" : "transparent",
                }}
              >
                <div style={{ fontSize: 13.5, fontWeight: 700, color: isActive ? "#2A3B7A" : "#1C2333", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                  {t.title?.trim() ? t.title : "新对话"}
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* thread column — dc.html lines 551-664 */}
      <div style={{ flex: 1, minWidth: 0, display: "flex", flexDirection: "column", background: "#F3F4F8" }}>
        <div style={{ flex: "none", height: 60, display: "flex", alignItems: "center", gap: 12, padding: "0 26px", borderBottom: "1px solid #EAECF2", background: "#fff" }}>
          <div style={{ width: 32, height: 32, flex: "none" }}>
            <Bean size={32} />
          </div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>{activeTitle}</div>
            <div style={{ fontSize: 11.5, color: "#8A92A3", fontWeight: 600 }}>自由对话 · AI 只提问，不替你下结论</div>
          </div>
          <div
            title="这些对话会成为你成长评估的一部分"
            style={{ flex: "none", display: "inline-flex", alignItems: "center", gap: 6, fontSize: 11.5, fontWeight: 700, color: "#4C9A82", background: "#E7F3EE", padding: "6px 11px", borderRadius: 999 }}
          >
            <GrowthPillIcon />
            计入成长评估
          </div>
        </div>

        <div style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "26px 26px 10px" }}>
          <div style={{ maxWidth: 760, margin: "0 auto" }}>
            {entries.length === 0 ? (
              <div style={{ textAlign: "center", color: "#9AA1B0", fontSize: 13.5, marginTop: 40 }}>
                还没有对话——把你正在想的、卡住的、好奇的说给它听。
              </div>
            ) : (
              entries.map((m) => (
                <div key={m.id} style={{ display: "flex", marginBottom: 18, justifyContent: m.role === "assistant" ? "flex-start" : "flex-end" }}>
                  {m.role === "assistant" && (
                    <div style={{ flex: "none", width: 30, height: 30, marginRight: 10, marginTop: 2 }}>
                      <Bean size={30} />
                    </div>
                  )}
                  <div style={{ display: "flex", flexDirection: "column", alignItems: m.role === "assistant" ? "flex-start" : "flex-end", minWidth: 0, maxWidth: "72%" }}>
                    <div
                      style={
                        m.role === "assistant"
                          ? { background: "#fff", border: "1px solid #E4E7F0", borderRadius: "4px 16px 16px 16px", padding: "12px 16px", fontSize: 14, lineHeight: 1.7, color: "#1C2333" }
                          : { background: "#2A3B7A", color: "#fff", borderRadius: "16px 16px 4px 16px", padding: "12px 16px", fontSize: 14, lineHeight: 1.6 }
                      }
                    >
                      {m.text}
                    </div>
                    {m.offer && m.offerPhase && (
                      <CardOfferBlock
                        entryId={m.id}
                        offer={m.offer}
                        phase={m.offerPhase}
                        onAccept={onAcceptOffer}
                        onDismiss={onDismissOffer}
                        onSubmit={onCardSubmit}
                        onSkip={onCardSkip}
                      />
                    )}
                  </div>
                </div>
              ))
            )}
          </div>
        </div>

        {/* composer — dc.html lines 616-663 */}
        <div style={{ flex: "none", padding: "12px 26px 20px", background: "#F3F4F8" }}>
          <div style={{ maxWidth: 760, margin: "0 auto", background: "#fff", border: "1px solid #E2E5EE", borderRadius: 18, boxShadow: "0 6px 22px rgba(20,30,60,.07)", overflow: "hidden" }}>
            <textarea
              value={draft}
              onChange={(e) => onDraftChange(e.target.value)}
              rows={1}
              placeholder="把你正在想的、卡住的、好奇的，说给它听……"
              disabled={sending}
              style={{ width: "100%", border: "none", outline: "none", resize: "none", fontSize: 14.5, lineHeight: 1.6, color: "#1C2333", background: "transparent", padding: "15px 16px 6px", fontFamily: "inherit" }}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  if (canSend) onSend();
                }
              }}
            />
            <div style={{ display: "flex", alignItems: "center", gap: 6, padding: "6px 12px 12px" }}>
              {/* Multimodal deferred (Slice 11 T7 scope): these render per the
                  binding design but are inert — no upload wiring. */}
              <div title="附加文件" aria-disabled="true" style={{ width: 34, height: 34, borderRadius: 10, display: "flex", alignItems: "center", justifyContent: "center", cursor: "default", color: "#C7CCDA" }}>
                <AttachIcon />
              </div>
              <div title="上传图片" aria-disabled="true" style={{ width: 34, height: 34, borderRadius: 10, display: "flex", alignItems: "center", justifyContent: "center", cursor: "default", color: "#C7CCDA" }}>
                <ImageIcon />
              </div>
              <div title="语音输入" aria-disabled="true" style={{ width: 34, height: 34, borderRadius: 10, display: "flex", alignItems: "center", justifyContent: "center", cursor: "default", color: "#C7CCDA" }}>
                <MicIcon />
              </div>
              <div style={{ flex: 1 }} />
              <button
                type="button"
                aria-label="发送"
                onClick={onSend}
                disabled={!canSend}
                style={{
                  width: 38,
                  height: 38,
                  borderRadius: 11,
                  border: "none",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  background: canSend ? "#2A3B7A" : "#C7CCDA",
                  cursor: canSend ? "pointer" : "not-allowed",
                }}
              >
                <SendIcon />
              </button>
            </div>
          </div>
          <div style={{ maxWidth: 760, margin: "8px auto 0", textAlign: "center", fontSize: 11, color: "#B6BCC9" }}>
            AI 会陪你把想法想深，但不替你得出结论 · 你的对话只属于你
          </div>
        </div>
      </div>
    </div>
  );
}
