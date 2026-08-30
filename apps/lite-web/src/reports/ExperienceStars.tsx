import { useState } from "react";
import { putReadingRating } from "../api/readingRoom";

/**
 * ExperienceStars — the five stars at the foot of the report.
 *
 *   > let students give a star (one to five) for his reading experience
 *   > […] at the bottom of the report we need to add a five star scorer
 *
 * 🚨 **Read the direction before touching this.** 铁律② forbids scoring HER —
 * no grade, no streak, no leaderboard. This points the other way: **she is
 * scoring US**. It never enters the evaluation, never reaches a model, never
 * appears on the public share payload, and nothing she can do here changes
 * anything about her own record. The copy has to keep saying that out loud,
 * because a row of stars under a report about her work is exactly the shape a
 * grade would take — 「这次阅读，你觉得怎么样？」 asks about the experience,
 * where 「给这次阅读打个分」 would ask about her.
 *
 * Three smaller decisions:
 *
 *  - **No submit button.** Tapping a star IS the answer; a second tap on a
 *    different star changes it. Asking her to confirm feedback she gave in one
 *    gesture is the kind of form this whole change removed.
 *  - **Optimistic, and honest when it fails.** The stars fill immediately, and
 *    if the write fails they roll back with a line saying so — a star that
 *    looks saved and isn't is worse than no stars.
 *  - **`null` is not zero.** An unanswered scorer shows five empty stars and
 *    no thank-you; a one-star answer shows one filled star. They must never
 *    render the same.
 */
export function ExperienceStars({ atomId, initial }: { atomId: string; initial: number | null }) {
  const [rating, setRating] = useState<number | null>(initial);
  const [hover, setHover] = useState(0);
  const [failed, setFailed] = useState(false);
  const [saving, setSaving] = useState(false);

  async function give(n: number) {
    if (saving) return;
    const previous = rating;
    setRating(n);
    setFailed(false);
    setSaving(true);
    try {
      await putReadingRating(atomId, n);
    } catch {
      setRating(previous);
      setFailed(true);
    } finally {
      setSaving(false);
    }
  }

  // `hover` wins while the pointer is on the row so the stars preview the
  // answer she is about to give; otherwise the stored answer shows.
  const shown = hover || rating || 0;

  return (
    <section
      className="rounded-mk-md border border-mk-border bg-mk-surface px-5 py-4"
      aria-labelledby="mk-stars-title"
    >
      <h3 id="mk-stars-title" className="text-mk-body font-semibold text-mk-ink">
        这次阅读，你觉得怎么样？
      </h3>
      <p className="mt-1 text-mk-small text-mk-muted">
        你在评的是这次带读，不是你自己。说给我们听，好让下一次更好。
      </p>

      <div
        className="mt-3 flex items-center gap-1"
        onPointerLeave={() => setHover(0)}
        role="radiogroup"
        aria-label="这次阅读的体验"
      >
        {[1, 2, 3, 4, 5].map((n) => (
          <button
            key={n}
            type="button"
            role="radio"
            aria-checked={rating === n}
            aria-label={`${n} 星`}
            className="mk-stars__btn"
            onPointerEnter={() => setHover(n)}
            onFocus={() => setHover(n)}
            onBlur={() => setHover(0)}
            onClick={() => void give(n)}
          >
            <Star filled={n <= shown} />
          </button>
        ))}

        {rating !== null && !failed && (
          <span className="ml-2 text-mk-small text-mk-muted">谢谢你告诉我们。</span>
        )}
      </div>

      {failed && (
        <p className="mt-2 text-mk-small text-mk-danger">这一下没存上，再点一次试试。</p>
      )}
    </section>
  );
}

function Star({ filled }: { filled: boolean }) {
  return (
    <svg width="26" height="26" viewBox="0 0 24 24" aria-hidden="true" className="mk-stars__icon">
      <path
        d="M12 3.6l2.47 5.01 5.53.8-4 3.9.94 5.5L12 16.2l-4.94 2.6.94-5.5-4-3.9 5.53-.8z"
        fill={filled ? "var(--mk-accent-500)" : "transparent"}
        stroke={filled ? "var(--mk-accent-500)" : "var(--mk-border)"}
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
    </svg>
  );
}
