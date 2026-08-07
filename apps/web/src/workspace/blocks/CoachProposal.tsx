import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { CardProposalWire } from "../api/workspace";
import { Icon } from "../Icon";

// CoachProposal — S4 · the cross-phase card OFFER (克制 summon rung). The coach
// may suggest opening a thinking-card mid-conversation; this renders the offer
// as a gentle, dismissable chip under the reply — spec §13 "召唤卡 chip: 卡图标
// + 卡名 + 一句 nudge + 「打开 / 暂不」". Opening is the student's tap (铁律:
// triggering is automatic, opening is confirmed) — this component NEVER
// auto-opens on render. 跳过 calls onDismiss; the caller records the decline
// upstream (so the coach stops offering that card — 铁律 · 不操纵).
export function CoachProposal({
  proposal,
  onOpen,
  onDismiss,
}: {
  proposal: CardProposalWire;
  onOpen: (cardId: string) => void;
  onDismiss: () => void;
}) {
  const spec = CARD_REGISTRY[proposal.cardId];
  return (
    <div className="mt-2 rounded-mk-sm border border-mk-accent-200 bg-mk-accent-50 px-3 py-2 text-mk-body text-mk-ink">
      <p className="flex items-center gap-1.5 text-[14px] font-bold text-mk-accent-700">
        <Icon name="spark" size={13} /> {spec?.name ?? proposal.cardId}
      </p>
      <p className="mt-1 leading-relaxed">{proposal.nudgeText}</p>
      {proposal.reason ? <p className="mt-1 text-mk-small text-mk-accent-700">{proposal.reason}</p> : null}
      <div className="mt-2 flex gap-2">
        <button
          type="button"
          onClick={() => onOpen(proposal.cardId)}
          className="rounded-mk-sm bg-mk-accent px-3 py-1 text-mk-small font-medium text-white hover:bg-mk-accent-600"
        >
          打开
        </button>
        <button
          type="button"
          onClick={onDismiss}
          className="rounded-mk-sm px-3 py-1 text-mk-small font-medium text-mk-accent-700 hover:bg-mk-accent-100"
        >
          跳过
        </button>
      </div>
    </div>
  );
}
