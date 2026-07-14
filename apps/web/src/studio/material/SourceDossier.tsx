import { useEffect, useRef, useState } from "react";
import type { Anchor, AnnotateState, MaterialSource } from "@mind-imprint/contracts";
import { Annotate } from "../../primitives/annotate";
import { AddSourceForm } from "./AddSourceForm";
import { SourceLog } from "./SourceLog";
import type { AddMaterialBody } from "../../api/materials";

export type SourceDossierProps = {
  sources: MaterialSource[];
  anchors?: Anchor[];
  onOpenLogged?: (materialId: string, timeSpentS: number) => void;
  onAddSource?: (body: AddMaterialBody) => Promise<void>;
  addSourceError?: string;
};

type AnnotateSpan = AnnotateState["spans"][number];

// Live card anchors are presentation-only here: they highlight the same spans
// the coach rail is asking about, but the mint never depends on this mapping.
// A source-level anchor (empty block_id + a 0..0 range, e.g. risk_note) has
// nowhere to highlight in the article body, so it's skipped.
export function anchorToSpan(anchor: Anchor): AnnotateSpan | null {
  const blockRef = anchor.block_id || undefined;
  const hasRange = anchor.end > anchor.start;
  if (!blockRef && !hasRange) return null;
  return {
    id: anchor.id,
    ...(hasRange ? { range: { start: anchor.start, end: anchor.end } } : {}),
    ...(blockRef ? { block_ref: blockRef } : {}),
    tag: anchor.dimension,
    note: anchor.answer || anchor.question,
    author: anchor.author,
  };
}

function LockIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <rect x="4" y="11" width="16" height="9" rx="2" stroke="#8A93A6" strokeWidth="2" />
      <path d="M8 11V7a4 4 0 0 1 8 0v4" stroke="#8A93A6" strokeWidth="2" />
    </svg>
  );
}

function BackIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M15 18l-6-6 6-6" stroke="#5C4A8A" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function SourceDossier({ sources, anchors, onOpenLogged, onAddSource, addSourceError }: SourceDossierProps) {
  const [openId, setOpenId] = useState<string | null>(null);
  const [activeSpanId, setActiveSpanId] = useState<string | null>(null);
  const openedAtRef = useRef<{ id: string; openedAt: number } | null>(null);
  const onOpenLoggedRef = useRef(onOpenLogged);
  onOpenLoggedRef.current = onOpenLogged;

  const lockedCount = sources.filter((s) => s.locked).length;
  const openSource = openId ? sources.find((s) => s.id === openId) ?? null : null;

  // The chip is DERIVED, never decorated (spec §6): the source has been
  // EVALUATED — a real `evaluated-as` graph edge exists, surfaced as
  // `MaterialSource.locked` (derived server-side, Slice 6b) — and still
  // lacks a lateral (cross-check) read. This is a statement about the
  // SOURCE's state, not about which card happens to be open on it: CRAAP's
  // own anchors also carry this material's id, and CRAAP auto-surfaces on
  // every freshly-added source, so gating on "a live card has anchors here"
  // made the chip fire on the ordinary vertical-check path before the
  // student had finished anything. `locked` only flips once a CRAAP
  // evaluation actually completes, so the chip now names a real, useful
  // debt — "you finished the vertical check; it still owes a lateral one" —
  // surfaced at the moment it becomes true, not as a nag beforehand.
  //
  // `!source.isLateralInstrument` mirrors the SERVER's summon rule exactly
  // (fix-wave finding [4]): SurfaceCardCandidates' treadmill guard never
  // proposes a SIFT card on a material that is itself some OTHER
  // cross_check's lateral (found) source — offering that would let every
  // lateral read manufacture its own next chore, forever. Without this
  // exclusion the chip could tell a student a source "needs 横向阅读" when
  // the summon logic will never actually surface that card, an unclearable
  // nag with no honest resolution. `isLateralInstrument` is server-derived
  // from the same graph edges the summon rule reads — never recomputed here.
  function needsLateralRead(source: MaterialSource): boolean {
    return source.locked && !source.lateralRead && !source.isLateralInstrument;
  }

  const reportOpenElapsed = () => {
    const opened = openedAtRef.current;
    if (!opened) return;
    const timeSpentS = Math.round((Date.now() - opened.openedAt) / 1000);
    openedAtRef.current = null;
    if (timeSpentS === 0) return;
    onOpenLoggedRef.current?.(opened.id, timeSpentS);
  };

  // Report the reading time if the component unmounts while a source is
  // still open (e.g. the student navigates away from 素材 entirely).
  useEffect(() => {
    return () => {
      reportOpenElapsed();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // MaterialSource carries its own persisted anchors (CRAAP-mint / source-log)
  // directly on `anchors` — there is no separate `annotate` field anymore.
  // `anchors` (the prop) is this session's live card anchors. This is the
  // single merge point for the two: live anchors carry the student's
  // in-progress answer and must win on an id collision, so they're placed
  // first — `segmentBlock`'s cursor and `Annotate`'s span lookup both take
  // the first match at a given id/position. Persisted anchors with no live
  // counterpart still render (so a completed card's highlights survive a
  // reload). The `material_id` filter applies only to the live side —
  // `openSource.anchors` is already scoped to this material.
  const liveForSource = openSource
    ? (anchors ?? []).filter((a) => a.material_id === openSource.id)
    : [];
  const liveIds = new Set(liveForSource.map((a) => a.id));
  const persistedForSource = openSource ? openSource.anchors.filter((a) => !liveIds.has(a.id)) : [];
  const mergedSpans = [...liveForSource, ...persistedForSource]
    .map(anchorToSpan)
    .filter((s): s is AnnotateSpan => s !== null);
  const annotateState: AnnotateState | null = openSource
    ? { material_id: openSource.id, spans: mergedSpans }
    : null;

  const openSourceView = (source: MaterialSource) => {
    setOpenId(source.id);
    setActiveSpanId(null);
    openedAtRef.current = { id: source.id, openedAt: Date.now() };
  };

  const backToList = () => {
    reportOpenElapsed();
    setOpenId(null);
    setActiveSpanId(null);
  };

  return (
    <div style={{ fontFamily: "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif" }}>
      {!openSource && (
        <div>
          <div style={{ marginBottom: 14 }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333" }}>信源档案 · 已收集 {sources.length} 篇</div>
            <div style={{ fontSize: 12.5, color: "#7A8296", marginTop: 2 }}>
              已锁定 {lockedCount}/{sources.length}
            </div>
          </div>

          {/* RL-2: the AI supplies no material — the student searches,
              themselves. This form is the only way a source enters a
              project. `onAddSource` is wired by the container (Task 9); a
              control that cannot actually add anything (no handler) must
              not be shown at all. */}
          {onAddSource && <AddSourceForm onSubmit={onAddSource} error={addSourceError} />}

          <div data-testid="dossier-source-list" style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            {sources.map((source) => (
              <button
                key={source.id}
                type="button"
                onClick={() => openSourceView(source)}
                style={{
                  display: "block",
                  textAlign: "left",
                  background: "#fff",
                  border: "1px solid #E4E6EE",
                  borderRadius: 12,
                  padding: "11px 13px",
                  cursor: "pointer",
                }}
              >
                <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                  {source.locked && <LockIcon />}
                  <span style={{ fontSize: 14, fontWeight: 700, color: "#1C2333" }}>{source.title}</span>
                  {/* Chip is derived purely from `locked` (a minted evaluated-as
                      edge) — there is no verdict field (可信/存疑) on
                      MaterialSource, and this never fabricates one. */}
                  <span
                    style={{
                      fontSize: 10.5,
                      fontWeight: 700,
                      padding: "2px 9px",
                      borderRadius: 999,
                      ...(source.locked ? { background: "#E7F3EE", color: "#4C9A82" } : { background: "#F1F2F6", color: "#5A6178" }),
                    }}
                  >
                    {source.locked ? "✓ 已锁定" : "待评估"}
                  </span>
                  {needsLateralRead(source) && (
                    <span
                      style={{
                        fontSize: 10.5,
                        fontWeight: 700,
                        padding: "3px 10px",
                        borderRadius: 999,
                        color: "#C96F4F",
                        background: "#FBEEE7",
                      }}
                    >
                      正在核对 · 需横向阅读
                    </span>
                  )}
                </div>
                <div style={{ fontSize: 12, color: "#8A93A6", marginTop: 4 }}>
                  {source.origin === "fetched" ? "网页" : "粘贴"}
                  {source.tier !== "" && ` · ${source.tier}`}
                </div>
                <div style={{ fontSize: 12.5, color: "#5A6178", marginTop: 6, lineHeight: 1.5 }}>
                  作用与风险：{source.role || "尚未写「作用与风险」"}
                </div>
              </button>
            ))}
          </div>

          <SourceLog sources={sources} />
        </div>
      )}

      {openSource && (
        <div>
          <button
            type="button"
            onClick={backToList}
            style={{
              display: "flex",
              alignItems: "center",
              gap: 4,
              background: "transparent",
              border: "none",
              padding: 0,
              marginBottom: 12,
              color: "#5C4A8A",
              fontSize: 13,
              fontWeight: 600,
              cursor: "pointer",
            }}
          >
            <BackIcon />
            返回信源列表
          </button>

          <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333" }}>{openSource.title}</div>
          <div style={{ fontSize: 12, color: "#8A93A6", margin: "2px 0 12px" }}>
            {openSource.origin === "fetched" ? "网页" : "粘贴"}
            {openSource.tier !== "" && ` · ${openSource.tier}`}
          </div>

          {/* Task 7: the old fixture's `view: "article" | "summary"` toggle
              had no honest producer on MaterialSource — every source now
              renders whatever it truthfully has (blocks and/or a takeaway),
              instead of faking a single-mode switch. */}
          {openSource.blocks.length > 0 && (
            <>
              {needsLateralRead(openSource) ? (
                // Design docs/design/思维印记_工作区.dc.html:1050-1053 — verbatim.
                // Replaces the generic caption below: this is the same fact
                // (locked/evaluated + no cross-check yet), just spelled out
                // for the open article's binding UI.
                <div style={{ display: "flex", alignItems: "center", gap: 7, marginBottom: 11 }}>
                  <span style={{ fontSize: 11.5, fontWeight: 700, color: "#C96F4F", background: "#FBEEE7", padding: "3px 10px", borderRadius: 999 }}>
                    正在核对 · 需横向阅读
                  </span>
                  <span style={{ fontSize: 11.5, color: "#9AA1B0" }}>点亮的句子 = 印记标出的可疑处</span>
                </div>
              ) : null}
              <Annotate
                blocks={openSource.blocks}
                state={annotateState!}
                activeSpanId={activeSpanId}
                onSelectSpan={setActiveSpanId}
              />
              {!needsLateralRead(openSource) && (
                <div style={{ marginTop: 16, fontSize: 12, color: "#A4ABBD" }}>
                  点亮的段落是 AI 标出的可疑处——追问会出现在旁边的陪练轨道。
                </div>
              )}
            </>
          )}

          <div style={{ fontSize: 13, color: "#5A6178", lineHeight: 1.6, marginTop: 12 }}>
            作用与风险：{openSource.role || "尚未写「作用与风险」"}
          </div>

          {openSource.takeaway.trim().length > 0 && (
            <div
              style={{
                marginTop: 14,
                background: "#F7F5FB",
                border: "1px solid #E3DCF2",
                borderRadius: 12,
                padding: "13px 15px",
              }}
            >
              <div style={{ fontSize: 12, fontWeight: 700, color: "#5C4A8A", marginBottom: 6 }}>一句话摘要</div>
              <div style={{ fontSize: 13.5, lineHeight: 1.6, color: "#3A4256" }}>{openSource.takeaway}</div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
