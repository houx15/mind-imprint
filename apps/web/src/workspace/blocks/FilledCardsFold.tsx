import { useState } from "react";
import { GuidedWritingCard } from "./GuidedWritingCard";

// FilledCardsFold — #82 · the one place to re-read every finished guidance card.
// Each written part/claim is a title-only row, folded by default (so a card
// AUTO-FOLDS the moment it's finished and the walk moves on); clicking it expands
// to show the student's own words. Shared by the proposal (its parts) and the
// essay (its claims). Empty parts are not passed in, so the list is exactly
// "what I've written so far".
//
// A finished part is FINISHED, not LOCKED — when `onEdit` is provided the
// expanded view is the SAME guided-writing card the student wrote in: its
// original guidance + example + the warm, auto-growing textarea, so re-reading
// and re-editing look no different from the first pass (铁律①: still the
// student's text — 印记 never wrote it, and doesn't now). A part with no cached
// guidance falls back to a bare editable textarea; without `onEdit`, read-only.

export type FilledCard = {
  key: string;
  title: string;
  text: string;
  // The part's original guidance + example (from its cached guide card), so the
  // expanded re-edit view is the same surface as when it was first written.
  guidance?: string;
  example?: string | null;
};

export function FilledCardsFold({
  cards,
  heading = "已写好的部分",
  onEdit,
}: {
  cards: FilledCard[];
  heading?: string;
  // When provided, an expanded part becomes editable in place (writing-block
  // style) and edits flow back through this callback; otherwise it's read-only.
  onEdit?: (key: string, text: string) => void;
}) {
  const [open, setOpen] = useState<Set<string>>(new Set());
  if (cards.length === 0) return null;
  return (
    <div className="rounded-mk-md border border-mk-border bg-mk-paper p-3">
      <p className="mb-2 text-[12px] font-bold text-mk-faint">
        {heading} · 你写过的都在这里，{onEdit ? "点开还能接着改" : "点开重读"}（印记不替你写）
      </p>
      <div className="flex flex-col gap-1.5">
        {cards.map((c) => {
          const isOpen = open.has(c.key);
          return (
            <div key={c.key} className="rounded-mk border border-mk-border bg-mk-surface">
              <button
                type="button"
                onClick={() =>
                  setOpen((s) => {
                    const n = new Set(s);
                    n.has(c.key) ? n.delete(c.key) : n.add(c.key);
                    return n;
                  })
                }
                className="flex w-full items-center gap-2 px-3 py-2 text-left"
              >
                <span className={"flex-none text-[11px] text-mk-faint transition-transform " + (isOpen ? "rotate-90" : "")}>▸</span>
                <span className="min-w-0 flex-1 truncate text-[13px] font-semibold text-mk-ink">{c.title}</span>
              </button>
              {isOpen && (
                // Expand on the SAME guided-writing surface the student wrote in:
                // original guidance + example + their text. `onEdit` → editable
                // (re-reading/editing reads no different from the first pass, 铁律①
                // still their own text). No `onEdit` (finished/locked project) →
                // the SAME card but read-only, so the guidance is still visible —
                // never a bare text slab that drops the guidance.
                <div className="px-2 pb-2 pt-1">
                  <GuidedWritingCard
                    guidance={c.guidance ?? ""}
                    example={c.example}
                    value={c.text}
                    onChange={(t) => onEdit?.(c.key, t)}
                    locked={!onEdit}
                    placeholder={onEdit ? "在这里接着改这一部分……" : "（还没写这一部分）"}
                  />
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
