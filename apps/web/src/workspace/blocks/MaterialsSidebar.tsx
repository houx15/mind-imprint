import { useEffect, useRef, useState } from "react";
import type { Reference } from "@mind-imprint/contracts";
import { getLibrary, getOutline } from "../api/workspace";
import { Icon } from "../Icon";

// #23 / #9 · the writing room's materials sidebar: a floating, DRAGGABLE panel
// that connects the three writing tabs. It browses three sources — 材料 (each
// reference's notes / 归纳 / reading-card fragments), 大纲 (the outline), and
// 片段 (the snippet board) — and places any fragment where the student is
// working: into the DRAFT at the caret when the 正文 tab is active, otherwise
// appended as a new snippet. On 正文 it can also lay the whole outline in as
// Markdown headings (把大纲导入正文), so the three tabs feel like one workspace.
// Defaults docked LEFT; drag the header to move; collapse to a small tab.

const CRED_LABEL: Record<NonNullable<Reference["credibility"]>, string> = {
  strong: "可信度高",
  mixed: "需交叉核实",
  weak: "存疑",
};

type Source = "material" | "outline" | "snippet";
type OutlineRow = { id: string; text: string; depth: number };

export function MaterialsSidebar({
  projectId,
  activeTab,
  locked,
  snippets,
  onAddSnippet,
  onInsertToDraft,
}: {
  projectId: string;
  activeTab: "outline" | "snippets" | "draft";
  locked: boolean;
  snippets: { id: string; text: string }[];
  onAddSnippet: (text: string) => void;
  onInsertToDraft: (text: string) => void;
}) {
  const [refs, setRefs] = useState<Reference[]>([]);
  const [outline, setOutline] = useState<OutlineRow[]>([]);
  const [source, setSource] = useState<Source>("material");
  const [open, setOpen] = useState(true);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [placed, setPlaced] = useState<string | null>(null);
  // Floating position within the writing room's relative container; defaults left.
  const [pos, setPos] = useState({ x: 16, y: 16 });
  const drag = useRef<{ dx: number; dy: number } | null>(null);

  // When 正文 is the active tab, placing an item drops it into the draft at the
  // caret; otherwise it's collected as a new snippet.
  const toDraft = activeTab === "draft";
  const actionLabel = toDraft ? "插入正文" : "收进片段";

  useEffect(() => {
    let alive = true;
    getLibrary(projectId)
      .then((lib) => { if (alive) setRefs(lib.references); })
      .catch(() => { /* leave empty — the sidebar just shows the empty state */ });
    return () => { alive = false; };
  }, [projectId]);

  // Load the outline lazily (and refresh it) whenever the 大纲 source is shown,
  // so recent outline edits appear without lifting the outline's state.
  useEffect(() => {
    if (source !== "outline") return;
    let alive = true;
    getOutline(projectId)
      .then((nodes) => { if (alive) setOutline(nodes.map((n) => ({ id: n.id, text: n.text, depth: n.depth }))); })
      .catch(() => { /* leave last-good */ });
    return () => { alive = false; };
  }, [source, projectId]);

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

  // Place a fragment where the student is working, with a brief "done" flash.
  function place(text: string, key: string) {
    const t = text.trim();
    if (!t) return;
    (toDraft ? onInsertToDraft : onAddSnippet)(t);
    setPlaced(key);
    window.setTimeout(() => setPlaced((k) => (k === key ? null : k)), 1200);
  }

  function importOutline() {
    const md = outlineToHeadings(outline);
    if (md) onInsertToDraft(md);
    setPlaced("import-outline");
    window.setTimeout(() => setPlaced((k) => (k === "import-outline" ? null : k)), 1200);
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

      {/* #9 · source switcher — browse materials, the outline, or your snippets */}
      <div className="flex flex-none gap-1 border-b border-mk-border px-2 py-1.5">
        <SourceTab active={source === "material"} onClick={() => setSource("material")}>材料</SourceTab>
        <SourceTab active={source === "outline"} onClick={() => setSource("outline")}>大纲</SourceTab>
        <SourceTab active={source === "snippet"} onClick={() => setSource("snippet")}>片段</SourceTab>
      </div>
      {locked && (
        // review M1 · archived project — browse-only, so no place buttons flash a
        // false "已插入" on a draft that can't change.
        <p className="flex-none border-b border-mk-border bg-mk-bg/60 px-3 py-1.5 text-[11.5px] font-semibold text-mk-muted-2">已归档 · 只读浏览</p>
      )}

      <div className="min-h-0 flex-1 overflow-y-auto p-2.5">
        {source === "material" ? (
          refs.length === 0 ? (
            <Empty>还没有文献。去阅读室加一些来源，读过的笔记会出现在这里。</Empty>
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
                                  {!locked && <PlaceButton done={placed === key} label={actionLabel} onClick={() => place(c.text, key)} />}
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
          )
        ) : source === "outline" ? (
          outline.filter((n) => n.text.trim()).length === 0 ? (
            <Empty>大纲还是空的。去「大纲」里搭个骨架，这里就能把它搬进正文。</Empty>
          ) : (
            <div className="flex flex-col gap-1.5">
              {toDraft && !locked && (
                <button
                  type="button"
                  onClick={importOutline}
                  className="mb-1 rounded-mk border border-mk-primary/40 px-2.5 py-1.5 text-[12px] font-bold text-mk-primary hover:bg-mk-primary-tint"
                >
                  {placed === "import-outline" ? "已导入 ✓" : "把大纲导入正文（作为标题）"}
                </button>
              )}
              {outline.filter((n) => n.text.trim()).map((n) => {
                const key = `o:${n.id}`;
                return (
                  <div key={key} className="rounded bg-mk-bg/50 p-2" style={{ marginLeft: Math.max(0, n.depth) * 12 }}>
                    <div className="text-[12.5px] leading-relaxed text-mk-ink">{n.text}</div>
                    {!locked && <PlaceButton done={placed === key} label={actionLabel} onClick={() => place(n.text, key)} />}
                  </div>
                );
              })}
            </div>
          )
        ) : (
          snippets.filter((s) => s.text.trim()).length === 0 ? (
            <Empty>还没有片段。在「片段」里攒一些，或从「材料」收进来。</Empty>
          ) : (
            <div className="flex flex-col gap-1.5">
              {snippets.filter((s) => s.text.trim()).map((s) => {
                const key = `s:${s.id}`;
                return (
                  <div key={key} className="rounded bg-mk-bg/50 p-2">
                    <div className="text-[12.5px] leading-relaxed text-mk-ink">{s.text}</div>
                    {/* in 片段 tab inserting a snippet into snippets is a no-op path;
                        only offer placing when it goes somewhere new (正文). */}
                    {toDraft && !locked && <PlaceButton done={placed === key} label={actionLabel} onClick={() => place(s.text, key)} />}
                  </div>
                );
              })}
            </div>
          )
        )}
      </div>
    </div>
  );
}

function SourceTab({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex-1 rounded-mk px-2 py-1 text-[12px] font-bold transition ${active ? "bg-mk-primary-tint text-mk-primary" : "text-mk-muted-2 hover:text-mk-ink"}`}
    >
      {children}
    </button>
  );
}

function PlaceButton({ done, label, onClick }: { done: boolean; label: string; onClick: () => void }) {
  return (
    <div className="mt-1 flex justify-end">
      <button
        type="button"
        onClick={onClick}
        className="rounded-full bg-mk-primary px-2.5 py-1 text-[11px] font-bold text-white transition hover:bg-mk-primary-hover"
      >
        {done ? `已${label} ✓` : label}
      </button>
    </div>
  );
}

function Empty({ children }: { children: React.ReactNode }) {
  return <p className="px-1 py-6 text-center text-[12.5px] text-mk-muted-2">{children}</p>;
}

// outlineToHeadings turns the flat depth list into Markdown headings the draft's
// preview/export already understand (depth 0 → #, 1 → ##, clamped at ###).
function outlineToHeadings(rows: OutlineRow[]): string {
  return rows
    .filter((n) => n.text.trim())
    // clamp both ends — String.repeat throws on a negative count, and the
    // OutlineNode contract doesn't floor depth at 0 (review L2).
    .map((n) => `${"#".repeat(Math.max(1, Math.min(n.depth + 1, 3)))} ${n.text.trim()}`)
    .join("\n\n");
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
