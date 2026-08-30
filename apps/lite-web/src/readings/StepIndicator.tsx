import type { ReadingTask } from "../api/readingRoom";

/**
 * StepIndicator — 「我现在在第几步」, at the top of 印记's column.
 *
 * Removed on 2026-08-30 when `ReadingPlanDial` folded the plan into a corner,
 * and asked back the same day:
 *
 *   > and I want the top "current step" row
 *
 * So the room now carries two step surfaces on purpose, and they answer
 * different questions. **The dial is the plan** — every step, how far along,
 * one hover away. **This row is the present tense** — the one step she is on,
 * always visible, never needing a hover. Splitting them is also what keeps
 * either from being a wall: the row never shows the step's own instruction
 * (印记 just said it in the message below) and never shows the other steps.
 *
 * Three things it is careful about:
 *
 *  - **Current step = the first `pending` one.** Same rule as the server's
 *    `currentReadingTask` (`reading_coach.go`) and as the dial, so the two
 *    surfaces can never point at different steps.
 *  - **No second progress meter.** It used to carry a filled track; the dial's
 *    ring is that now. Two meters saying one thing is how 「0 / 6」 started
 *    reading as a score in the first place.
 *  - **It is a position, never a score** (铁律②). 第 N 步 / 共 M 步 says where
 *    she is in a plan. No percentage, no ✓/✗, no streak — and nothing here is
 *    clickable, because 印记 moves her between steps.
 *
 * Motion lives in `index.css` as `.mk-stepnow*` (130ms /
 * cubic-bezier(0.2, 0, 0, 1), the same vocabulary as `.mk-blockbar` and
 * `.mk-plandial`) precisely so `prefers-reduced-motion: reduce` can switch it
 * off — an inline `animation:` could not be reduced away.
 */
export function StepIndicator({ tasks }: { tasks: ReadingTask[] }) {
  // Before 印记 has planned, there is no position to report. A 「第 0 步 /
  // 共 0 步」 row, or an empty box where one will be, is worse than the quiet.
  if (tasks.length === 0) return null;

  const index = tasks.findIndex((t) => t.status === "pending");
  const current = index === -1 ? null : tasks[index]!;

  return (
    <div
      className="mk-stepnow flex flex-none items-baseline gap-2.5 rounded-mk-md px-3.5 py-2.5"
      style={{
        background: "color-mix(in srgb, var(--mk-accent-500) 7%, var(--mk-surface))",
        border: "1px solid color-mix(in srgb, var(--mk-accent-500) 18%, transparent)",
      }}
      role="status"
      aria-live="polite"
    >
      {current && <span className="mk-stepnow__dot" aria-hidden="true" />}
      <span
        className="flex-none text-mk-small font-medium tabular-nums"
        style={{ color: current ? "var(--mk-accent-700)" : "var(--mk-muted)" }}
      >
        {current ? `第 ${index + 1} 步 / 共 ${tasks.length} 步` : `带读走完了 · 共 ${tasks.length} 步`}
      </span>

      {current && (
        // Keyed on the step id so React mounts a NEW node when 印记 moves her
        // on — that remount is what replays the CSS animation. Swapping the
        // text of a reused node would change the sentence silently.
        <p key={current.id} className="mk-stepnow__label min-w-0 flex-1 truncate text-mk-body text-mk-ink">
          {current.label}
        </p>
      )}
    </div>
  );
}
