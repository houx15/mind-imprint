import { useState } from "react";

// FilledCardsFold — #82 · the one place to re-read every finished guidance card.
// Each written part/claim is a title-only row, folded by default; clicking it
// expands to show the student's own words (read-only — 印记 never wrote them).
// Shared by the proposal (its parts) and the essay (its claims). Empty parts are
// not passed in, so the list is exactly "what I've written so far".

export type FilledCard = { key: string; title: string; text: string };

export function FilledCardsFold({ cards, heading = "已写好的部分" }: { cards: FilledCard[]; heading?: string }) {
  const [open, setOpen] = useState<Set<string>>(new Set());
  if (cards.length === 0) return null;
  return (
    <div className="rounded-mk-md border border-mk-border bg-mk-paper p-3">
      <p className="mb-2 text-[12px] font-bold text-mk-faint">
        {heading} · 你写过的都在这里，点开重读（印记不替你写）
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
                <p className="whitespace-pre-wrap border-t border-mk-border px-3 py-2 text-[13.5px] leading-relaxed text-mk-ink">
                  {c.text}
                </p>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
