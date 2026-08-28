import { useEffect, useRef, useState } from "react";
import { Button, Icon } from "@/ui";
import { BookOpen, GraduationCap } from "lucide-react";
import type { WritingBlockGuide, WritingGuideMethod } from "../api/writingRoom";
import { VocabExamples } from "./VocabExamples";

/**
 * GuideBox — the guiding box, shared by 结构 and 段落.
 *
 * It lives in both because the product asked for guidance at both moments:
 * *"AI should guide me to think about the outlines"* AND *"the key is the
 * AI-generated guiding box"* for the paragraphs. Those are the same
 * mechanism at two zoom levels — what is this block for, and then what goes
 * in it — so they are one component, not two that drift.
 *
 * ## Why this got rebuilt (Task 9)
 *
 * The product verdict on the old shape, verbatim: *"our snippets is not real
 * guidance, it is even not good as pro version"* and *"a large paragraph of
 * small texts is not easy to read."* The old box was a numbered list of
 * questions at 14px in a tinted panel — legible to nobody, and it taught
 * nothing because it never said WHY a block matters or WHAT method might do
 * the job. Four parts now, each doing one thing:
 *
 *   1. 这一段要做的事 — one line on the block's job, so she knows what she is
 *      aiming at before she is asked to think about it.
 *   2. 常见的几种写法 — real method names from vocab's library, named in
 *      prose with our one-line definitions. Naming a method and explaining it
 *      is teaching, not a picker — see the "no picker" note below.
 *   3. 想一想 — 2–4 questions, now at reading size (`text-mk-body-lg`,
 *      16px/1.75) with real breathing room between them, not stacked at
 *      chrome-sized 14px.
 *   4. 要不要看几个例子 — opens VocabExamples for the named methods, on her
 *      ask, never painted open by default.
 *
 * ## 铁律① IS STILL ENFORCED BY THE OUTPUT TYPE, NOT THE COMPONENT'S MANNERS
 *
 * `questions` is still the only field that could carry a sentence for her
 * essay, and it is still the only one the server hard-filters (every entry
 * must end in ？/?, writing_guide.go's parseWritingGuide). `job` is about the
 * BLOCK's task ("这一段要为读者做成什么事"), never her content — the same job
 * holds for any student writing this kind of block. `methods` are ids
 * resolved against vocab's registry (印记 selects and tunes, it never invents
 * a term) and their `examples` are pre-authored about topics no student is
 * writing on (VocabExamples' own guarantee). Nothing in here writes to any
 * field — there is no "use this" affordance, because there is nothing here
 * that could be used.
 *
 * ## No picker
 *
 * `methods` are named in flowing prose — a name in emphasis, a one-line
 * definition after it — never rendered as a grid of selectable tiles or a
 * radio group. That shape ("choose one to apply to your essay") was
 * explicitly killed in this product before; naming methods and showing
 * examples is teaching, choosing one for her is not.
 *
 * ## What is this method, explained on tap (F3, 2026-08-28; widened same day)
 *
 * `name` is what she reads — 举个例子, 比一比 — never essay-theory jargon.
 * Every method's name is tappable and opens a card answering "what is this,
 * really" — its definition at reading size, plus the same borrowed
 * example/pattern `VocabExamples` already renders. That is true even for
 * 最后提个建议 (no curriculum term) and 开门见山 (curriculum term identical to
 * the name she already reads) — a vague-sounding plain name is exactly the
 * kind of thing she'd want explained, term or no term.
 *
 * `formalName`, the curriculum word a 语文 teacher would use for the same
 * move, is a NARROWER thing than the card itself: one sentence inside it —
 * "在语文课上，这个叫「…」" — that appears only when `formalName` is
 * non-empty AND differs from `name` (the original, correct half of the
 * ruling; what was wrong the first time was gating the whole card on it).
 * For 留个悬念 → 留悬念, one character apart, that sentence still shows: the
 * canonical form is what a teacher would write on the board, and a
 * one-character difference is still a difference. For 最后提个建议 and
 * 开门见山 the sentence is simply absent — the card still opens, it just has
 * nothing further to name.
 */
export function GuideBox({
  guide,
  onDismiss,
  onDeepen,
}: {
  guide: WritingBlockGuide;
  onDismiss: () => void;
  /** 深入一层 — opens `DeepenDrawer` on this block (the same 印记, scoped to
   *  one block). This component owns only the button and the callback. */
  onDeepen: () => void;
}) {
  const [showExamples, setShowExamples] = useState(false);
  // Index into guide.methods of the card currently open, or null. Index
  // rather than name — two methods could in principle share a display name,
  // and this keeps "which one is open" unambiguous either way.
  const [openTermIndex, setOpenTermIndex] = useState<number | null>(null);
  const hasMethods = guide.methods.length > 0;
  // Patterns count as something to show. The English methods in vocab carry
  // sentence FRAMES instead of worked examples, so gating this button on
  // `examples` alone hid 例子 from every English block that had frames to
  // teach — which was most of them.
  const hasIllustrations = guide.methods.some((m) => m.examples.length > 0 || m.patterns.length > 0);

  return (
    <div
      className="flex flex-col gap-4 rounded-mk-md border p-4"
      style={{ borderColor: "var(--mk-accent-300)", background: "color-mix(in srgb, var(--mk-accent-500) 5%, var(--mk-paper))" }}
    >
      <div className="flex items-center justify-between gap-2">
        <span
          className="rounded-mk-full px-2 py-0.5 text-mk-label font-semibold"
          style={{ background: "var(--mk-accent-100)", color: "var(--mk-accent-700)" }}
        >
          写作引导
        </span>
        <button type="button" onClick={onDismiss} className="text-mk-small text-mk-muted hover:text-mk-ink">
          收起
        </button>
      </div>

      {guide.job.trim() && (
        <section className="flex flex-col gap-1">
          <h3 className="text-mk-label font-semibold text-mk-accent-700">这一段要做的事</h3>
          <p className="text-mk-body text-mk-ink">{guide.job}</p>
        </section>
      )}

      {hasMethods && (
        <section className="flex flex-col gap-1.5">
          <h3 className="text-mk-label font-semibold text-mk-accent-700">常见的几种写法</h3>
          <div className="flex flex-col gap-1.5">
            {guide.methods.map((m, i) => {
              const open = openTermIndex === i;
              return (
                <div key={`${m.name}-${i}`} className="flex flex-col gap-1.5">
                  <p className="text-mk-body">
                    <button
                      type="button"
                      onClick={() => setOpenTermIndex(open ? null : i)}
                      aria-haspopup="dialog"
                      aria-expanded={open}
                      className="inline-flex items-center gap-1 font-semibold underline decoration-dotted underline-offset-4 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                      style={{ color: "var(--mk-accent-700)" }}
                    >
                      {m.name}
                      <Icon icon={GraduationCap} size={13} />
                    </button>
                    <span className="text-mk-muted">：{m.definition}</span>
                  </p>
                  {open && <FormalTermCard method={m} onClose={() => setOpenTermIndex(null)} />}
                </div>
              );
            })}
          </div>
        </section>
      )}

      {guide.questions.length > 0 && (
        <section className="flex flex-col gap-1.5">
          <h3 className="text-mk-label font-semibold text-mk-accent-700">想一想</h3>
          <ol className="flex list-none flex-col gap-3">
            {guide.questions.map((q, i) => (
              <li key={i} className="flex gap-2.5">
                <span
                  className="mt-[3px] flex h-5 w-5 shrink-0 items-center justify-center rounded-mk-full text-mk-label"
                  style={{ background: "var(--mk-accent-100)", color: "var(--mk-accent-700)" }}
                >
                  {i + 1}
                </span>
                <span className="text-mk-body-lg text-mk-ink">{q}</span>
              </li>
            ))}
          </ol>
        </section>
      )}

      <div className="flex flex-wrap items-center gap-2 pt-1">
        {hasIllustrations && (
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setShowExamples((v) => !v)}
            iconStart={<Icon icon={BookOpen} size={14} />}
          >
            {showExamples ? "收起例子" : "要不要看几个例子"}
          </Button>
        )}
        <Button variant="secondary" size="sm" onClick={onDeepen}>
          深入一层
        </Button>
      </div>

      {showExamples && hasIllustrations && <VocabExamples methods={guide.methods} />}
    </div>
  );
}

/**
 * FormalTermCard — "what is this method, really" for one method (F3;
 * widened same day from "the 语文课 term card" — see the file-level doc).
 *
 * ALWAYS shows the definition (reading size, `text-mk-body-lg` — matching
 * 想一想 above, not the 11px `mk-label` chip size called unreadable for
 * content in this room) and the borrowed example/pattern (`VocabExamples`
 * itself, scoped to just this one method — one renderer for "here is a
 * worked example" in this file, not two).
 *
 * The "在语文课上，这个叫「…」" sentence is the ONE part that is conditional
 * — only when `method.formalName` is non-empty and differs from
 * `method.name`. A teacher offering a word, not a glossary entry: it anchors
 * to the plain name she already read rather than leading with jargon.
 *
 * Nothing here writes anywhere. There is no "apply this" button, no
 * checkbox, no selectable state — the card explains, it does not choose. It
 * is dismissible three ways, matching how the room's other small overlays
 * behave (`BlockToolbar`, `DeepenDrawer`): Escape, a click outside it, or
 * re-tapping the name that opened it (handled by the parent's toggle).
 */
function FormalTermCard({ method, onClose }: { method: WritingGuideMethod; onClose: () => void }) {
  const cardRef = useRef<HTMLDivElement | null>(null);
  const hasTerm = method.formalName.trim() !== "" && method.formalName !== method.name;

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    function onDown(e: PointerEvent) {
      const card = cardRef.current;
      if (!card) return;
      const target = e.target as Node | null;
      if (target && card.contains(target)) return;
      onClose();
    }
    window.addEventListener("keydown", onKey);
    window.addEventListener("pointerdown", onDown, true);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("pointerdown", onDown, true);
    };
  }, [onClose]);

  return (
    <div
      ref={cardRef}
      role="dialog"
      aria-label={hasTerm ? `语文课上，这个叫「${method.formalName}」` : `${method.name}是什么`}
      className="ml-1 flex flex-col gap-2 rounded-mk-md border p-3"
      style={{
        borderColor: "var(--mk-accent-300)",
        background: "color-mix(in srgb, var(--mk-accent-500) 8%, var(--mk-paper))",
      }}
    >
      <div className="flex items-start justify-between gap-2">
        <span
          className="inline-flex w-fit items-center rounded-mk-full px-2 py-0.5 text-mk-label font-semibold"
          style={{ background: "var(--mk-accent-100)", color: "var(--mk-accent-700)" }}
        >
          {hasTerm ? "语文课怎么说" : "这是什么"}
        </span>
        <button
          type="button"
          onClick={onClose}
          className="text-mk-small text-mk-muted hover:text-mk-ink"
        >
          知道了
        </button>
      </div>
      <p className="text-mk-body-lg text-mk-ink">
        {/* Conditional half of the card — only when there IS a curriculum term
            AND it differs from the plain name. Everything else on this card
            (this paragraph's definition, and VocabExamples below) is
            unconditional: every method gets explained, the term is just one
            more thing some of them have. */}
        {hasTerm && (
          <>
            在语文课上，这个叫「<span className="font-semibold">{method.formalName}</span>」。{" "}
          </>
        )}
        {method.definition}
      </p>
      <VocabExamples methods={[method]} />
    </div>
  );
}
