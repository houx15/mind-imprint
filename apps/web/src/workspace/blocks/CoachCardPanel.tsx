import { useState } from "react";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import { persistProjectCard, reflectProjectCard, dismissProposal, type CardProposalWire } from "../api/workspace";
import { compileCardForCoach } from "../../studio/compileCard";
import { CoachProposal } from "./CoachProposal";
import { StudioCardSheet } from "../../studio/StudioCardSheet";
import { Icon } from "../Icon";

// #18: the tool-card surface shared by forming (立题) + the library (找资料),
// mirroring the writing room's rail. Two paths, both 打开由学生确认:
//   - AI-proposed: `proposal` (from the coach turn) renders a dismissable chip;
//   - self-summon: a 工具卡 shelf lets the student open a deck card herself.
// Opening → StudioCardSheet → persistProjectCard (过程即数据; 印记 never fills it).
//
// Phase-scoped decks (#17/#18): each surface offers only the cards that help
// THAT phase. 立题 sharpens the question and plans the search; 找资料/读 dissects a
// source and weighs perspectives; the exploration graph owns the one
// resource-finding card (rabbit-hole, elsewhere). The writing room has its own
// deck in WritingBlock. The server mirrors every summonable id in
// card_persist.go's persistable allowlist.
export const FORMING_DECK = ["question-card", "perspective-matrix", "search-plan"];
export const READING_DECK = ["fact-opinion-value", "perspective-matrix"];
// The three general thinking cards the cross-phase proposer may offer
// (card_persist.go coachProposableCards) — kept as the default when no
// phase deck is passed.
export const THINKING_DECK = ["fact-opinion-value", "concession"];

export function CoachCardPanel({
  projectId,
  proposal,
  onProposalConsumed,
  onLogged,
  onReflected,
  surface,
  deck = THINKING_DECK,
}: {
  projectId: string;
  proposal: CardProposalWire | null;
  onProposalConsumed: () => void;
  // Legacy path (no coach turn): the card persists and a single acknowledgement
  // string is surfaced. Used where a coach reflect isn't wired yet.
  onLogged?: (text: string) => void;
  // Slice 2 reflect path: when `surface` is set, submit runs a coach turn that
  // responds to the card's content; the compiled student turn + the AI reply are
  // handed back so the parent shows BOTH in the thread.
  onReflected?: (studentText: string, reply: string) => void;
  surface?: string;
  deck?: string[];
}) {
  const [openCardId, setOpenCardId] = useState<string | null>(null);

  function openCard(id: string) {
    onProposalConsumed();
    setOpenCardId(id);
  }
  function dismiss() {
    const id = proposal?.cardId;
    onProposalConsumed();
    if (id) void dismissProposal(projectId, id).catch(() => {});
  }
  async function submit(fieldValues: Record<string, unknown>, eventTrace: unknown[]) {
    const id = openCardId;
    setOpenCardId(null);
    if (!id) return;
    const spec = CARD_REGISTRY[id];

    // Slice 2 · reflect path: the coach responds to what the student wrote.
    if (surface && onReflected) {
      const studentText = spec ? compileCardForCoach(spec, fieldValues) : "";
      try {
        const { reply } = await reflectProjectCard(projectId, id, fieldValues, eventTrace, surface);
        onReflected(studentText, reply);
      } catch {
        // Soft inline note — never crash the room; the student's words still show.
        onReflected(studentText, "刚才没接住这张卡，等下再试一次。");
      }
      return;
    }

    // Legacy path: persist only, with a canned acknowledgement.
    try {
      await persistProjectCard(projectId, id, fieldValues, eventTrace);
      // #9 · name the card so the used card leaves a visible trace in the thread.
      const name = spec?.name;
      onLogged?.(name ? `记下了——你在《${name}》里的思考已存进过程。` : "记下了——你刚才的思考已经存进过程里。");
    } catch {
      onLogged?.("刚才没存上，等下再试一次。");
    }
  }

  const availableDeck = deck.filter((id) => CARD_REGISTRY[id]);

  return (
    <div className="flex flex-col gap-2">
      {proposal && !openCardId && (
        <CoachProposal proposal={proposal} onOpen={openCard} onDismiss={dismiss} />
      )}
      {openCardId && CARD_REGISTRY[openCardId] && (
        <StudioCardSheet
          spec={CARD_REGISTRY[openCardId]!}
          onSubmit={(env) => submit(env.field_values, env.event_trace)}
          onSkip={() => setOpenCardId(null)}
        />
      )}
      {/* #8 · the card shelf is always visible (was hidden behind a 工具卡 toggle
          that students ignored). 触发自动、打开由学生确认 — showing the deck isn't
          opening a card; tapping one is her choice. */}
      {!openCardId && availableDeck.length > 0 && (
        <div className="rounded-mk border border-mk-border-2 bg-mk-bg/50 p-2">
          <p className="mb-1.5 flex items-center gap-1 px-0.5 text-[11px] font-bold text-mk-muted-2">
            <Icon name="spark" size={12} /> 工具卡 · 挑一张想清楚（你填，印记不替你写）
          </p>
          <div className="flex flex-wrap gap-1.5">
            {availableDeck.map((id) => (
              <button
                key={id}
                type="button"
                onClick={() => openCard(id)}
                title={CARD_REGISTRY[id]!.purpose}
                className="rounded-mk border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-semibold text-mk-ink hover:border-mk-primary hover:text-mk-primary"
              >
                {CARD_REGISTRY[id]!.name}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
