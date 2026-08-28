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
    <div className="flex flex-col gap-3">
      <h2 className="text-mk-label text-mk-faint">读完这篇，还能往下想</h2>
      <div className="flex flex-col gap-3">
        {questions.map((q) => (
          <div
            key={q.id}
            className="rounded-mk-md border border-mk-border bg-mk-surface p-4 shadow-mk-sm"
          >
            <p className="text-mk-body-lg text-mk-ink">{q.text}</p>
            <p className="mt-2 text-mk-small text-mk-muted">从这句想到的：{q.anchorQuote}</p>
            <div className="mt-3">
              <Button variant="secondary" onClick={() => writeAbout(q.text)}>
                <Icon icon={PenLine} size={14} />
                去写一写
              </Button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

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
