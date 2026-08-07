import type { CardSpec } from "@mind-imprint/contracts";

export function CompletedCard({ card, filledCount }: { card: CardSpec; filledCount: number }) {
  return (
    <div className="rounded-mk border border-mk-border bg-white p-4">
      <div className="flex items-center gap-2.5">
        <span className="rounded-full bg-mk-primary-tint px-2.5 py-0.5 text-[12px] font-semibold text-mk-primary">{card.category}</span>
        <span className="inline-flex items-center gap-1 rounded-[9px] bg-mk-green-tint px-2.5 py-1 text-[12px] font-semibold text-mk-green">✓ 已完成</span>
      </div>
      <div className="mt-2 text-sm font-bold text-mk-ink">{card.name}</div>
      <div className="mt-1 text-[12px] text-mk-muted-2">已填 {filledCount} 项</div>
    </div>
  );
}
