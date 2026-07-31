import { useEffect, useRef, useState } from "react";
import type { Reference } from "@mind-imprint/contracts";
import { getLibrary } from "../api/workspace";
import { Icon } from "../Icon";

// #23 · the writing room's materials sidebar: a floating, DRAGGABLE panel that
// lists the project's references and unfolds each source's notes (我的笔记 /
// 归纳 / reading-card notes). Any fragment can be 收进片段 → appended to the
// snippet board via onInsert, so the student pulls evidence into her writing
// without leaving the room. Defaults docked at the LEFT; drag the header to
// move it anywhere; collapse to a small tab.

const CRED_LABEL: Record<NonNullable<Reference["credibility"]>, string> = {
  strong: "可信度高",
  mixed: "需交叉核实",
  weak: "存疑",
};

export function MaterialsSidebar({ projectId, onInsert }: { projectId: string; onInsert: (text: string) => void }) {
  const [refs, setRefs] = useState<Reference[]>([]);
  const [open, setOpen] = useState(true);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [inserted, setInserted] = useState<string | null>(null);
  // Floating position within the writing room's relative container; defaults left.
  const [pos, setPos] = useState({ x: 16, y: 16 });
  const drag = useRef<{ dx: number; dy: number } | null>(null);

  useEffect(() => {
    let alive = true;
    getLibrary(projectId)
      .then((lib) => { if (alive) setRefs(lib.references); })
      .catch(() => { /* leave empty — the sidebar just shows the empty state */ });
    return () => { alive = false; };
  }, [projectId]);

  function onHeaderPointerDown(e: React.PointerEvent) {
    drag.current = { dx: e.clientX - pos.x, dy: e.clientY - pos.y };
    (e.target as HTMLElement).setPointerCapture(e.pointerId);
  }
  function onHeaderPointerMove(e: React.PointerEvent) {
    if (!drag.current) return;
    // Clamp so the panel can never be dragged fully off-screen and lost (its
    // header — and the close button — must stay reachable).
    const maxX = Math.max(0, window.innerWidth - 120);
    const maxY = Math.max(0, window.innerHeight - 80);
    setPos({
      x: Math.min(maxX, Math.max(0, e.clientX - drag.current.dx)),
      y: Math.min(maxY, Math.max(0, e.clientY - drag.current.dy)),
    });
  }
  function onHeaderPointerUp(e: React.PointerEvent) {
    drag.current = null;
    (e.target as HTMLElement).releasePointerCapture(e.pointerId);
  }

  function insert(text: string, key: string) {
    const t = text.trim();
    if (!t) return;
    onInsert(t);
    setInserted(key);
    window.setTimeout(() => setInserted((k) => (k === key ? null : k)), 1200);
  }

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        style={{ left: pos.x, top: pos.y }}
        className="absolute z-30 flex items-center gap-1.5 rounded-mk-lg border border-mk-border bg-mk-surface px-3 py-2 text-[12.5px] font-bold text-mk-primary shadow-[0_4px_16px_rgba(28,35,51,0.12)] hover:bg-mk-primary-tint"
      >
        <Icon name="reading" size={14} /> 材料
      </button>
    );
  }

  return (
    <div
      style={{ left: pos.x, top: pos.y }}
      className="absolute z-30 flex max-h-[80%] w-[300px] flex-col overflow-hidden rounded-mk-lg border border-mk-border bg-mk-surface shadow-[0_8px_32px_rgba(28,35,51,0.18)]"
    >
      <header
        onPointerDown={onHeaderPointerDown}
        onPointerMove={onHeaderPointerMove}
        onPointerUp={onHeaderPointerUp}
        className="flex flex-none cursor-grab items-center justify-between border-b border-mk-border bg-mk-bg/60 px-3 py-2 active:cursor-grabbing"
      >
        <div className="flex items-center gap-1.5 text-mk-primary">
          <Icon name="reading" size={14} />
          <span className="text-[13px] font-bold">材料</span>
          <span className="text-[11px] font-semibold text-mk-muted-2">拖动可移动</span>
        </div>
        <button type="button" onClick={() => setOpen(false)} className="text-[15px] leading-none text-mk-muted-2 hover:text-mk-ink">×</button>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto p-2.5">
        {refs.length === 0 ? (
          <p className="px-1 py-6 text-center text-[12.5px] text-mk-muted-2">还没有文献。去阅读室加一些来源，读过的笔记会出现在这里。</p>
        ) : (
          <div className="flex flex-col gap-1.5">
            {refs.map((r) => {
              const isOpen = expanded === r.id;
              const chunks = referenceChunks(r);
              return (
                <div key={r.id} className="rounded-mk border border-mk-border">
                  <button
                    type="button"
                    onClick={() => setExpanded((e) => (e === r.id ? null : r.id))}
                    className="flex w-full items-center gap-1.5 px-2.5 py-2 text-left"
                  >
                    <span className={`transition ${isOpen ? "rotate-90" : ""} text-mk-muted-2`}>▸</span>
                    <span className="min-w-0 flex-1 truncate text-[12.5px] font-semibold text-mk-ink">{r.title || "未命名来源"}</span>
                    {r.credibility && <span className="flex-none rounded bg-mk-bg px-1.5 py-0.5 text-[10px] font-bold text-mk-muted-2">{CRED_LABEL[r.credibility]}</span>}
                  </button>
                  {isOpen && (
                    <div className="border-t border-mk-border px-2.5 py-2">
                      {chunks.length === 0 ? (
                        <p className="text-[12px] text-mk-muted-2">这篇还没有笔记或归纳——去阅读室读一读。</p>
                      ) : (
                        <div className="flex flex-col gap-2">
                          {chunks.map((c, i) => {
                            const key = `${r.id}:${i}`;
                            return (
                              <div key={key} className="rounded bg-mk-bg/50 p-2">
                                <div className="text-[10px] font-bold uppercase tracking-wide text-mk-muted-2">{c.label}</div>
                                <div className="mt-0.5 text-[12.5px] leading-relaxed text-mk-ink">{c.text}</div>
                                <div className="mt-1 flex justify-end">
                                  <button
                                    type="button"
                                    onClick={() => insert(c.text, key)}
                                    className="rounded-full bg-mk-primary px-2.5 py-1 text-[11px] font-bold text-white transition hover:bg-mk-primary-hover"
                                  >
                                    {inserted === key ? "已收进片段 ✓" : "收进片段"}
                                  </button>
                                </div>
                              </div>
                            );
                          })}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

// referenceChunks flattens a reference's note-bearing fields into individually
// insertable fragments: 我的笔记, the 归纳 (proposalImpact + findings + key
// quotes), and each reading-card note (quote → finding).
type Chunk = { label: string; text: string };
function referenceChunks(r: Reference): Chunk[] {
  const out: Chunk[] = [];
  if (r.readingNote && r.readingNote.trim()) out.push({ label: "我的笔记", text: r.readingNote.trim() });
  const tk = r.takeaway;
  if (tk) {
    if (tk.proposalImpact && tk.proposalImpact.trim()) out.push({ label: "对论证的影响", text: tk.proposalImpact.trim() });
    for (const f of tk.findings ?? []) if (f.trim()) out.push({ label: "发现", text: f.trim() });
    for (const q of tk.keyQuotes ?? []) if (q.quote?.trim()) out.push({ label: "关键引文", text: q.why?.trim() ? `“${q.quote.trim()}” —— ${q.why.trim()}` : `“${q.quote.trim()}”` });
  }
  for (const n of r.notes ?? []) {
    const parts = [n.quote?.trim(), n.finding?.trim()].filter(Boolean);
    if (parts.length) out.push({ label: "阅读笔记", text: parts.join(" —— ") });
  }
  return out;
}
