import type { CardProposalWire } from "../api/workspace";

// CoachProposal — S4 · the cross-phase card OFFER (克制 summon rung). The coach
// may suggest opening a thinking-card mid-conversation; this renders the offer
// as a gentle, dismissable chip under the reply. Opening is the student's tap
// (铁律: triggering is automatic, opening is confirmed) — this component NEVER
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
  return (
    <div className="mt-2 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900">
      <p className="leading-relaxed">{proposal.nudgeText}</p>
      {proposal.reason ? <p className="mt-1 text-xs text-amber-700">{proposal.reason}</p> : null}
      <div className="mt-2 flex gap-2">
        <button
          type="button"
          onClick={() => onOpen(proposal.cardId)}
          className="rounded-lg bg-amber-600 px-3 py-1 text-xs font-medium text-white hover:bg-amber-700"
        >
          打开
        </button>
        <button
          type="button"
          onClick={onDismiss}
          className="rounded-lg px-3 py-1 text-xs font-medium text-amber-700 hover:bg-amber-100"
        >
          跳过
        </button>
      </div>
    </div>
  );
}
