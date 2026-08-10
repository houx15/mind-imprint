import { useEffect, useRef, useState } from "react";

// FilledCardsFold — #82 · the one place to re-read every finished guidance card.
// Each written part/claim is a title-only row, folded by default; clicking it
// expands to show the student's own words. Shared by the proposal (its parts)
// and the essay (its claims). Empty parts are not passed in, so the list is
// exactly "what I've written so far".
//
// A finished part is FINISHED, not LOCKED — when `onEdit` is provided the
// expanded view is the same warm, auto-growing writing surface the guided card
// uses, so the student can still fix their own words in place (铁律①: still the
// student's text — 印记 never wrote it, and doesn't now). Without `onEdit` it
// falls back to a read-only paragraph.

export type FilledCard = { key: string; title: string; text: string };

// One expanded part: an auto-growing textarea styled like GuidedWritingCard's,
// so a finished card reads and edits like the writing surface, not a flat
// read-only slab.
function EditablePart({ text, onEdit }: { text: string; onEdit: (text: string) => void }) {
  const taRef = useRef<HTMLTextAreaElement | null>(null);
  function grow() {
    const ta = taRef.current;
    if (!ta) return;
    ta.style.height = "auto";
    ta.style.height = `${ta.scrollHeight}px`;
  }
  useEffect(() => {
    grow();
  }, [text]);
  return (
    <div className="border-t border-mk-border px-3 py-2">
      <textarea
        ref={taRef}
        value={text}
        onChange={(e) => { onEdit(e.target.value); grow(); }}
        onInput={grow}
        rows={2}
        className="block w-full resize-none overflow-hidden rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-[13.5px] leading-relaxed text-mk-ink outline-none focus:border-mk-accent"
      />
    </div>
  );
}

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
              {isOpen &&
                (onEdit ? (
                  <EditablePart text={c.text} onEdit={(t) => onEdit(c.key, t)} />
                ) : (
                  <p className="whitespace-pre-wrap border-t border-mk-border px-3 py-2 text-[13.5px] leading-relaxed text-mk-ink">
                    {c.text}
                  </p>
                ))}
            </div>
          );
        })}
      </div>
    </div>
  );
}
