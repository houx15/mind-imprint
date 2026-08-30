import { useEffect, useRef, useState, type CSSProperties } from "react";
import { Check, SkipForward } from "lucide-react";
import { Icon } from "@/ui";
import type { ReadingTask } from "../api/readingRoom";

/**
 * ReadingPlanDial — 带读进度, folded into a floating dial.
 *
 * It replaces BOTH step surfaces the room used to carry: the `lg:`-gated 262px
 * `ReadingPlanRail` beside the article, and the `StepIndicator` bar at the top
 * of the coach column. The ask that collapsed them into one thing:
 *
 *   > task list as a hoverable - hover then span box. floating icon. by
 *   > default only shows 3/5 loading state, with a circle showing progress
 *   > percent. hover then expand, and move out then fold. so that AI area is
 *   > wider.
 *
 * Four things it is careful about:
 *
 *  - **Folded, it costs 54px of a corner instead of a column.** That column is
 *    what the article and the conversation get back, which is the point.
 *  - **The percent is the ARC, never a number.** 「3 / 5」 is a position in a
 *    plan; 「60%」 beside it is a grade (铁律②). The ring draws the same
 *    fraction the two numbers already say, so it adds a shape, not a score —
 *    which is also why the folded disc carries no ✓, no streak and no
 *    percentage glyph.
 *  - **Current step = the first `pending` one.** Same rule as the server's
 *    `currentReadingTask` (`reading_coach.go`) and as the rail before it, so
 *    the dial and 印记 can never point at different steps.
 *  - **Nothing in it is clickable except the disc itself.** 印记 moves her
 *    between steps; a checkbox here would hand the stage management straight
 *    back to her. The disc's own click exists for touch, where there is no
 *    hover to expand with.
 *
 * All motion is `.mk-plandial*` in `index.css` — never an inline `animation:`,
 * which `prefers-reduced-motion: reduce` could not switch off.
 */

/** The ring geometry. `r` and the 48-unit box are shared with the CSS. */
const RING_R = 19;
const RING_C = 2 * Math.PI * RING_R;

/** How long the panel survives the pointer leaving, so crossing the 10px gap
 *  between the disc and the panel does not fold it mid-reach. */
const FOLD_DELAY_MS = 140;

export function ReadingPlanDial({ tasks }: { tasks: ReadingTask[] }) {
  const [open, setOpen] = useState(false);
  const foldTimer = useRef<number | null>(null);
  /**
   * The press a `click` came from: what pressed, and whether the panel was
   * open BEFORE the press. Both halves are load-bearing — see `onClick`.
   */
  const press = useRef<{ pointerType: string; wasOpen: boolean } | null>(null);

  function cancelFold() {
    if (foldTimer.current !== null) {
      window.clearTimeout(foldTimer.current);
      foldTimer.current = null;
    }
  }

  useEffect(() => cancelFold, []);

  // Before 印记 has planned, there is no position to report. A 「第 0 步 /
  // 共 0 步」 dial, or an empty ring where one will be, is worse than the quiet.
  if (tasks.length === 0) return null;

  const index = tasks.findIndex((t) => t.status === "pending");
  const current = index === -1 ? null : tasks[index]!;
  const settled = tasks.filter((t) => t.status !== "pending").length;
  const total = tasks.length;
  const position = current ? `第 ${index + 1} 步 / 共 ${total} 步` : `带读走完了 · 共 ${total} 步`;

  return (
    <div
      className={`mk-plandial${open ? " is-open" : ""}`}
      // Hover must NOT open on a finger: a tap fires pointerenter and then
      // click, so the panel would open and the click would immediately toggle
      // it shut again. Written as "not touch" rather than "is mouse" because
      // jsdom has no `PointerEvent` at all — an `=== "mouse"` test would be
      // dead in every test that exercises hovering. A real browser always
      // reports a pointerType, so the touch guard is exact where it matters.
      onPointerEnter={(e) => {
        if (e.pointerType === "touch" || e.pointerType === "pen") return;
        cancelFold();
        setOpen(true);
      }}
      onPointerLeave={(e) => {
        if (e.pointerType === "touch" || e.pointerType === "pen") return;
        cancelFold();
        foldTimer.current = window.setTimeout(() => setOpen(false), FOLD_DELAY_MS);
      }}
      // Keyboard reaches the disc by tab, which is not a pointer — so the panel
      // has to open on focus too, or the step list is mouse-only.
      onFocus={() => {
        cancelFold();
        setOpen(true);
      }}
      onBlur={() => setOpen(false)}
    >
      <button
        type="button"
        className="mk-plandial__disc"
        aria-expanded={open}
        // The whole point of the folded state is that the numbers are legible
        // as a shape rather than as a sentence; the sentence is here.
        aria-label={`带读进度 · ${position}`}
        // Recorded on the way DOWN, because by the time `click` runs the panel
        // may already have been opened by this very press — pressing a button
        // focuses it, and focus opens the panel.
        onPointerDown={(e) => {
          press.current = { pointerType: e.pointerType, wasOpen: open };
        }}
        onClick={() => {
          cancelFold();
          const from = press.current;
          press.current = null;

          // No pointer at all: Enter or Space on a focused disc. Focus has
          // already opened it, so a plain toggle is the only key that can also
          // close it again.
          if (!from) {
            setOpen((v) => !v);
            return;
          }
          // A MOUSE arrives with the panel already hover-opened, so toggling
          // would shut it — and it could not reopen without leaving and
          // re-entering the disc, which reads as a broken button. With a mouse
          // the hover governs and the click only ever confirms.
          if (from.pointerType === "mouse") {
            setOpen(true);
            return;
          }
          // 🚨 A FINGER, and the reason `wasOpen` exists. The tap is the only
          // control touch has, so it must toggle — but the same tap has
          // already run focus→open, so `setOpen(v => !v)` would read the
          // POST-focus state and shut the panel the tap just opened. That is
          // exactly what the phone screenshot came back showing. Toggle
          // against the state the press STARTED from instead.
          setOpen(!from.wasOpen);
        }}
      >
        {/* The sweep and the halo are pure decoration and must never be read
            out: the button's own label already says everything. */}
        <span className="mk-plandial__sweep" aria-hidden="true" />
        {current && <span className="mk-plandial__halo" aria-hidden="true" />}
        <svg className="mk-plandial__ring" viewBox="0 0 48 48" aria-hidden="true">
          <circle className="mk-plandial__ring-track" cx="24" cy="24" r={RING_R} />
          <circle
            className="mk-plandial__ring-fill"
            cx="24"
            cy="24"
            r={RING_R}
            strokeDasharray={RING_C}
            // The arc IS the percentage. Drawn, not written.
            strokeDashoffset={RING_C * (1 - settled / total)}
          />
        </svg>
        <span className="mk-plandial__count" aria-hidden="true">
          <b>{settled}</b>
          <i>/</i>
          <em>{total}</em>
        </span>
      </button>

      {open && (
        <div className="mk-plandial__panel" role="group" aria-label="带读进度">
          <div className="mk-plandial__panel-head">
            <span className="mk-plandial__title">带读进度</span>
            <span className="mk-plandial__position">{position}</span>
            <span className="mk-plandial__scan" aria-hidden="true" />
          </div>

          <ol className="mk-plandial__steps">
            {tasks.map((task, i) => {
              const isCurrent = current?.id === task.id;
              return (
                <li
                  key={task.id}
                  className={`mk-plandial__step is-${task.status}${isCurrent ? " is-current" : ""}`}
                  style={{ "--i": i } as CSSProperties}
                >
                  <StepDot index={i} status={task.status} current={isCurrent} />
                  <span className="mk-plandial__step-body">
                    <span className="mk-plandial__step-label">{task.label}</span>
                    {/* The step's own instruction, shown ONLY for the step she
                        is on. On every step it is a wall of text she reads
                        instead of the article; on none of them she has to
                        scroll the chat back to remember what to do. */}
                    {isCurrent && task.detail && (
                      <span className="mk-plandial__step-detail">{task.detail}</span>
                    )}
                  </span>
                </li>
              );
            })}
          </ol>
        </div>
      )}
    </div>
  );
}

function StepDot({
  index,
  status,
  current,
}: {
  index: number;
  status: ReadingTask["status"];
  current: boolean;
}) {
  return (
    <span
      aria-hidden="true"
      className={`mk-plandial__dot is-${status}${current ? " is-current" : ""}`}
    >
      {status === "done" ? (
        <Icon icon={Check} size={11} />
      ) : status === "skipped" ? (
        <Icon icon={SkipForward} size={10} />
      ) : (
        index + 1
      )}
    </span>
  );
}
