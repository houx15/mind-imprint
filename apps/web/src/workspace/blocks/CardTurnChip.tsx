import { useState } from "react";
import { CARD_REGISTRY, type CardTurnRef } from "@mind-imprint/contracts";
import { cardFieldEntries } from "../../studio/compileCard";
import { CardRenderer } from "../../cards/CardRenderer";
import { Icon } from "../Icon";

// A completed tool card, rendered in the coach thread as a compact, clickable
// student-side chip. Content-first (per the refinement): it LEADS with the
// student's own answer — the first filled field's text, truncated — and shows
// the card name only as a secondary provenance label. Tapping it opens a
// READ-ONLY view of everything she filled (never editable — it's the record,
// 铁律). Shared by all three rooms (forming / writing / review) so live turns
// and reloaded history render identically.
export function CardTurnChip({ card }: { card: CardTurnRef }) {
  const [open, setOpen] = useState(false);
  const spec = CARD_REGISTRY[card.cardId];
  const entries = spec ? cardFieldEntries(spec, card.fieldValues) : [];
  const name = spec?.name ?? card.cardId;
  // Content-first: the student's own words lead the chip; card name is secondary.
  const preview = entries[0]?.value ?? "";
  const previewText = preview.length > 64 ? `${preview.slice(0, 64)}…` : preview;

  return (
    <div className="flex justify-end">
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="max-w-[88%] cursor-pointer rounded-mk-lg border border-white/30 bg-mk-accent px-3.5 py-2.5 text-left text-white transition hover:brightness-105"
        title="点开看你填的这张卡"
      >
        <span className="mb-1 flex items-center gap-1.5 text-[10.5px] font-bold uppercase tracking-wide text-white/75">
          <Icon name="spark" size={11} /> 工具卡 · {name}
        </span>
        {previewText ? (
          <span className="block whitespace-pre-wrap text-[13px] font-medium leading-relaxed">{previewText}</span>
        ) : (
          <span className="block text-[13px] italic leading-relaxed text-white/85">（这张卡还没填内容）</span>
        )}
        <span className="mt-1.5 block text-[11px] font-semibold text-white/80">点开看你填的 →</span>
      </button>
      {open && spec && <CardTurnViewer card={card} onClose={() => setOpen(false)} />}
    </div>
  );
}

// The read-only card record modal. Reuses CardRenderer's read-only mode (the
// SAME step/section walk as the live card) so a filled card reads consistently
// whether being filled or reviewed. No inputs, no submit/skip — neither the
// student nor the AI can edit the answers here (铁律: it's a record).
function CardTurnViewer({ card, onClose }: { card: CardTurnRef; onClose: () => void }) {
  const spec = CARD_REGISTRY[card.cardId]!;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-mk-ink/30 px-6" role="dialog" aria-modal="true" onClick={onClose}>
      <div className="flex max-h-[86%] w-[560px] flex-col overflow-hidden rounded-mk-lg border border-mk-border bg-mk-surface shadow-[0_20px_60px_rgba(28,35,51,0.25)]" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-start justify-between border-b border-mk-border px-6 py-4">
          <div>
            <div className="flex items-center gap-2">
              <span className="text-[10.5px] font-bold uppercase tracking-wide text-mk-accent">工具卡 · 你填的记录</span>
              <span className="rounded-mk-full bg-mk-accent-50 px-2 py-0.5 text-[10.5px] font-bold text-mk-accent">{spec.category}</span>
            </div>
            <h3 className="mt-1 font-sans text-[17px] font-bold text-mk-ink">{spec.name}</h3>
            <p className="mt-0.5 text-[11.5px] text-mk-faint">这是你当时填的内容——只读，改不动（这是你的思维记录）。</p>
          </div>
          <button type="button" onClick={onClose} aria-label="关闭" className="text-[20px] leading-none text-mk-faint hover:text-mk-ink">×</button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto bg-mk-paper/40 px-6 py-5">
          <CardRenderer card={spec} values={card.fieldValues} onField={() => {}} onExpandStep={() => {}} readOnly />
        </div>
        <div className="flex justify-end border-t border-mk-border px-6 py-3.5">
          <button type="button" onClick={onClose} className="rounded-mk-sm bg-mk-accent px-4 py-2 text-[13px] font-bold text-white hover:bg-mk-accent-600">
            知道了
          </button>
        </div>
      </div>
    </div>
  );
}
