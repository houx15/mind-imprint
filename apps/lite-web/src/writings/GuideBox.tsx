import { useState } from "react";
import { Button, Icon } from "@/ui";
import { BookOpen } from "lucide-react";
import type { WritingBlockGuide } from "../api/writingRoom";
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
 */
export function GuideBox({
  guide,
  onDismiss,
  onDeepen,
}: {
  guide: WritingBlockGuide;
  onDismiss: () => void;
  /** 深入一层 — opens a fuller drawer on this block. The drawer itself is a
   *  later task; this component only owns the button and the callback. */
  onDeepen: () => void;
}) {
  const [showExamples, setShowExamples] = useState(false);
  const hasMethods = guide.methods.length > 0;
  const hasExamples = guide.methods.some((m) => m.examples.length > 0);

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
            {guide.methods.map((m) => (
              <p key={m.name} className="text-mk-body">
                <span className="font-semibold" style={{ color: "var(--mk-accent-700)" }}>
                  {m.name}
                </span>
                <span className="text-mk-muted">：{m.definition}</span>
              </p>
            ))}
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
        {hasExamples && (
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

      {showExamples && hasExamples && <VocabExamples methods={guide.methods} />}
    </div>
  );
}
