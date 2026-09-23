import type { WritingGuideMethod } from "../api/writingRoom";

/**
 * VocabExamples — the worked examples for the methods 印记 just named.
 *
 * This is the 要不要看几个例子 payoff: GuideBox names 1–3 methods in prose,
 * and if she asks to see them, this is what she gets — pre-authored examples
 * from vocab's registry (apps/api/internal/vocab), never a sentence written
 * about HER piece.
 *
 * The 铁律① property this component exists to guarantee for the METHOD-level
 * examples: every one of them is labelled with the `topic` it is actually
 * about, right next to the text, so an example about 两个菜市场 can never be
 * read as a suggestion for her essay about something else entirely. That
 * caption is not decoration — drop it and a borrowed example becomes
 * indistinguishable from a written one. Keep it on `m.examples`.
 *
 * The PATTERN-level examples below do NOT carry that caption. They render as a
 * bare 例句：, by product ruling 2026-09-23 (「just say 例句： is ok」), and
 * `Pattern` has no `topic` field to render even if we wanted one. What stands
 * in for the caption there is the frame printed directly above the sentence:
 * the student reads the skeleton with its blanks first, so the sentence below
 * it is visibly one filled-in instance of that skeleton, not advice about her
 * piece. The barrier that keeps a pattern example off her topic is upstream
 * and editorial — the avoid-list in the `Pattern` doc comment in
 * apps/api/internal/vocab/vocab.go. Do not add the caption back here.
 *
 * ## 句式 (Task 11)
 *
 * The English methods in the library (`en_concession`, `en_qualify`,
 * `en_evidence`) carry `patterns` rather than prose examples — sentence
 * frames with blanks, e.g. *"While it is true that ___, this does not mean
 * ___."* Nothing rendered them until now, which meant a student writing in
 * English opened 例子 and got an empty panel from methods that had plenty to
 * teach her. They are shown here beside the examples.
 *
 * A frame is safe under 铁律① for a structural reason, not a stylistic one:
 * the blanks are where the content goes, and the content is the part we will
 * not write. She still has to know what ___ is; the frame only tells her what
 * shape the sentence takes. That is why the blanks are never pre-filled and
 * why there is no copy-into-my-paragraph button here.
 */
export function VocabExamples({ methods }: { methods: WritingGuideMethod[] }) {
  const shown = methods.filter((m) => m.examples.length > 0 || m.patterns.length > 0);
  if (shown.length === 0) return null;

  return (
    <div
      className="flex flex-col gap-3 rounded-mk-sm border p-3"
      style={{ borderColor: "var(--mk-border)", background: "var(--mk-paper)" }}
    >
      {shown.map((m) => (
        <div key={m.name} className="flex flex-col gap-1.5">
          <p className="text-mk-small font-semibold text-mk-ink">{m.name}是这样用的</p>
          {m.examples.length > 0 && (
            <div className="flex flex-col gap-2">
              {m.examples.map((ex, i) => (
                <div key={i} className="flex flex-col gap-0.5 border-l-2 pl-2.5" style={{ borderColor: "var(--mk-accent-300)" }}>
                  {/* The topic label IS the 铁律① guarantee — never drop it, and
                      never let it merge into the example text itself. */}
                  <span className="text-mk-small text-mk-muted">这一段说的是「{ex.topic}」，不是你这篇的内容：</span>
                  <p className="text-mk-body text-mk-ink">{ex.text}</p>
                </div>
              ))}
            </div>
          )}
          {m.patterns.length > 0 && (
            <div className="flex flex-col gap-2">
              <span className="text-mk-small text-mk-muted">常用的句式（横线上的内容要你自己填）：</span>
              {m.patterns.map((p, i) => (
                <div key={i} className="flex flex-col gap-0.5 border-l-2 pl-2.5" style={{ borderColor: "var(--mk-accent-300)" }}>
                  <span className="text-mk-small text-mk-muted">{p.label}</span>
                  <p className="text-mk-body text-mk-ink">{p.frame}</p>
                  {p.gloss && (
                    <p className="text-mk-small text-mk-muted">{p.gloss}</p>
                  )}
                  {p.example && (
                    <p className="text-mk-body text-mk-ink">例句：{p.example}</p>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
