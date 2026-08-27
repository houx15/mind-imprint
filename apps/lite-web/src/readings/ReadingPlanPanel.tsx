import { useState } from "react";
import { Check, SkipForward, ListChecks, RotateCcw, Crosshair } from "lucide-react";
import { Button, Icon } from "@/ui";
import { ApiError } from "../api/client";
import {
  generateReadingPlan,
  setReadingTaskStatus,
  type ReadingPlan,
  type ReadingTask,
} from "../api/readingRoom";

/**
 * ReadingPlanPanel — 印记's task list for this article.
 *
 * The product call: *"an intelligent reading coach would generate a task list
 * after students giving a paragraph … general scan → look at one/two focal
 * paragraphs → read through a lens → think about information → finish the
 * questions."*
 *
 * The steps come from a fixed routine library server-side; the model picks a
 * routine, names the paragraphs worth slowing down on, and tunes each line to
 * this article. It cannot invent steps.
 *
 * ## Not a checklist that nags
 *
 * 铁律②. Every step is always actionable, in any order, and 跳过 sits beside
 * 做完了 with equal weight — skipping is RECORDED (过程即数据), not prevented,
 * and there is no "you haven't finished step 1" state anywhere in here. The
 * progress line counts what she did; it never scolds her for what she didn't.
 */
export function ReadingPlanPanel({
  readingId,
  plan,
  onPlan,
  onFocusBlock,
}: {
  readingId: string;
  plan: ReadingPlan | null;
  onPlan: (next: ReadingPlan) => void;
  /** Scrolls the article to the paragraph 印记 singled out. */
  onFocusBlock: (blockId: string) => void;
}) {
  const [planning, setPlanning] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function makePlan() {
    setPlanning(true);
    setError(null);
    try {
      onPlan(await generateReadingPlan(readingId));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "这次没排出来，再试一次。");
    } finally {
      setPlanning(false);
    }
  }

  async function setStatus(task: ReadingTask, status: ReadingTask["status"]) {
    if (!plan) return;
    // Optimistic: a checkbox that waits on a round-trip feels broken, and the
    // only cost of being wrong is one row's status, which the next load fixes.
    const before = plan;
    onPlan({ ...plan, tasks: plan.tasks.map((t) => (t.id === task.id ? { ...t, status } : t)) });
    try {
      await setReadingTaskStatus(readingId, task.id, status);
    } catch (err) {
      onPlan(before);
      setError(err instanceof ApiError ? err.message : "记录这一步失败，请重试。");
    }
  }

  if (!plan || plan.tasks.length === 0) {
    return (
      <div className="flex flex-col items-start gap-2 rounded-mk-md border border-mk-border bg-mk-paper p-4">
        <p className="text-mk-body text-mk-muted">
          不知道从哪儿下手？让印记看看这篇文章，给你排一份读法——先读哪儿、哪一段值得慢慢看。
        </p>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => void makePlan()}
          loading={planning}
          iconStart={<Icon icon={ListChecks} size={14} />}
        >
          帮我排一份读法
        </Button>
        {error && <p className="text-mk-small text-mk-danger">{error}</p>}
      </div>
    );
  }

  const done = plan.tasks.filter((t) => t.status !== "pending").length;

  return (
    <section className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex min-w-0 flex-col">
          <span className="text-mk-label text-mk-faint">这篇的读法</span>
          <span className="truncate text-mk-body font-semibold text-mk-ink">{plan.routineName || "任务清单"}</span>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <span className="text-mk-small text-mk-muted">
            {done} / {plan.tasks.length}
          </span>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => void makePlan()}
            loading={planning}
            iconStart={<Icon icon={RotateCcw} size={13} />}
          >
            重排
          </Button>
        </div>
      </div>

      <ol className="flex list-none flex-col gap-2">
        {plan.tasks.map((task, i) => (
          <TaskRow
            key={task.id}
            task={task}
            index={i}
            onFocusBlock={onFocusBlock}
            onDone={() => void setStatus(task, task.status === "done" ? "pending" : "done")}
            onSkip={() => void setStatus(task, task.status === "skipped" ? "pending" : "skipped")}
          />
        ))}
      </ol>

      {error && <p className="text-mk-small text-mk-danger">{error}</p>}
    </section>
  );
}

function TaskRow({
  task,
  index,
  onFocusBlock,
  onDone,
  onSkip,
}: {
  task: ReadingTask;
  index: number;
  onFocusBlock: (blockId: string) => void;
  onDone: () => void;
  onSkip: () => void;
}) {
  const settled = task.status !== "pending";

  return (
    <li
      className="flex flex-col gap-2 rounded-mk-md border bg-mk-surface p-3"
      style={
        task.status === "done"
          ? { borderColor: "var(--mk-accent-200)", background: "var(--mk-accent-50)" }
          : { borderColor: "var(--mk-border)" }
      }
    >
      <div className="flex items-start gap-2.5">
        <span
          aria-hidden="true"
          className="mt-[2px] flex h-5 w-5 shrink-0 items-center justify-center rounded-mk-full text-mk-label"
          style={
            task.status === "done"
              ? { background: "var(--mk-accent-500)", color: "white" }
              : task.status === "skipped"
                ? { border: "1px dashed var(--mk-border)", color: "var(--mk-faint)" }
                : { border: "1px solid var(--mk-border)", color: "var(--mk-muted)" }
          }
        >
          {task.status === "done" ? <Icon icon={Check} size={11} /> : index + 1}
        </span>

        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span
            className="text-mk-body font-semibold"
            style={{ color: task.status === "skipped" ? "var(--mk-faint)" : "var(--mk-ink)" }}
          >
            {task.label}
          </span>
          {task.detail && <span className="text-mk-small text-mk-muted">{task.detail}</span>}
          {task.blockId && (
            <button
              type="button"
              onClick={() => onFocusBlock(task.blockId)}
              className="mt-1 flex w-fit items-center gap-1 rounded-mk-xs px-1.5 py-0.5 text-mk-small text-mk-accent-700 transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
            >
              <Icon icon={Crosshair} size={12} /> 跳到这一段
            </button>
          )}
        </div>
      </div>

      {/* 做完了 and 跳过 sit side by side with equal weight. Skipping is a real
          answer, recorded rather than discouraged — 铁律②. */}
      <div className="flex items-center gap-1.5 pl-[30px]">
        <Button variant={task.status === "done" ? "secondary" : "ghost"} size="sm" onClick={onDone}>
          {task.status === "done" ? "已完成" : "做完了"}
        </Button>
        <Button variant="ghost" size="sm" onClick={onSkip} iconStart={<Icon icon={SkipForward} size={13} />}>
          {task.status === "skipped" ? "已跳过" : "跳过"}
        </Button>
        {settled && task.status === "skipped" && (
          <span className="text-mk-small text-mk-faint">跳过也会记下来，不影响你继续</span>
        )}
      </div>
    </li>
  );
}
