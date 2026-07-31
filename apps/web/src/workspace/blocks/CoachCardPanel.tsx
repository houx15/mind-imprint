import { useState } from "react";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import { persistProjectCard, dismissProposal, type CardProposalWire } from "../api/workspace";
import { CoachProposal } from "./CoachProposal";
import { StudioCardSheet } from "../../studio/StudioCardSheet";
import { Icon } from "../Icon";

// #18: the tool-card surface shared by forming (立题) + the library (找资料),
// mirroring the writing room's rail. Two paths, both 打开由学生确认:
//   - AI-proposed: `proposal` (from the coach turn) renders a dismissable chip;
//   - self-summon: a 工具卡 picker lets the student open a deck card herself.
// Opening → StudioCardSheet → persistProjectCard (过程即数据; 印记 never fills it).
// The default deck is the three general thinking cards the server allows the
// cross-phase proposer to persist (card_persist.go coachProposableCards).
export const THINKING_DECK = ["fact-opinion-value", "certainty-spectrum", "steelman"];

export function CoachCardPanel({
  projectId,
  proposal,
  onProposalConsumed,
  onLogged,
  deck = THINKING_DECK,
}: {
  projectId: string;
  proposal: CardProposalWire | null;
  onProposalConsumed: () => void;
  onLogged?: (text: string) => void;
  deck?: string[];
}) {
  const [openCardId, setOpenCardId] = useState<string | null>(null);
  const [deckOpen, setDeckOpen] = useState(false);

  function openCard(id: string) {
    onProposalConsumed();
    setDeckOpen(false);
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
    try {
      await persistProjectCard(projectId, id, fieldValues, eventTrace);
      onLogged?.("记下了——你刚才的思考已经存进过程里。");
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
      {!openCardId && availableDeck.length > 0 && (
        <div>
          <button
            type="button"
            onClick={() => setDeckOpen((o) => !o)}
            className="inline-flex items-center gap-1 rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-semibold text-mk-muted-2 hover:border-mk-primary hover:text-mk-primary"
          >
            <Icon name="spark" size={12} /> 工具卡
          </button>
          {deckOpen && (
            <div className="mt-1.5 flex flex-wrap gap-1.5">
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
          )}
        </div>
      )}
    </div>
  );
}
