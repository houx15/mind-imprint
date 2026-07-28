import { Fragment, type ReactNode } from "react";
import type { AnnotateState } from "@mind-imprint/contracts";
import { segmentBlock } from "./segment";
import { selectionToSpan, type CreatedSpan } from "./selection";

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
  /** Block ids currently referenced — rendered with a subtle highlight. */
  referencedBlockIds?: string[];
};

const AUTHOR_MARK_STYLE: Record<"ai" | "student" | "imported", { background: string; border: string }> = {
  ai: { background: "#F0ECF8", border: "#7C6BB5" },
  student: { background: "#EAF3EE", border: "#4E9A6F" },
  imported: { background: "#F3F0E8", border: "#B79A4C" },
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
  referencedBlockIds,
}: AnnotateProps) {
  const activeSpan = activeSpanId ? state.spans.find((s) => s.id === activeSpanId) ?? null : null;

  // Referencing (引用原文) is only offered outside evidence-pick mode — while
  // selectMode is active the article is for picking evidence, not for
  // building the conversation's referenced-sentence set.
  const referenceEnabled = Boolean(onReferenceBlock) && !selectMode;

  // In select-mode, picking is CLICK-to-pick-a-whole-sentence (matches the
  // reference demo): clicking any sentence selects that entire block as the
  // evidence. A drag still works too (partial selection) via onMouseUp.
  const pickBlock = (block: { id: string; text: string }) => {
    onCreateSpan?.({ blockId: block.id, start: 0, end: Array.from(block.text).length, text: block.text });
  };

  // Attached only when selectMode is set — with it absent, no handler exists
  // on the element and behavior is unchanged. A drag ends in a real text
  // selection; a plain click leaves it collapsed and is handled by the block
  // onClick (pickBlock) instead, so the two never double-fire.
  const handleMouseUp = selectMode
    ? () => {
        const span = selectionToSpan();
        if (span) onCreateSpan?.(span);
      }
    : undefined;

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
            background: "#F7F5FB",
            border: "1px solid #E3DCF2",
            borderRadius: 10,
            padding: "9px 13px",
          }}
        >
          <span style={{ fontSize: 13, color: "#5C4A8A" }}>
            在文章里选出你要用来回答「{selectMode.dimension}」的那句话
          </span>
          <button
            type="button"
            onClick={selectMode.onCancel}
            style={{
              fontSize: 12.5,
              color: "#8A90A3",
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
                    ? () => pickBlock(block)
                    : referenceEnabled
                      ? () => onReferenceBlock?.(block.id)
                      : undefined
                }
                style={{
                  fontSize: 15,
                  lineHeight: 2.1,
                  color: "#2B3346",
                  margin: "0 0 14px",
                  padding: referenced ? "2px 10px" : "2px 0",
                  borderLeft: referenced ? "3px solid #5C4A8A" : "3px solid transparent",
                  background: referenced ? "#F7F5FB" : "transparent",
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
                      onClick={(e) => {
                        // In select-mode a click anywhere in a sentence —
                        // including on the AI's underlined example — picks that
                        // whole sentence, so the mark must NOT swallow it.
                        if (selectMode) {
                          pickBlock(block);
                          return;
                        }
                        // Otherwise a mark click is its own action (open the
                        // span) and must not also toggle a block reference.
                        e.stopPropagation();
                        onSelectSpan(run.spanId);
                      }}
                      style={{
                        background: tone.background,
                        color: "#1C2333",
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
            </Fragment>
          );
        })}
      </div>

      {activeSpan && (
        <div style={{ marginTop: 14, background: "#F7F5FB", border: "1px solid #E3DCF2", borderRadius: 12, padding: "13px 15px" }}>
          <div style={{ fontSize: 12, fontWeight: 700, color: "#5C4A8A", marginBottom: 6 }}>{activeSpan.tag}</div>
          <div style={{ fontSize: 13.5, lineHeight: 1.6, color: "#3A4256" }}>{activeSpan.note}</div>
        </div>
      )}
    </div>
  );
}
