import { useEffect, useMemo, useRef, useState } from "react";
import type { ExplorationLead, Reference } from "@mind-imprint/contracts";
import { getLibrary, getOutline } from "../api/workspace";
import { getExploration } from "../../api/exploration";
import { Icon } from "../Icon";

// #23 / #9 · the writing room's materials sidebar: a floating, DRAGGABLE panel
// that connects the three writing tabs. It browses three sources — 材料 (each
// reference's notes / 归纳 / reading-card fragments), 大纲 (the outline), and
// 片段 (the snippet board) — and places any fragment where the student is
// working: into the DRAFT at the caret when the 正文 tab is active, otherwise
// appended as a new snippet. The 大纲 source also offers two whole-outline
// imports, independent of which tab is active: 把大纲导入正文 lays the headings
// into the draft as Markdown; 把大纲导入为片段分组 (#6) turns them into foldable
// 片段 board sections (an explicit one-shot opt-in — the live outline is never
// auto-rendered there). Defaults docked LEFT; drag the header to move; collapse
// to a small tab.

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
  onImportOutlineAsGroups,
  importedSections,
}: {
  projectId: string;
  activeTab: "outline" | "snippets" | "draft";
  locked: boolean;
  snippets: { id: string; text: string; section: string | null }[];
  onAddSnippet: (text: string) => void;
  onInsertToDraft: (text: string) => void;
  // #6 · 把大纲导入为片段分组 — a ONE-TIME explicit action (never automatic) that
  // turns the outline's top-level headings into foldable 片段 board sections.
  // importedSections is the current imported set, so the button can read
  // "already imported" instead of always inviting a re-import.
  onImportOutlineAsGroups: (headings: string[]) => void;
  importedSections: string[];
}) {
  const [refs, setRefs] = useState<Reference[]>([]);
  const [outline, setOutline] = useState<OutlineRow[]>([]);
  const [source, setSource] = useState<Source>("material");
  // Batch5 follow-up Item B · which source tabs are offered depends on the
  // active MAIN panel: 大纲 only needs 材料 (nothing to browse yet); 片段 adds
  // 大纲 (turning headings into evidence); 正文 adds 片段 too (everything you've
  // gathered, ready to place). 材料 is always available. If the panel changes
  // out from under a hidden source (e.g. she was on 大纲/片段 source, then
  // switched the main panel back to 大纲), fall back to 材料 rather than
  // leaving the sidebar stuck on a source it no longer offers a tab for.
  const showOutlineSource = activeTab !== "outline";
  const showSnippetSource = activeTab === "draft";
  useEffect(() => {
    setSource((cur) => {
      if (cur === "outline" && !showOutlineSource) return "material";
      if (cur === "snippet" && !showSnippetSource) return "material";
      return cur;
    });
  }, [showOutlineSource, showSnippetSource]);
  const [open, setOpen] = useState(true);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [placed, setPlaced] = useState<string | null>(null);
  // #15 · a brief toast so placing a fragment isn't a silent no-op when its
  // destination (the 片段 board) isn't the tab you're looking at.
  const [toast, setToast] = useState<string | null>(null);
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // #16 · filter the 片段 browse by category (章节/线索 label the snippet is filed under).
  const [snipFilter, setSnipFilter] = useState<string>("__all__");
  // #16 (materials) · the exploration graph's leads, fetched read-only so the
  // 材料 tab can categorize references by which 线索 they're under — no new
  // persistence, the association already lives in the leads themselves
  // (sourceReferenceId / connectedReferenceId, incl. child-branch sources).
  const [leads, setLeads] = useState<ExplorationLead[]>([]);
  const [matFilter, setMatFilter] = useState<string>("__all__");
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

  // #16 (materials) · fetched once on mount, same as refs above — read-only,
  // no spend. A failed fetch just means no 线索 categorization is offered yet.
  useEffect(() => {
    let alive = true;
    getExploration(projectId)
      .then((v) => { if (alive) setLeads(v.leads); })
      .catch(() => { /* the filter is a nicety; the tab stays usable without it */ });
    return () => { alive = false; };
  }, [projectId]);

  // A reference is associated with a lead when it's that lead's
  // sourceReferenceId (the source that spawned the thread) or
  // connectedReferenceId (a resolved lead OR — Item A — a child branch
  // created specifically to attach a source under a parent lead). Child
  // associations roll up to their TOP-LEVEL ancestor for the category label,
  // since a child created just to hold a source has no meaningful name of
  // its own — the meaningful 线索 is the thread it hangs under.
  const { refToLead, leadOptions } = useMemo(() => {
    const byId = new Map(leads.map((l) => [l.id, l]));
    function rootOf(l: ExplorationLead): ExplorationLead {
      let cur = l;
      const seen = new Set<string>();
      while (cur.parentLeadId && byId.has(cur.parentLeadId) && !seen.has(cur.id)) {
        seen.add(cur.id);
        cur = byId.get(cur.parentLeadId)!;
      }
      return cur;
    }
    const refToLead = new Map<string, { id: string; text: string }>();
    for (const l of leads) {
      const refIds = [l.sourceReferenceId, l.connectedReferenceId].filter((x): x is string => !!x);
      if (refIds.length === 0) continue;
      const root = rootOf(l);
      for (const rid of refIds) {
        if (!refToLead.has(rid)) refToLead.set(rid, { id: root.id, text: root.text });
      }
    }
    const byRootId = new Map<string, string>();
    for (const v of refToLead.values()) byRootId.set(v.id, v.text);
    const leadOptions = Array.from(byRootId.entries()).map(([id, text]) => ({ id, text }));
    return { refToLead, leadOptions };
  }, [leads]);
  // clamp a filter whose 线索 has since vanished back to 全部 (mirrors 片段's effFilter guard).
  const effMatFilter =
    matFilter === "__all__" || matFilter === "__uncat__" || leadOptions.some((o) => o.id === matFilter) ? matFilter : "__all__";
  const filteredRefs = useMemo(
    () =>
      refs.filter((r) => {
        if (effMatFilter === "__all__") return true;
        if (effMatFilter === "__uncat__") return !refToLead.has(r.id);
        return refToLead.get(r.id)?.id === effMatFilter;
      }),
    [refs, effMatFilter, refToLead],
  );

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
  // When it lands somewhere off-screen (收进片段 while not on the 片段 tab), also
  // toast so it never feels like nothing happened (#15).
  function place(text: string, key: string) {
    const t = text.trim();
    if (!t) return;
    (toDraft ? onInsertToDraft : onAddSnippet)(t);
    setPlaced(key);
    window.setTimeout(() => setPlaced((k) => (k === key ? null : k)), 1200);
    if (!toDraft && activeTab !== "snippets") {
      setToast("已收进「片段」——切到片段标签查看");
      if (toastTimer.current) clearTimeout(toastTimer.current);
      toastTimer.current = setTimeout(() => setToast(null), 2400);
    }
  }
  // clear a pending toast timer on unmount (review Low)
  useEffect(() => () => { if (toastTimer.current) clearTimeout(toastTimer.current); }, []);

  function importOutline() {
    const md = outlineToHeadings(outline);
    if (md) onInsertToDraft(md);
    setPlaced("import-outline");
    window.setTimeout(() => setPlaced((k) => (k === "import-outline" ? null : k)), 1200);
  }

  // #6 · 把大纲导入为片段分组 — top-level headings become foldable 片段 board
  // sections (a one-time opt-in; the live outline is never auto-rendered as
  // snippet sections). Independent of which room tab is active — the target
  // is always the 片段 board, not "wherever you're currently working".
  const topHeadings = outline.filter((n) => n.depth === 0 && n.text.trim()).map((n) => n.text.trim());
  const alreadyImported = topHeadings.length > 0 && topHeadings.every((h) => importedSections.includes(h));
  function importOutlineGroups() {
    if (topHeadings.length === 0) return;
    onImportOutlineAsGroups(topHeadings);
    setPlaced("import-outline-groups");
    window.setTimeout(() => setPlaced((k) => (k === "import-outline-groups" ? null : k)), 1200);
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

      {/* #9 · source switcher — browse materials, the outline, or your snippets.
          Item B: which tabs show up depends on the active main panel (大纲 only
          gets 材料; 片段 adds 大纲; 正文 adds 片段 too). */}
      <div className="flex flex-none gap-1 border-b border-mk-border px-2 py-1.5">
        <SourceTab active={source === "material"} onClick={() => setSource("material")}>材料</SourceTab>
        {showOutlineSource && <SourceTab active={source === "outline"} onClick={() => setSource("outline")}>大纲</SourceTab>}
        {showSnippetSource && <SourceTab active={source === "snippet"} onClick={() => setSource("snippet")}>片段</SourceTab>}
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
              {/* #16 · 线索 as a categorization tag for materials — filter to
                  references under a chosen 线索 (only shown once at least one
                  reference is actually associated with a lead). */}
              {leadOptions.length > 0 && (
                <select
                  value={effMatFilter}
                  onChange={(e) => setMatFilter(e.target.value)}
                  aria-label="按线索筛选材料"
                  className="mb-1 rounded border border-mk-border bg-mk-surface px-2 py-1 text-[11.5px] text-mk-ink outline-none focus:border-mk-primary"
                >
                  <option value="__all__">全部线索</option>
                  {leadOptions.map((o) => (
                    <option key={o.id} value={o.id}>{o.text}</option>
                  ))}
                  <option value="__uncat__">未归入线索</option>
                </select>
              )}
              {filteredRefs.length === 0 && (
                <Empty>这条线索下还没有材料——换一个筛选，或者去阅读室归类。</Empty>
              )}
              {filteredRefs.map((r) => {
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
              {!locked && (
                <div className="mb-1 flex flex-col gap-1.5">
                  {toDraft && (
                    <button
                      type="button"
                      onClick={importOutline}
                      className="rounded-mk border border-mk-primary/40 px-2.5 py-1.5 text-[12px] font-bold text-mk-primary hover:bg-mk-primary-tint"
                    >
                      {placed === "import-outline" ? "已导入 ✓" : "把大纲导入正文（作为标题）"}
                    </button>
                  )}
                  {/* #6 · imports the outline's top-level headings as foldable 片段
                      board sections — an explicit, one-shot action; the board
                      never auto-renders the live outline. */}
                  <button
                    type="button"
                    onClick={importOutlineGroups}
                    className="rounded-mk border border-mk-primary/40 px-2.5 py-1.5 text-[12px] font-bold text-mk-primary hover:bg-mk-primary-tint"
                  >
                    {alreadyImported || placed === "import-outline-groups" ? "已导入为片段分组 ✓" : "把大纲导入为片段分组"}
                  </button>
                </div>
              )}
              {/* #6 · per-row 收进片段 was removed — dumping a heading's NAME as a
                  flat snippet's text read like a stray outline entry, not a
                  student thought. On 正文 a row can still insert its own heading
                  at the caret (mirrors 材料/片段 rows); on 片段 use the import
                  button above instead. */}
              {outline.filter((n) => n.text.trim()).map((n) => {
                const key = `o:${n.id}`;
                return (
                  <div key={key} className="rounded bg-mk-bg/50 p-2" style={{ marginLeft: Math.max(0, n.depth) * 12 }}>
                    <div className="text-[12.5px] leading-relaxed text-mk-ink">{n.text}</div>
                    {toDraft && !locked && <PlaceButton done={placed === key} label="插入正文" onClick={() => place(n.text, key)} />}
                  </div>
                );
              })}
            </div>
          )
        ) : (() => {
          const withText = snippets.filter((s) => s.text.trim());
          if (withText.length === 0) return <Empty>还没有片段。在「片段」里攒一些，或从「材料」收进来。</Empty>;
          // #16 · categories the snippets are filed under (章节/线索 labels)
          const cats = Array.from(new Set(withText.map((s) => s.section).filter((x): x is string => !!x)));
          // clamp a filter whose category has since vanished back to 全部 so the
          // pane never strands the user on a dead value (review Low).
          const effFilter = snipFilter === "__all__" || snipFilter === "__unfiled__" || cats.includes(snipFilter) ? snipFilter : "__all__";
          const filtered = withText.filter((s) =>
            effFilter === "__all__" ? true : effFilter === "__unfiled__" ? s.section == null : s.section === effFilter,
          );
          return (
            <div className="flex flex-col gap-1.5">
              {cats.length > 0 && (
                <select
                  value={effFilter}
                  onChange={(e) => setSnipFilter(e.target.value)}
                  aria-label="按分类筛选片段"
                  className="mb-1 rounded border border-mk-border bg-mk-surface px-2 py-1 text-[11.5px] text-mk-ink outline-none focus:border-mk-primary"
                >
                  <option value="__all__">全部分类</option>
                  {cats.map((c) => <option key={c} value={c}>{c}</option>)}
                  <option value="__unfiled__">未归类</option>
                </select>
              )}
              {filtered.map((s) => {
                const key = `s:${s.id}`;
                return (
                  <div key={key} className="rounded bg-mk-bg/50 p-2">
                    {s.section && <div className="mb-0.5 truncate text-[10px] font-bold text-mk-primary">{s.section}</div>}
                    <div className="text-[12.5px] leading-relaxed text-mk-ink">{s.text}</div>
                    {/* in 片段 tab inserting a snippet into snippets is a no-op path;
                        only offer placing when it goes somewhere new (正文). */}
                    {toDraft && !locked && <PlaceButton done={placed === key} label={actionLabel} onClick={() => place(s.text, key)} />}
                  </div>
                );
              })}
            </div>
          );
        })()}
      </div>
      {toast && (
        <div className="flex-none border-t border-mk-border bg-mk-primary-tint px-3 py-2 text-[11.5px] font-semibold text-mk-primary">{toast}</div>
      )}
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
