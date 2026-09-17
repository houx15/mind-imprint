import { Fragment, type ReactNode } from "react";
import type { AnnotateState } from "@mind-imprint/contracts";
import { parseMarkdownTable } from "./markdownTable";
import { segmentBlock } from "./segment";
import { splitByKeywords } from "./keywords";
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
   * Block ids that are section headings rather than prose (轻量版的分级阅读库
   * 用它，见 apps/api/internal/library)。
   *
   * They stay `<p data-block-id>` on purpose: annotation offsets, tool-card
   * anchors and sentence segmentation all key off that element, and swapping
   * the tag for an `<h2>` would quietly change what a span can be made
   * against. Only the type and the role change. Absent by default — output is
   * byte-identical to before this prop existed when it is not supplied.
   */
  headingBlockIds?: string[];
  /**
   * Block ids the coach judged to be load-bearing — the paragraphs that carry
   * the argument rather than support it (轻量版的导读，见
   * `apps/api/internal/api/reading_outline.go`).
   *
   * Renders as `data-core` and NOTHING else: the styling is one line of
   * lite's own CSS colouring the `3px solid transparent` left border this
   * component already puts on every paragraph, so there is zero layout
   * change. Absent by default — output is byte-identical to before this prop
   * existed when it is not supplied, and pro never passes it.
   */
  coreBlockIds?: string[];
  /**
   * 她**此刻正在读的那几段** —— 通读走到哪一部分，那一部分的段落就是它。
   *
   * 🚨 产品负责人 2026-09-17：「insert this into the paragraphs. namely the
   * article, during 通读stage, is becoming several parts, with each parts a
   * guidance, so students are able to read a long article patiently.」
   * 一篇十八段的文章摊在她面前，她不知道这一步要读到哪儿为止 —— 步骤名里写着
   * 「通读第1–4段」，而文章那一栏上一个记号都没有。
   *
   * 和 coreBlockIds 一样只渲染成 `data-active`，样式全在轻量版自己的 CSS 里，
   * 不传时输出逐字节不变，pro 从不传它。
   */
  activeBlockIds?: string[];
  /**
   * 某一段**之前**要摆的一行小字，`blockId → 那句话`。
   *
   * 通读到某一部分时，这一部分的第一段前面出现一行「现在读这一部分 · 开篇与
   * 类比」。产品负责人：「or even better, sometimes ai will ask questions part
   * by part. then for each part, during that part, highlights those
   * paragraphs, with a small guidance above them.」
   *
   * 🚨 它是**段落之外**的一个元素，不进 block.text —— 标注的锚点是段落文本里的
   * 偏移，往正文里插一个字，之前存下来的每一条标注就都错位了（data-n 那个段号
   * 用 CSS 画出来也是这个理由）。
   *
   * 不传时输出逐字节不变。
   */
  blockLead?: Record<string, string>;
  /**
   * 她在这一段上开过的那几件工具的名字，`blockId → ["翻译", "语法"]`。
   * 画成一行小字跟在那一段**后面**。
   *
   * 🚨 产品负责人 2026-09-17：「once students clicked one thing, can we reveal
   * that in the paragraph? like tags after the texts? … should not be overlap
   * with text, should not be too highlighted. just indicate that students can
   * view these things later.」
   *
   * 两条约束直接决定了它长什么样：**跟在后面**（不压正文，所以是自己的一行，
   * 不是绝对定位的角标），**不抢眼**（样式全在轻量版的 CSS 里，比正文小、比
   * 正文淡）。它不是按钮 —— 段落工具条点一下段落就出来，这一行只负责说
   * 「这儿有东西可以回去看」。
   *
   * 和 blockLead 同一条纪律：段落**之外**的元素，不进 block.text。
   * 不传时输出逐字节不变。
   */
  blockMarks?: Record<string, string[]>;
  /**
   * 荧光笔：每一段里要标出来的关键词，`blockId → terms`（轻量版的关键单词，
   * 见 `apps/lite-web/src/readings/WordCards.tsx`）。
   *
   * 服务端已经保证每个词逐字出现在它那一段里，所以这里只做逐字查找，不猜。
   * 标出来的是 `<span data-kw>`，**不是 `<mark>`** —— `<mark>` 在这个组件里
   * 已经是「一条标注」的意思，两者混在一起，她点一个关键词会以为自己点开了
   * 一条批注。样式由轻量版自己的 CSS 给。
   *
   * 🚨 切出来的段落拼回去逐字等于原文：标注的锚点是这一段文本里的偏移，
   * 正文里多一个或少一个字符，之前存下来的每一条标注就都错位了。
   *
   * 不传时输出与这个 prop 存在之前逐字节相同，pro 从不传它。
   */
  keywordTerms?: Record<string, string[]>;
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
  /**
   * 她在正文里划选了一段。`at` 是这次划选在屏幕上的位置（viewport 坐标，
   * 选区的下边缘中点），房间据此把工具条摆在她划的那几个字底下。
   */
  onReferenceSelection?: (blockId: string, quote: string, at?: { x: number; y: number }) => void;
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
  headingBlockIds,
  coreBlockIds,
  activeBlockIds,
  blockLead,
  blockMarks,
  keywordTerms,
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
          if (!span || !span.text.trim()) return;
          // 🚨 **不要清掉选区。** 这里原来跟着一句 `removeAllRanges()`，于是她
          // 划完一个词，那几个字当场不再高亮 —— 产品负责人 2026-09-18 逐字报的
          // 「when I select a word or a sentence, my selection disappears and
          // cannot ask a word's meaning」。划选是一个**还没说完的动作**：她划出
          // 那个词，是要对它做点什么（查词、语法），而选区正是「对哪几个字」的
          // 唯一记录。清掉它，她面前就只剩一条引用，和一个没人接的意图。
          //
          // 位置也要带出去：工具条得摆在她划的那几个字旁边，不是屏幕某处。
          const sel = window.getSelection();
          let at: { x: number; y: number } | undefined;
          const rect = sel && sel.rangeCount > 0 ? sel.getRangeAt(0).getBoundingClientRect() : null;
          if (rect && (rect.width > 0 || rect.height > 0)) {
            at = { x: rect.left + rect.width / 2, y: rect.bottom };
          }
          onReferenceSelection(span.blockId, span.text.trim(), at);
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
        {blocks.map((block, blockIndex) => {
          // 🚨 表格块单独画。见 markdownTable.ts：标注的锚点是这一段文本里的
          // 字节偏移，把一段文字拆成单元格就对不上了，所以表格不进
          // `<p data-block-id>` 那条路，其余段落一个字都不变。
          const table = parseMarkdownTable(block.text);
          if (table) {
            return (
              <div
                key={block.id}
                data-block-id={block.id}
                data-table=""
                data-n={blockIndex + 1}
                className="mk-article-table"
              >
                <table>
                  <thead>
                    <tr>
                      {table.header.map((h, i) => (
                        <th key={i}>{h}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {table.rows.map((row, r) => (
                      <tr key={r}>
                        {row.map((cell, c) => (
                          <td key={c}>{cell}</td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            );
          }
          const runs = segmentBlock(block.id, block.text, state.spans);
          const referenced = Boolean(referencedBlockIds?.includes(block.id));
          const heading = Boolean(headingBlockIds?.includes(block.id));
          const core = Boolean(coreBlockIds?.includes(block.id));
          const active = Boolean(activeBlockIds?.includes(block.id));
          const lead = blockLead?.[block.id];
          const marks = blockMarks?.[block.id];
          return (
            <Fragment key={block.id}>
              {lead && (
                <p data-block-lead className="mk-block-lead">
                  {lead}
                </p>
              )}
              <p
                data-block-id={block.id}
                // 段号。🚨 用属性而不是往正文里插字：标注的锚点是这一段文本里
                // 的字节偏移，正文里多一个字符，之前存下来的每一条标注就都错位。
                // 数字由 CSS ::before 画出来（见 index.css）。
                data-n={blockIndex + 1}
                data-heading={heading ? "" : undefined}
                data-core={core ? "" : undefined}
                data-active={active ? "" : undefined}
                data-referenced={referenced ? "" : undefined}
                role={heading ? "heading" : undefined}
                aria-level={heading ? 2 : undefined}
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
                  fontSize: heading ? 18 : 15,
                  fontWeight: heading ? 600 : undefined,
                  lineHeight: heading ? 1.5 : 2.1,
                  color: "var(--mk-ink)",
                  margin: heading ? "26px 0 10px" : "0 0 14px",
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
                    // 荧光笔只落在**没有标注**的那些段落上。一段文字同时是
                    // 一条标注又是一个关键词时，标注赢 —— 标注是她自己（或
                    // 印记）在这篇文章上做过的事，关键词只是一个词表。
                    const terms = keywordTerms?.[block.id];
                    if (!terms || terms.length === 0) {
                      return <span key={i}>{run.text}</span>;
                    }
                    return (
                      <span key={i}>
                        {splitByKeywords(run.text, terms).map((kw, j) =>
                          kw.term == null ? (
                            <span key={j}>{kw.text}</span>
                          ) : (
                            <span key={j} data-kw="">
                              {kw.text}
                            </span>
                          ),
                        )}
                      </span>
                    );
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
              {marks && marks.length > 0 && (
                <p data-block-marks className="mk-block-marks">
                  {marks.map((m, i) => (
                    <span key={m}>
                      {i > 0 && <span aria-hidden="true"> · </span>}
                      {m}
                    </span>
                  ))}
                </p>
              )}
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
