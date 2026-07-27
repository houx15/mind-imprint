import type { Anchor, AnnotateState, MaterialSource } from "@mind-imprint/contracts";
import type { CreatedSpan } from "../../primitives/annotate";
import { AddSourceForm } from "./AddSourceForm";
import { SourceLog } from "./SourceLog";
import type { AddMaterialBody } from "../../api/materials";

export type SourceDossierProps = {
  sources: MaterialSource[];
  anchors?: Anchor[];
  onOpenLogged?: (materialId: string, timeSpentS: number) => void;
  // Fires when a source is opened for reading — asks 印记 to surface the
  // source's evaluation card + generate the article's flagged-sentence anchors
  // ("read it with you"). The container refetches on resolve, so returning its
  // promise lets this component show a brief "印记 正在读这篇…" indicator until
  // the highlights land. Best-effort; may be void.
  onPrepareAnnotation?: (materialId: string) => void | Promise<void>;
  onAddSource?: (body: AddMaterialBody) => Promise<void>;
  addSourceError?: string;
  // N3c task 9 (spec §7.2): 「去文章里选出这句」 needs to force this exact
  // source open in the center pane, from OUTSIDE this component's own
  // list→article navigation. Optional and CONTROLLED-when-set: non-null
  // wins over the local `openId` the moment it changes; absent (the
  // default), behavior is byte-identical to before this prop existed.
  openSourceId?: string | null;
  // Task-9 review IMPORTANT 1 fix: the container's monotonically increasing
  // request id, bumped every time she takes a fresh 「去文章里选出这句」
  // action — even one that names the SAME material as a still-pending
  // `openSourceId`. Without a token distinct from the material id, a second
  // locate click after she had navigated back to 信源列表 herself (which
  // only changes THIS component's local `openId`, never the container's
  // `openSourceId`) left the effect below with an unchanged dep value, so
  // React never re-ran it and the source silently failed to reopen — a dead
  // control. See the effect's own doc comment for why keying on the token
  // (rather than re-adding an `openSourceId === openId` guard) is the fix.
  openToken?: number;
  // Select-mode wiring for the open article (spec §8) — present only while
  // the container's locate request targets THIS source; gated below against
  // `openSourceId` so it can never be shown against the wrong article (e.g.
  // she navigated to a different source herself while a request was still
  // pending elsewhere).
  selectMode?: { dimension: string; onCancel: () => void } | null;
  onCreateSpan?: (span: CreatedSpan) => void;
  // Task 8 (read-together redesign): when set, clicking a source-LIST row
  // opens the focused ReadingRoom surface (coach left / article right)
  // INSTEAD of this component's own in-place article view — the container
  // owns that navigation (mirrors the directory→studio swap). Optional and
  // additive: absent (the default), the in-place `activateSource` behavior
  // below is byte-identical to before this prop existed, so every other
  // call site/test is unaffected.
  onOpenReading?: (materialId: string) => void;
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

// Task 8 (read-together redesign): SourceDossier is list-mode ONLY now — the
// in-place article view (open/close, annotate-prepare, the locate-into-
// article machinery) was retired in Task 11 once SIFT/CRAAP moved entirely
// into the reading room (ReadingRoom.tsx). A row click always calls
// onOpenReading; `anchors`/`onOpenLogged`/`onPrepareAnnotation`/
// `openSourceId`/`openToken`/`selectMode`/`onCreateSpan` are accepted for
// caller back-compat (every ViewFrame/PerspectivesView mount still passes a
// subset of them) but are no longer read here — ReadingRoom now owns reading-
// time logging, annotate-prepare, and the article select-mode entirely.
export function SourceDossier({ sources, onAddSource, addSourceError, onOpenReading }: SourceDossierProps) {
  const lockedCount = sources.filter((s) => s.locked).length;

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
  //
  // `!source.siftSkipped` is FIX 3 (whole-branch review): the server's
  // siftSurfaced suppression (classifier.go) treats a SKIPPED SIFT the same
  // as any in-flight/terminal one and never re-proposes it on this material
  // — so without this exclusion the chip would keep claiming "正在核对 ·
  // 需横向阅读" (an ACTIVE process) forever after she explicitly chose to
  // skip it, the exact unclearable-nag-with-no-honest-resolution class the
  // isLateralInstrument fix above exists to prevent, just reached by a
  // different path. Her skip decision is not erased anywhere — it still
  // lives in the process tree (the card_instance row + its event_trace) —
  // this only stops the LIVE chip from mis-describing the state as
  // "currently being verified" once nothing is.
  function needsLateralRead(source: MaterialSource): boolean {
    return source.locked && !source.lateralRead && !source.isLateralInstrument && !source.siftSkipped;
  }

  return (
    <div style={{ fontFamily: "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif" }}>
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
                onClick={() => onOpenReading?.(source.id)}
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
    </div>
  );
}
