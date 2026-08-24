import { Fragment, type ReactNode } from "react";
import type { AnnotateState } from "@mind-imprint/contracts";
import { segmentBlock } from "./segment";
import { selectionToSpan, pointToRuneOffset, type CreatedSpan } from "./selection";
import { segmentSentences, sentenceAtOffset } from "./sentences";

export type AnnotateProps = {
  blocks: { id: string; text: string }[];
  state: AnnotateState;
  activeSpanId: string | null;
  onSelectSpan: (id: string | null) => void;
  /**
   * When set, the article renders in student-locates-the-span mode (guidance
   * ladder L2/L3, spec §2/§8): an instructional hint bar names the dimension
   * she is looking for, with a cancel escape hatch — no level name, no badge,
   * no praise (铁律 2). When absent (the default), render and behavior are
   * unchanged from before this mode existed.
   */
  selectMode?: { dimension: string; onCancel: () => void } | null;
  /** Called with the student's selection once she releases the mouse in select-mode. */
  onCreateSpan?: (span: CreatedSpan) => void;
  /**
   * Optional inline slot, rendered immediately after each block's `<p>` (the
   * read-together redesign's hanging card, Task 9). Absent by default —
   * behavior and output are byte-identical to before this prop existed when
   * it is not supplied.
   */
  renderAfterBlock?: (blockId: string) => ReactNode;
  /**
   * "Click a sentence to reference it" (引用原文). When supplied AND
   * `selectMode` is null (i.e. NOT in evidence-pick mode), each block
   * becomes clickable and calls back with the block's id. Absent by
   * default — behavior is unchanged when it is not supplied.
   */
  onReferenceBlock?: (blockId: string) => void;
  /**
   * Drag-select a phrase/sentence to reference it (引用原文, fine-grained).
   * Fired on mouse-up when a real (non-collapsed) text selection exists AND
   * `selectMode` is null. Lets her quote an arbitrary span into the chat, not
   * only a whole paragraph. Absent by default — behavior unchanged.
   */
  onReferenceSelection?: (blockId: string, quote: string) => void;
  /** Block ids currently referenced (whole-paragraph) — rendered with a subtle highlight. */
  referencedBlockIds?: string[];
  /**
   * Renders the body of the inline card shown when a highlight is clicked. Lets
   * the caller (the reading room) show a rich, real 透镜卡 recap (verdict +
   * checks) for a confirmed finding instead of the built-in dimension+note box.
   * The `rr-inline-card` wrapper + inline placement (after the owning paragraph,
   * foot fallback) stay here; only the card body is delegated. Absent → the
   * built-in box renders, output unchanged.
   */
  renderActiveCard?: (span: AnnotateSpan) => ReactNode;
  /**
   * When a highlighted run belongs to this span, its `<mark>` also carries
   * `data-tour="rr-lens-mark"` — so the guided tour can spotlight/click the ONE
   * confirmed-finding highlight (rich 透镜卡) rather than whichever mark happens
   * to come first in the document. Absent → no mark is tagged.
   */
  lensMarkSpanId?: string;
};

type AnnotateSpan = AnnotateState["spans"][number];

// Three distinct macarons for a genuine 3-way category (who authored this
// span), not one accent collapsed across all of them (色彩纪律): AI-authored
// → taro (matches the app's other reading-annotation chrome, e.g.
// HangingCard's taro treatment), student-authored → matcha, imported →
// butter.
const AUTHOR_MARK_STYLE: Record<"ai" | "student" | "imported", { background: string; border: string }> = {
  ai: { background: "var(--mk-taro-bg)", border: "var(--mk-taro)" },
  student: { background: "var(--mk-matcha-bg)", border: "var(--mk-matcha)" },
  imported: { background: "var(--mk-butter-bg)", border: "var(--mk-butter)" },
};

export function Annotate({
  blocks,
  state,
  activeSpanId,
  onSelectSpan,
  selectMode,
  onCreateSpan,
  renderAfterBlock,
  onReferenceBlock,
  onReferenceSelection,
  referencedBlockIds,
  renderActiveCard,
  lensMarkSpanId,
}: AnnotateProps) {
  const activeSpan = activeSpanId ? state.spans.find((s) => s.id === activeSpanId) ?? null : null;

  // Referencing (引用原文) is only offered outside evidence-pick mode — while
  // selectMode is active the article is for picking evidence, not for
  // building the conversation's referenced-sentence set.
  const referenceEnabled = Boolean(onReferenceBlock) && !selectMode;

  // In select-mode, a click picks the ONE SENTENCE under the cursor (#9),
  // resolved from the click point → rune offset → containing sentence. Falls
  // back to the whole block when the block is a single sentence or the point
  // can't be resolved. A drag still gives a fine-grained partial span via onMouseUp.
  const pickBlock = (block: { id: string; text: string }) => {
    onCreateSpan?.({ blockId: block.id, start: 0, end: Array.from(block.text).length, text: block.text });
  };
  const pickSentence = (block: { id: string; text: string }, clientX: number, clientY: number) => {
    if (!onCreateSpan) return;
    const sentences = segmentSentences(block.text);
    if (sentences.length <= 1) {
      pickBlock(block);
      return;
    }
    const pt = pointToRuneOffset(clientX, clientY);
    if (!pt || pt.blockId !== block.id) {
      pickBlock(block); // couldn't locate the click → whole block, never a wrong guess
      return;
    }
    const sel = sentenceAtOffset(sentences, pt.offset);
    if (!sel) {
      pickBlock(block);
      return;
    }
    onCreateSpan({ blockId: block.id, start: sel.start, end: sel.end, text: sel.text });
  };

  // Attached in select-mode (pick evidence) OR reference-mode (quote into the
  // chat). A drag ends in a real text selection; a plain click leaves it
  // collapsed → selectionToSpan returns null → the block onClick handles it
  // instead, so the two never double-fire. In reference-mode the block onClick
  // additionally bails when a live selection exists, so a drag never ALSO
  // toggles the whole paragraph.
  const handleMouseUp = selectMode
    ? () => {
        const span = selectionToSpan();
        if (span) onCreateSpan?.(span);
      }
    : referenceEnabled && onReferenceSelection
      ? () => {
          const span = selectionToSpan();
          if (span && span.text.trim()) {
            onReferenceSelection(span.blockId, span.text.trim());
            window.getSelection()?.removeAllRanges();
          }
        }
      : undefined;

  // The lens card for the active (clicked) span — its dimension tag + the
  // question/finding it hung on that sentence. Rendered INLINE, right after the
  // paragraph that owns the span (see the block loop below), so clicking a
  // highlight reveals the card next to the sentence — "inside the article" —
  // instead of far down at the article's foot. A range-only span with no
  // `block_ref` has no paragraph to anchor to and falls back to the foot.
  const activeCard = activeSpan ? (
    // The wrapper owns only placement (inline gap + the tour anchor); the card
    // body is either the caller's rich 透镜卡 recap (renderActiveCard) or the
    // built-in dimension+note box.
    <div data-tour="rr-inline-card" style={{ margin: "6px 0 14px" }}>
      {renderActiveCard ? (
        renderActiveCard(activeSpan)
      ) : (
        <div style={{ background: "var(--mk-accent-50)", border: "1px solid var(--mk-accent-200)", borderRadius: 12, padding: "13px 15px" }}>
          <div style={{ fontSize: 12, fontWeight: 700, color: "var(--mk-accent-700)", marginBottom: 6 }}>{activeSpan.tag}</div>
          <div style={{ fontSize: 13.5, lineHeight: 1.6, color: "var(--mk-secondary)" }}>{activeSpan.note}</div>
        </div>
      )}
    </div>
  ) : null;

  return (
    <div style={{ fontFamily: "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif" }}>
      {selectMode && (
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 10,
            marginBottom: 12,
            background: "var(--mk-accent-50)",
            border: "1px solid var(--mk-accent-200)",
            borderRadius: 10,
            padding: "9px 13px",
          }}
        >
          <span style={{ fontSize: 13, color: "var(--mk-accent-700)" }}>
            在文章里选出你要用来回答「{selectMode.dimension}」的那句话
          </span>
          <button
            type="button"
            onClick={selectMode.onCancel}
            style={{
              fontSize: 12.5,
              color: "var(--mk-faint)",
              background: "none",
              border: "none",
              cursor: "pointer",
              padding: "2px 4px",
            }}
          >
            取消
          </button>
        </div>
      )}
      <div onMouseUp={handleMouseUp}>
        {blocks.map((block) => {
          const runs = segmentBlock(block.id, block.text, state.spans);
          const referenced = Boolean(referencedBlockIds?.includes(block.id));
          return (
            <Fragment key={block.id}>
              <p
                data-block-id={block.id}
                onClick={
                  selectMode
                    ? (e) => pickSentence(block, e.clientX, e.clientY)
                    : referenceEnabled
                      ? () => {
                          // A drag-select just fired onReferenceSelection via
                          // mouse-up; don't ALSO toggle the whole paragraph.
                          const sel = window.getSelection();
                          if (sel && !sel.isCollapsed && sel.toString().trim()) return;
                          onReferenceBlock?.(block.id);
                        }
                      : undefined
                }
                style={{
                  fontSize: 15,
                  lineHeight: 2.1,
                  color: "var(--mk-ink)",
                  margin: "0 0 14px",
                  padding: referenced ? "2px 10px" : "2px 0",
                  borderLeft: referenced ? "3px solid var(--mk-accent)" : "3px solid transparent",
                  background: referenced ? "var(--mk-accent-50)" : "transparent",
                  borderRadius: referenced ? 4 : 0,
                  cursor: selectMode || referenceEnabled ? "pointer" : "default",
                  transition: "background 0.14s ease, border-color 0.14s ease",
                }}
              >
                {runs.map((run, i) => {
                  if (run.spanId == null) {
                    return <span key={i}>{run.text}</span>;
                  }
                  const tone = AUTHOR_MARK_STYLE[run.author ?? "ai"];
                  const active = run.spanId === activeSpanId;
                  return (
                    <mark
                      key={i}
                      data-tour={run.spanId === lensMarkSpanId ? "rr-lens-mark" : undefined}
                      onClick={(e) => {
                        // In select-mode a click anywhere — including on the
                        // AI's underlined example — picks the SENTENCE under the
                        // cursor, so the mark must NOT swallow it.
                        if (selectMode) {
                          // Stop the click from ALSO bubbling to the <p> onClick,
                          // which would fire pickSentence twice (two evaluate spends).
                          e.stopPropagation();
                          pickSentence(block, e.clientX, e.clientY);
                          return;
                        }
                        // Otherwise a mark click is its own action (open the
                        // span) and must not also toggle a block reference.
                        e.stopPropagation();
                        onSelectSpan(run.spanId);
                      }}
                      style={{
                        background: tone.background,
                        color: "var(--mk-ink)",
                        borderBottom: `2px solid ${tone.border}`,
                        borderRadius: 3,
                        padding: "1px 2px",
                        cursor: "pointer",
                        outline: active ? `2px solid ${tone.border}` : "none",
                        outlineOffset: 1,
                      }}
                    >
                      {run.text}
                    </mark>
                  );
                })}
              </p>
              {renderAfterBlock?.(block.id)}
              {activeSpan?.block_ref === block.id && activeCard}
            </Fragment>
          );
        })}
      </div>

      {/* Fallback for a range-only active span with no owning paragraph. */}
      {activeSpan && !activeSpan.block_ref && activeCard}
    </div>
  );
}
