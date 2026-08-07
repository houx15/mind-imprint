import { useState } from "react";
import { CARD_REGISTRY, type CardTurnRef } from "@mind-imprint/contracts";
import { persistProjectCard, reflectProjectCard, dismissProposal, type CardProposalWire } from "../api/workspace";
import { compileCardForCoach } from "../../studio/compileCard";
import { CoachProposal } from "./CoachProposal";
import { StudioCardSheet } from "../../studio/StudioCardSheet";
import { Icon } from "../Icon";
import { coverGradient, type MacaronName } from "@/ui";

// Each shelf chip is tinted with one of the 7 macarons (spec §10: "工具卡 ·
// 描边级 radius 8，取一支马卡龙浅底"), deterministically by card id via the
// same `coverGradient` hash used elsewhere (card gallery covers) — so a given
// card always wears the same macaron across the product, no per-card color
// authored by hand.
const MACARON_CHIP: Record<MacaronName, string> = {
  peach: "bg-mk-peach-bg text-mk-peach-fg",
  butter: "bg-mk-butter-bg text-mk-butter-fg",
  matcha: "bg-mk-matcha-bg text-mk-matcha-fg",
  lake: "bg-mk-lake-bg text-mk-lake-fg",
  mist: "bg-mk-mist-bg text-mk-mist-fg",
  taro: "bg-mk-taro-bg text-mk-taro-fg",
  berry: "bg-mk-berry-bg text-mk-berry-fg",
};

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
  // handed back so the parent shows BOTH in the thread. `card` (present unless
  // the card was empty) lets the parent render the student turn as a content-
  // first clickable chip instead of raw compiled text.
  onReflected?: (studentText: string, reply: string, card?: CardTurnRef) => void;
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
        const res = await reflectProjectCard(projectId, id, fieldValues, eventTrace, surface);
        // The server echoes the persisted card ref (null on an empty-card no-op);
        // pass it so the parent renders a content-first chip, not raw text.
        onReflected(studentText, res.reply, res.card ?? undefined);
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
        <div className="rounded-mk-sm border border-mk-border bg-mk-paper p-2">
          <p className="mb-1.5 flex items-center gap-1 px-0.5 text-[12px] font-bold text-mk-faint">
            <Icon name="spark" size={12} /> 工具卡 · 挑一张想清楚（你填，印记不替你写）
          </p>
          <div className="flex flex-col gap-1.5">
            {availableDeck.map((id) => {
              const spec = CARD_REGISTRY[id]!;
              return (
                <button
                  key={id}
                  type="button"
                  onClick={() => openCard(id)}
                  title={spec.purpose}
                  aria-label={spec.name}
                  className={`flex w-full flex-col items-start gap-0.5 overflow-hidden rounded-mk-sm px-2.5 py-1.5 text-left transition-colors duration-[120ms] ease-mk hover:brightness-95 ${MACARON_CHIP[coverGradient(id).macaron]}`}
                >
                  <span aria-hidden="true" className="flex w-full min-w-0 items-center gap-1 text-[12px] font-bold">
                    <span className="shrink-0"><Icon name="spark" size={11} /></span> <span className="truncate">{spec.name}</span>
                  </span>
                  <span aria-hidden="true" className="w-full truncate text-[12px] font-medium opacity-80">
                    {spec.purpose}
                  </span>
                </button>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
