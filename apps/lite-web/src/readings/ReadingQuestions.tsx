import { useEffect, useState } from "react";
import { PenLine } from "lucide-react";
import { Button, Icon } from "@/ui";
import { useAlive } from "../shared/useAlive";
import { getReadingQuestions, type ReadingQuestion } from "../api/readingRoom";
import { liteRoutePath, navigate } from "../routing";

/**
 * ReadingQuestions — what a finished reading leaves her with besides a full
 * stop.
 *
 * The product ruling this exists for, verbatim: *"just a suggestion, but
 * suggestion is very important, some interesting questions would grow from
 * this reading — why xxxx, what is xxx, how people view xxx."* A suggestion,
 * not a pre-seeded writing project — so 去写一写 carries ONLY the question
 * text into 写作, never the article, her notes, or a plan. She starts fresh,
 * with a good question in hand.
 *
 * Each question is anchored to the sentence in the article that provoked
 * it (Task 8's `anchorQuote`), because "here's a question" is a much weaker
 * offer than "here's a question AND the line that raised it" — the anchor is
 * what makes it feel grown from THIS reading rather than generic.
 *
 * An empty list is a designed outcome (the article was too thin to grow at
 * least two anchored questions), not an error — so this renders nothing at
 * all rather than an empty-state apology. A fetch failure is treated the
 * same way: this is a suggestion, and a broken suggestion box under a
 * finished reading would be a worse experience than no box.
 */
export function ReadingQuestions({ readingId }: { readingId: string }) {
  const [questions, setQuestions] = useState<ReadingQuestion[] | null>(null);
  const alive = useAlive();

  useEffect(() => {
    setQuestions(null);
    getReadingQuestions(readingId)
      .then((qs) => {
        if (alive.current) setQuestions(qs);
      })
      .catch(() => {
        // A suggestion that fails to load is just no suggestion — never an
        // error banner under a finished reading.
        if (alive.current) setQuestions([]);
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [readingId]);

  if (!questions || questions.length === 0) return null;

  return (
    // Bubbles, at the size of an invitation.
    //
    // This section used to be an 11px label over 16px cards in a single
    // stacked column — "the text fonts is too small […] several bubbles
    // jumping". So: a real heading, question text at the report's own
    // pull-quote size, macaron fill per card, a speech-bubble tail, and each
    // card bobbing on its own phase (`--i`, read by `.mk-rq-bubble` in
    // index.css — in unison they would read as the page loading, not as
    // bubbles). Reduced motion stops all of it.
    <div className="flex flex-col gap-4 pt-2">
      <h2 className="text-mk-h1 text-mk-ink">读完这篇，还能往下想</h2>
      <div className="mk-rq-bubbles">
        {questions.map((q, i) => {
          const { bg, fg } = MACARON[i % MACARON.length] ?? MACARON[0]!;
          return (
            <div
              key={q.id}
              className="mk-rq-bubble p-6"
              style={{ background: bg, ["--i" as string]: i } as React.CSSProperties}
            >
              <p className="text-mk-report-quote text-mk-ink">{q.text}</p>
              <p className="mt-3 text-mk-body text-mk-muted">从这句想到的：{q.anchorQuote}</p>
              <div className="mt-4">
                <Button variant="secondary" onClick={() => writeAbout(q.text)}>
                  <Icon icon={PenLine} size={15} />
                  去写一写
                </Button>
              </div>
              <span className="mk-rq-bubble__tail" aria-hidden="true" style={{ background: bg }} />
            </div>
          );
        })}
      </div>
    </div>
  );
}

/** The same macaron cycle the report uses, so a finished reading reads as one
 *  page rather than two designs meeting at a seam. Bare `var(--mk-…)` in an
 *  inline style, never a Tailwind alpha class — `mk-*` are bare custom
 *  properties and `bg-mk-peach-bg/40` emits no CSS at all. */
const MACARON = [
  { bg: "var(--mk-lake-bg)", fg: "var(--mk-lake-fg)" },
  { bg: "var(--mk-butter-bg)", fg: "var(--mk-butter-fg)" },
  { bg: "var(--mk-taro-bg)", fg: "var(--mk-taro-fg)" },
  { bg: "var(--mk-peach-bg)", fg: "var(--mk-peach-fg)" },
] as const;

/** Read-and-clear on the other side (`WritingsLanding`) — this key is a
 *  one-shot handoff for THIS arrival, not a standing preference. */
export const WRITING_IDEA_KEY = "lite:writing-idea";

function writeAbout(text: string) {
  try {
    sessionStorage.setItem(WRITING_IDEA_KEY, text);
  } catch {
    // Private mode / storage disabled: the navigation still helps her, she
    // just retypes the question. Never let a storage failure eat the click.
  }
  navigate(liteRoutePath({ tab: "writings" }));
}
