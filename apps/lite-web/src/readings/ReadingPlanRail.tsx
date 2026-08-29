import { Check, SkipForward } from "lucide-react";
import { Icon, Pebble } from "@/ui";
import type { ReadingTask } from "../api/readingRoom";

/**
 * ReadingPlanRail — 带读进度, and nothing else.
 *
 * This column used to hold the whole 带读 experience: the invitation, the
 * conversation, the composer AND the step list — while the room beside it ran
 * its own AI chat. Two 印记 talking on one screen:
 *
 *   > we don't have two AIs. only one AI talks. but the left column can serve
 *   > as a task status column.
 *
 * So the conversation moved into the room's own coach column (one thread, one
 * composer — see ReadingRoomHost's `renderCoach`) and what is left here is
 * exactly what the ruling names: **status**. Nothing in it is clickable, and
 * that is not an oversight — 印记 moves her between steps; she reads and
 * answers. A checkbox here would hand the stage management straight back.
 */
export function ReadingPlanRail({ tasks }: { tasks: ReadingTask[] }) {
  const current = tasks.find((t) => t.status === "pending") ?? null;
  const settled = tasks.filter((t) => t.status !== "pending").length;

  // Before 印记 has planned there is nothing to report, and an empty bordered
  // card reporting nothing reads as something broken. A quiet line does not.
  if (tasks.length === 0) {
    return (
      <div className="flex flex-col items-center gap-3 rounded-mk-lg border border-dashed border-mk-border px-4 py-8 text-center">
        <Pebble state="idle" size={34} />
        <p className="text-mk-small leading-relaxed text-mk-muted">
          印记还没排路线。
          <br />
          在右边说一声「开始」，这里就会长出来。
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-baseline justify-between gap-2">
        <h2 className="text-mk-h3 text-mk-ink">带读进度</h2>
      </div>

      <ProgressBar done={settled} total={tasks.length} />

      <ol className="flex list-none flex-col gap-2.5">
        {tasks.map((task, i) => {
          const isCurrent = current?.id === task.id;
          return (
            <li key={task.id} className="flex items-start gap-2.5">
              <StepDot index={i} status={task.status} current={isCurrent} />
              <span className="min-w-0 flex-1">
                <span
                  className="block text-mk-body"
                  style={{
                    color: isCurrent
                      ? "var(--mk-ink)"
                      : task.status === "pending"
                        ? "var(--mk-faint)"
                        : "var(--mk-muted)",
                    fontWeight: isCurrent ? 600 : 400,
                    textDecoration: task.status === "skipped" ? "line-through" : undefined,
                  }}
                >
                  {task.label}
                </span>
                {/* The step's own instruction, shown ONLY for the step she is
                    on. Shown for every step it would be a wall of text she is
                    reading instead of the article; hidden for every step she
                    has to scroll back to the chat to remember what to do. */}
                {isCurrent && task.detail && (
                  <span className="mt-0.5 block text-mk-small leading-relaxed text-mk-secondary">
                    {task.detail}
                  </span>
                )}
              </span>
            </li>
          );
        })}
      </ol>
    </div>
  );
}

/**
 * Decorative only (铁律②).
 *
 * The header above this used to also carry a bare 「{settled} / {total}」, and
 * `StepIndicator` — 300px to its right, on the same screen — says
 * 「第 N 步 / 共 M 步」. Two progress numbers at once, and the rail's was the
 * FIRST thing she read on entering the room, where 「0 / 6」 reads as a score
 * rather than as a position. The ruling: keep the step indicator, drop the
 * rail's number.
 *
 * So the track is `aria-hidden` with no `progressbar` role: a role carries
 * `aria-valuenow`/`aria-valuemax`, which would just say 「0 of 6」 out loud —
 * the same number, only invisible. The step list underneath already shows
 * exactly which steps are settled, one dot at a time.
 */
function ProgressBar({ done, total }: { done: number; total: number }) {
  const pct = total === 0 ? 0 : Math.round((done / total) * 100);
  return (
    <div
      className="h-1.5 w-full overflow-hidden rounded-mk-full"
      aria-hidden="true"
      style={{ background: "color-mix(in srgb, var(--mk-accent-500) 12%, transparent)" }}
    >
      <div
        className="h-full rounded-mk-full transition-[width] duration-300 ease-mk motion-reduce:transition-none"
        style={{ width: `${pct}%`, background: "var(--mk-accent-500)" }}
      />
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
      className="mt-[3px] flex h-[18px] w-[18px] shrink-0 items-center justify-center rounded-mk-full text-[10px] font-semibold"
      style={
        status === "done"
          ? { background: "var(--mk-accent-500)", color: "white" }
          : status === "skipped"
            ? { border: "1px dashed var(--mk-border)", color: "var(--mk-faint)" }
            : current
              ? {
                  border: "1.5px solid var(--mk-accent-500)",
                  color: "var(--mk-accent-700)",
                  background: "color-mix(in srgb, var(--mk-accent-500) 10%, transparent)",
                }
              : { border: "1px solid var(--mk-border)", color: "var(--mk-faint)" }
      }
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
