import { useEffect, useRef } from "react";
import { ReviewingHint } from "@/ui";

// GuidedWritingCard — slice 4b · the ONE shared, configurable guided-writing card
// (§4/§6). Both the proposal parts (3a, retrofit) and the essay claims (4b) use
// it, injected with different content: a clear guidance line + an optional
// English example + a multi-line textarea that AUTO-GROWS as the student writes +
// 我依然有问题 / 我写好了 + an optional 写作卡 offer. It never writes the body
// (铁律①) — the student writes in the textarea; 我写好了 asks for 批注.

export function GuidedWritingCard({
  title,
  guidance,
  example,
  value,
  onChange,
  onStillStuck,
  onDone,
  doneLabel = "我写好了",
  reviewing = false,
  locked = false,
  placeholder = "在这里写……",
  cardOffer,
  onOfferCard,
  footer,
}: {
  title?: string;
  guidance: string;
  example?: string | null;
  value: string;
  onChange: (v: string) => void;
  onStillStuck: () => void;
  onDone: () => void;
  doneLabel?: string;
  reviewing?: boolean;
  locked?: boolean;
  placeholder?: string;
  // Optional 写作卡 offer (essay claims: e.g. ["pee","toulmin","argument-map"]).
  cardOffer?: { id: string; label: string }[];
  onOfferCard?: (cardId: string) => void;
  footer?: React.ReactNode;
}) {
  const taRef = useRef<HTMLTextAreaElement | null>(null);

  // Auto-grow: the textarea height tracks its content (no fixed row cage).
  function grow() {
    const ta = taRef.current;
    if (!ta) return;
    ta.style.height = "auto";
    ta.style.height = `${ta.scrollHeight}px`;
  }
  useEffect(() => {
    grow();
  }, [value]);

  return (
    <div className="rounded-mk-lg border border-mk-accent bg-mk-accent-50 px-5 py-4">
      {title && <h3 className="font-sans text-[15px] font-bold text-mk-ink">{title}</h3>}
      <p className="mt-1 whitespace-pre-wrap text-[14.5px] leading-relaxed text-mk-ink">{guidance}</p>
      {example && (
        <div className="mt-2.5 rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2">
          <p className="text-[12px] font-bold text-mk-muted">范例（英文，供参考）</p>
          <p className="mt-1 whitespace-pre-wrap text-[13px] leading-relaxed text-mk-muted">{example}</p>
        </div>
      )}

      <textarea
        ref={taRef}
        value={value}
        onChange={(e) => { onChange(e.target.value); grow(); }}
        onInput={grow}
        readOnly={locked}
        placeholder={placeholder}
        rows={3}
        className="mt-3 block w-full resize-none overflow-hidden rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-[14.5px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent"
      />

      {cardOffer && cardOffer.length > 0 && onOfferCard && (
        <div className="mt-2.5 flex flex-wrap items-center gap-2">
          <span className="text-[12px] font-bold text-mk-muted">需要帮你把论证搭起来吗？</span>
          {cardOffer.map((c) => (
            <button
              key={c.id}
              type="button"
              onClick={() => onOfferCard(c.id)}
              className="rounded-full border border-mk-border bg-mk-surface px-2.5 py-0.5 text-[12px] font-bold text-mk-accent hover:bg-mk-accent-100"
            >
              {c.label}
            </button>
          ))}
        </div>
      )}

      <div className="mt-3 flex flex-wrap items-center gap-3">
        <button
          type="button"
          onClick={onStillStuck}
          className="rounded-mk-md border border-mk-border px-3 py-1.5 text-[14px] font-bold text-mk-ink hover:border-mk-accent"
        >
          我依然有问题
        </button>
        <button
          type="button"
          disabled={reviewing}
          onClick={onDone}
          className="rounded-mk-md bg-mk-accent px-4 py-1.5 text-[14px] font-bold text-white hover:bg-mk-accent-600 disabled:opacity-50"
        >
          {reviewing ? "印记在看……" : doneLabel}
        </button>
        {footer}
      </div>
      {reviewing && <div className="mt-2"><ReviewingHint /></div>}
    </div>
  );
}
