import { useState, type ReactNode } from "react";
import type { NoteProposal, ProposalSection } from "@mind-imprint/contracts";
import { ChatLog, type ChatMessage } from "./ChatLog";
import { Composer } from "./Composer";
import { withRecap } from "./RecapHint";
import { useStudioChat, type StudioChatMsg } from "./StudioChatContext";
import { CardTurnChip } from "../../workspace/blocks/CardTurnChip";
import { CoachProposal } from "../../workspace/blocks/CoachProposal";

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

// The scripted opening line for a brand-new project's chat-first panel — a
// display-only fallback (never stored), shown until the first real turn lands.
const CHAT_INTRO =
  "把你手上的真实任务丢给我——一个题目、一段困惑、一篇要读的文章都行。我们一起把它想清楚，我会在对的时候为你打开对的工作台。";

export function StudioCoachChat({ recap, header }: { recap?: string | null; header?: ReactNode }) {
  const { messages, sending, sendStudioTurn } = useStudioChat();
  const [draft, setDraft] = useState("");
  // Display-only intro fallback: an empty hoisted store falls back to the
  // scripted line; once any turn lands the store is non-empty and IT shows.
  const displayChat: StudioChatMsg[] = messages.length ? messages : [{ role: "ai", text: CHAT_INTRO }];

  async function onSend() {
    const text = draft.trim();
    if (!text || sending) return;
    setDraft("");
    await sendStudioTurn(text);
  }

  return (
    <div className="flex h-full flex-col gap-3 p-4">
      {header}
      <div className="flex min-h-0 flex-1 flex-col gap-3.5 overflow-y-auto pr-1">
        <ChatLog messages={withRecap(recap, toChatMessages(displayChat))} thinking={sending} />
        <StudioTurnChips />
      </div>
      <Composer
        value={draft}
        onChange={setDraft}
        onSend={onSend}
        state={sending ? "replying" : undefined}
        placeholder="和印记说说你的项目……（Shift+Enter 换行）"
        className="flex-none"
      />
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
  const { pendingNote, confirmNote, dismissNote, pendingCard, openCard, dismissCard, sending } = useStudioChat();
  if (sending) return null;
  return (
    <>
      {pendingNote && <NoteConfirmChip note={pendingNote} onConfirm={confirmNote} onDismiss={dismissNote} />}
      {pendingCard && (
        <CoachProposal proposal={pendingCard} onOpen={openCard} onDismiss={() => dismissCard(pendingCard.cardId)} />
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
    <div className="rounded-mk-lg border border-mk-success bg-mk-success-bg px-3.5 py-3 text-[13px] text-mk-ink">
      <p className="font-semibold leading-snug text-mk-success">要不要把这点记进「{label}」？</p>
      <p className="mt-1 text-[12.5px] leading-relaxed text-mk-muted">{note.value}</p>
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

// Render **bold** spans inline; everything else is plain text (mirrors the
// rooms' own renderRich so bold markers survive into the shared ChatLog).
function renderRich(text: string) {
  return text.split(/(\*\*[^*]+\*\*)/g).map((part, i) =>
    part.startsWith("**") && part.endsWith("**") ? (
      <strong key={i} className="font-bold">{part.slice(2, -2)}</strong>
    ) : (
      <span key={i}>{part}</span>
    ),
  );
}

// StudioChatMsg[] → the shared ChatLog's ChatMessage[]. A card-turn (msg.card)
// maps to a system-role node (no bubble chrome) so CardTurnChip is the whole
// message; a quotedPart (就这一段) rides as a callout above the text. Mirrors
// WritingBlock's mapper (the superset that handles both card + quotedPart).
export function toChatMessages(chat: StudioChatMsg[]): ChatMessage[] {
  return chat.map((m, i) =>
    m.card
      ? { id: String(i), role: "system", node: <CardTurnChip card={m.card} /> }
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
              <span className="whitespace-pre-wrap">{renderRich(m.text)}</span>
            </>
          ),
        },
  );
}
