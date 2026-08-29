import type { ReadingTask } from "../api/readingRoom";

/**
 * StepIndicator — 「我现在在第几步」, at the top of the coach column.
 *
 * This is the slot pro fills with 「你读这篇是为了：点击填写…」. Lite's fork of
 * the room deliberately did not copy that bar (in lite it was write-only: it
 * fed only the room composer's own `readTurn` prompt, which 带读 replaced), and
 * the ask that named what belongs there instead was:
 *
 *   > change this: 你读这篇是为了：点击填写… to an active current step, which
 *   > has some animation.
 *
 * Three things it is careful about:
 *
 *  - **Current step = the first `pending` one.** Same rule as the server's
 *    `currentReadingTask` (`reading_coach.go`) and the same one
 *    `ReadingPlanRail` derives, so the bar and the rail can never disagree.
 *  - **It is the first step surface on a narrow screen.** `ReadingPlanRail`
 *    hangs in an `lg:`-gated aside (`ReadingRoomHost`), so below that
 *    breakpoint there was no step surface at all. Nothing in here may be
 *    responsively hidden.
 *  - **It is a position, never a score** (铁律②). 第 N 步 / 共 M 步 says where
 *    she is in a plan. No percentage in words, no ✓/✗, no streak, no grade —
 *    and nothing here is clickable, because 印记 moves her between steps.
 *
 * Motion lives in `index.css` as `.mk-stepnow*` (130ms /
 * cubic-bezier(0.2, 0, 0, 1), the same vocabulary as `.mk-blockbar` and
 * `.mk-coachcard`) precisely so `prefers-reduced-motion: reduce` can switch it
 * off — an inline `animation:` could not be reduced away.
 */
export function StepIndicator({ tasks }: { tasks: ReadingTask[] }) {
  // Before 印记 has planned, there is no position to report. A 「第 0 步 /
  // 共 0 步」 bar, or an empty box where one will be, is worse than the quiet.
  if (tasks.length === 0) return null;

  const index = tasks.findIndex((t) => t.status === "pending");
  const current = index === -1 ? null : tasks[index]!;
  const settled = tasks.filter((t) => t.status !== "pending").length;
  const filled = Math.round((settled / tasks.length) * 100);

  return (
    <div
      className="mk-stepnow flex flex-none flex-col gap-1.5 border-b border-mk-border px-5 py-2.5"
      style={{ background: "color-mix(in srgb, var(--mk-accent-500) 6%, var(--mk-paper))" }}
      role="status"
      aria-live="polite"
    >
      <div className="flex items-baseline gap-2">
        {current && <span className="mk-stepnow__dot" aria-hidden="true" />}
        <span
          className="text-mk-small font-medium tabular-nums"
          style={{ color: current ? "var(--mk-accent-500)" : "var(--mk-muted)" }}
        >
          {current ? `第 ${index + 1} 步 / 共 ${tasks.length} 步` : `带读走完了 · 共 ${tasks.length} 步`}
        </span>
      </div>

      {current && (
        // Keyed on the step id so React mounts a NEW node when 印记 moves her
        // on — that remount is what replays the CSS animation. Swapping the
        // text of a reused node would change the sentence silently.
        <p key={current.id} className="mk-stepnow__label text-mk-body text-mk-ink">
          {current.label}
        </p>
      )}

      {/* Decorative only: the sentence above already says where she is, so the
          track is aria-hidden rather than a progressbar announcing a number. */}
      <div className="mk-stepnow__track" aria-hidden="true">
        <div className="mk-stepnow__fill" style={{ width: `${filled}%` }} />
      </div>
    </div>
  );
}
