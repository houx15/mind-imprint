import { useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { ChevronUp, Check } from "lucide-react";
import type { NoteProposal, ProposalSection, QuestionProposal } from "@mind-imprint/contracts";
import { Icon } from "@/ui/Icon";
import { ChatLog, type ChatMessage } from "./ChatLog";
import { ChatMarkdown } from "./ChatMarkdown";
import { Composer } from "./Composer";
import { withRecap } from "./RecapHint";
import { SubagentHint } from "./SubagentHint";
import { useStudioChat, type StudioChatMsg } from "./StudioChatContext";
import { CardTurnChip } from "../../workspace/blocks/CardTurnChip";
import { CoachProposal } from "../../workspace/blocks/CoachProposal";
// The 开始 button's icon (Task 6, start gate) uses the geometric line-icon
// set the working rooms already draw "spark"/"arrow" from — aliased so it
// doesn't collide with `@/ui/Icon`'s lucide-wrapper `Icon` used above.
import { Icon as GlyphIcon } from "../../workspace/Icon";

/**
 * StudioCoachChat (Task 9b) — the ONE 印记 chat body. Renders the continuous
 * thread (`ChatLog`), the per-turn note/card OFFER chips (`StudioTurnChips`), and
 * the `Composer` wired to the container-owned `sendStudioTurn`. Used as the
 * chat-first panel body (createPortal'd into the AiPanel when 印记 keeps chat
 * primary); the working rooms keep their own richer coach bodies but render the
 * SAME `StudioTurnChips` so an offer looks identical everywhere.
 *
 * Design-system: solid `mk-*` tokens, one Tailwind class per competing property.
 */

export function StudioCoachChat({ recap, header }: { recap?: string | null; header?: ReactNode }) {
  const {
    messages,
    sending,
    sendStudioTurn,
    historyHasMore,
    loadEarlier,
    loadingEarlier,
    started,
    startJourney,
    starting,
  } = useStudioChat();
  const [draft, setDraft] = useState("");
  // An empty thread renders no fake AI line — a brand-new project simply
  // starts with the Composer (no more scripted CHAT_INTRO fallback).
  const displayChat: StudioChatMsg[] = messages;

  // Scroll-position preservation (Task 5): prepending an older page must not
  // jump the viewport. `pendingDelta` captures the scroll container's
  // `scrollHeight` right before `loadEarlier()` kicks off its fetch; once the
  // older messages land and the DOM grows ABOVE the current view, a layout
  // effect (fires before paint) adds the height delta to `scrollTop` so the
  // messages the student was already reading stay in place. ChatLog's own
  // scroll-to-bottom is disabled for this consumer (`stickToBottom={false}`
  // below) — it would otherwise re-jump to the newest turn on every prepend —
  // so this effect ALSO takes over the "stick to the newest turn" behavior
  // for the normal (non-prepend) case: a sent/received turn, or the 「印记正在
  // 打字」indicator appearing.
  const scrollRef = useRef<HTMLDivElement>(null);
  const pendingDelta = useRef<number | null>(null);

  function handleLoadEarlier() {
    const el = scrollRef.current;
    if (el) pendingDelta.current = el.scrollHeight;
    loadEarlier();
  }

  useLayoutEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    if (pendingDelta.current != null) {
      el.scrollTop += el.scrollHeight - pendingDelta.current;
      pendingDelta.current = null;
      return;
    }
    el.scrollTop = el.scrollHeight;
  }, [messages, sending]);

  async function onSend() {
    const text = draft.trim();
    if (!text || sending) return;
    setDraft("");
    await sendStudioTurn(text);
  }

  return (
    <div className="flex h-full flex-col gap-3 p-4">
      {header}
      <div ref={scrollRef} className="flex min-h-0 flex-1 flex-col gap-3.5 overflow-y-auto pr-1">
        {historyHasMore && (
          <button
            type="button"
            onClick={handleLoadEarlier}
            disabled={loadingEarlier}
            className="mx-auto flex shrink-0 items-center gap-1.5 rounded-full border border-mk-border bg-mk-surface px-3.5 py-1.5 text-mk-body font-semibold text-mk-muted transition hover:text-mk-accent focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-mk-accent disabled:cursor-not-allowed disabled:opacity-60"
          >
            <Icon icon={ChevronUp} size={14} />
            {loadingEarlier ? "载入中…" : "载入更早的对话"}
          </button>
        )}
        <ChatLog messages={withRecap(recap, toChatMessages(displayChat))} thinking={sending} stickToBottom={false} />
        <StudioTurnChips />
      </div>
      {started ? (
        <Composer
          value={draft}
          onChange={setDraft}
          onSend={onSend}
          state={sending ? "replying" : undefined}
          placeholder="和印记说说你的项目……（Shift+Enter 换行）"
          className="flex-none"
        />
      ) : (
        // Task 6 (start gate): before the student taps 开始, there is no
        // composer at all — 印记's opening turn ends 准备好开始了吗 and this
        // is the one real, clearly-visible action that answers it (never a
        // 12px hint chip). Clicking calls `coach/start`, which flips
        // `started` → true and opens 提案 — the container re-renders the
        // switcher/tabs in.
        <button
          type="button"
          onClick={() => void startJourney()}
          disabled={starting}
          className="flex flex-none items-center justify-center gap-2 rounded-mk-lg bg-mk-accent px-5 py-3 text-mk-body font-bold text-white transition hover:bg-mk-accent-600 focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-mk-accent disabled:cursor-not-allowed disabled:opacity-60"
        >
          <GlyphIcon name={starting ? "spark" : "arrow"} size={16} />
          {starting ? "准备中…" : "开始"}
        </button>
      )}
    </div>
  );
}

/**
 * StudioTurnChips — the per-turn note + card OFFERS from the container store
 * (铁律②: 印记 proposes, the student confirms/opens). Rendered inside EVERY
 * coach body (chat-first + both rooms) so exactly one implementation drives the
 * chips wherever the thread is shown. Nothing renders while a turn is in flight.
 */
export function StudioTurnChips() {
  const {
    pendingNote,
    confirmedNote,
    confirmNote,
    dismissNote,
    pendingCard,
    openCard,
    dismissCard,
    pendingQuestion,
    confirmQuestion,
    dismissQuestion,
    sending,
  } = useStudioChat();
  if (sending) return null;
  return (
    <>
      {pendingNote && <NoteConfirmChip note={pendingNote} onConfirm={confirmNote} onDismiss={dismissNote} />}
      {/* After confirm, the actionable chip becomes a quiet 记下了 acknowledgment
          (not a disappearance). Only when no fresh offer is pending. */}
      {!pendingNote && confirmedNote && <NoteRecordedChip note={confirmedNote} />}
      {pendingCard && (
        <CoachProposal proposal={pendingCard} onOpen={openCard} onDismiss={() => dismissCard(pendingCard.cardId)} />
      )}
      {pendingQuestion && (
        <QuestionConfirmChip question={pendingQuestion} onConfirm={confirmQuestion} onDismiss={dismissQuestion} />
      )}
    </>
  );
}

const SECTION_LABEL: Record<ProposalSection, string> = {
  objective: "目标",
  reason: "缘由",
  activities: "活动",
  resources: "资源",
  counterpoints: "可能的反例 / 张力",
};

// The 克制 confirm chip: 印记 offers to record a faithful one-line summary of
// the student's own words into a proposal section. Nothing is written until she
// taps 记进 (打开由学生确认). Mirrors PlanBlock's former DimConfirmChip.
function NoteConfirmChip({ note, onConfirm, onDismiss }: { note: NoteProposal; onConfirm: () => void; onDismiss: () => void }) {
  const label = SECTION_LABEL[note.section];
  return (
    <div className="rounded-mk-lg border border-mk-success bg-mk-success-bg px-3.5 py-3 text-[14px] text-mk-ink">
      <p className="font-semibold leading-snug text-mk-success">要不要把这点记进「{label}」？</p>
      <p className="mt-1 text-[14px] leading-relaxed text-mk-muted">{note.value}</p>
      <div className="mt-2.5 flex items-center gap-2">
        <button type="button" onClick={onConfirm} className="rounded-full bg-mk-success px-3.5 py-1.5 text-[12px] font-bold text-white transition hover:opacity-90">
          记进「{label}」
        </button>
        <button type="button" onClick={onDismiss} className="rounded-full px-2.5 py-1.5 text-[12px] font-semibold text-mk-faint hover:text-mk-muted">
          跳过
        </button>
      </div>
    </div>
  );
}

// The post-confirm acknowledgment: 记进 was tapped, the note is now in the
// proposal board. Replaces NoteConfirmChip's actionable form with a quiet, no-
// button "记下了" state so the student's tap has lasting confirmation instead of
// vanishing. A SOLID border token (design gotcha: `border-mk-<token>/<NN>`
// renders transparent).
function NoteRecordedChip({ note }: { note: NoteProposal }) {
  const label = SECTION_LABEL[note.section];
  return (
    <div className="rounded-mk-lg border border-mk-success bg-mk-success-bg px-3.5 py-2.5 text-[14px] text-mk-ink">
      <p className="flex items-center gap-1.5 font-semibold leading-snug text-mk-success">
        <Icon icon={Check} size={14} />
        已记进「{label}」
      </p>
      <p className="mt-1 text-[14px] leading-relaxed text-mk-muted">{note.value}</p>
    </div>
  );
}

// Task 7 (P2b) · the 克制 confirm chip for 印记's `propose_question` OFFER:
// mirrors NoteConfirmChip's markup but confirming creates an exploration lead
// instead of writing a proposal section. DESIGN GOTCHA: a SOLID border token
// (`border-mk-accent`) — never `border-mk-<token>/<NN>` (renders transparent).
function QuestionConfirmChip({
  question,
  onConfirm,
  onDismiss,
}: {
  question: QuestionProposal;
  onConfirm: () => void;
  onDismiss: () => void;
}) {
  return (
    <div className="rounded-mk-lg border border-mk-accent bg-mk-accent-50 px-3.5 py-3 text-[14px] text-mk-ink">
      <p className="font-semibold leading-snug text-mk-accent-700">要不要把这个问题加进探索图谱？</p>
      <p className="mt-1 text-[14px] leading-relaxed text-mk-muted">{question.text}</p>
      <div className="mt-2.5 flex items-center gap-2">
        <button type="button" onClick={onConfirm} className="rounded-full bg-mk-accent px-3.5 py-1.5 text-[12px] font-bold text-white transition hover:opacity-90">
          加入探索图谱
        </button>
        <button type="button" onClick={onDismiss} className="rounded-full px-2.5 py-1.5 text-[12px] font-semibold text-mk-faint hover:text-mk-muted">
          跳过
        </button>
      </div>
    </div>
  );
}

// StudioChatMsg[] → the shared ChatLog's ChatMessage[]. A card-turn (msg.card)
// maps to a system-role node (no bubble chrome) so CardTurnChip is the whole
// message; a hint (msg.hint, Task 7) likewise maps to a system-role node
// rendering a `SubagentHint` (done — these are post-hoc acknowledgments, not a
// live spinner); a quotedPart (就这一段) rides as a callout above the text.
// Mirrors WritingBlock's mapper (the superset that handles both card + quotedPart).
export function toChatMessages(chat: StudioChatMsg[]): ChatMessage[] {
  return chat.map((m, i) =>
    m.card
      ? { id: String(i), role: "system", node: <CardTurnChip card={m.card} /> }
      : m.hint
        ? { id: String(i), role: "system", node: <SubagentHint text={m.hint} done /> }
        : {
            id: String(i),
            role: m.role === "ai" ? "assistant" : "student",
            node: (
              <>
                {m.quotedPart && (
                  <blockquote className="mb-1 rounded-mk-sm border-l-[3px] border-mk-peach bg-mk-peach-bg px-2.5 py-1.5 text-[12px] italic leading-snug text-mk-muted">
                    {m.quotedPart}
                  </blockquote>
                )}
                {m.role === "ai" ? (
                  <ChatMarkdown text={m.text} />
                ) : (
                  <span className="whitespace-pre-wrap">{m.text}</span>
                )}
              </>
            ),
          },
  );
}
