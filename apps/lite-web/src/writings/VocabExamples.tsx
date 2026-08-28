import type { WritingGuideMethod } from "../api/writingRoom";

/**
 * VocabExamples — the worked examples for the methods 印记 just named.
 *
 * This is the 要不要看几个例子 payoff: GuideBox names 1–3 methods in prose,
 * and if she asks to see them, this is what she gets — pre-authored examples
 * from vocab's registry (apps/api/internal/vocab), never a sentence written
 * about HER piece.
 *
 * The 铁律① property this component exists to guarantee: every example is
 * labelled with the `topic` it is actually about, right next to the text, so
 * an example about 两个菜市场 can never be read as a suggestion for her essay
 * about something else entirely. This is not a caption for decoration — drop
 * it and a borrowed example becomes indistinguishable from a written one.
 */
export function VocabExamples({ methods }: { methods: WritingGuideMethod[] }) {
  const withExamples = methods.filter((m) => m.examples.length > 0);
  if (withExamples.length === 0) return null;

  return (
    <div
      className="flex flex-col gap-3 rounded-mk-sm border p-3"
      style={{ borderColor: "var(--mk-border)", background: "var(--mk-paper)" }}
    >
      {withExamples.map((m) => (
        <div key={m.name} className="flex flex-col gap-1.5">
          <p className="text-mk-small font-semibold text-mk-ink">{m.name}是这样用的</p>
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
        </div>
      ))}
    </div>
  );
}
