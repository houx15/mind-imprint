import { useEffect, useRef, useState, type CSSProperties } from "react";
import { Check, Flag, MapPin, SkipForward, ChevronUp } from "lucide-react";
import { Icon } from "@/ui";
import type { ReadingTask } from "../api/readingRoom";

/**
 * ReadingPlanDial — 带读进度, folded into a dial that lives on 印记's column.
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
 * ## 2026-09-22 · 它搬到了右栏，而且吃掉了最后一块常驻的步骤条
 *
 * 在这之前，房间上**两块**步骤面同时在：这个盘浮在房间左下角，而 印记 那一栏
 * 的顶上还钉着一整块 `StepIndicator`（编号徽章 + 第几步 + 步骤标题 + 定位原文
 * + 十五个圆点的路线）。产品负责人 2026-09-22 附截图，红框圈的正是那一块：
 *
 *   > 红框内模块占用空间有点多，导致印记的提示句被压缩在下面很小的地方，
 *   > 需要不停上下翻动
 *   > we have a round button showing all steps. can we move it to the right
 *   > ai side? and hover can show the steps? and click can be this expanded
 *   > view?
 *
 * 所以：常驻的那一块整个删掉（`StepIndicator.tsx` 已经没有调用方），盘搬进
 * 印记 那一栏的页签行，并且长出**两层**：
 *
 *   悬停 → 步骤清单（浮层，和它原来那一层一样）
 *   点击 → 展开视图（**就地**摊在页签下面，把红框里那一块原样摆出来：
 *          编号徽章、第几步 / 共几步、这一步的标题和说明、定位原文、
 *          十五个编号圆点）
 *
 * 展开视图是**就地**的，不是浮层：她点开它是要照着做事（跳到原文、看清这一步
 * 要干嘛），一块盖在对话上面的浮层会在她一动鼠标就消失。默认收着，所以
 * 印记 的话拿回了那一块地方 —— 那正是这一条要解决的事。
 *
 * 另外三件它一直很在意的事，没有变：
 *
 *  - **The percent is the ARC, never a number.** 「3 / 5」 is a position in a
 *    plan; 「60%」 beside it is a grade. The ring draws the same fraction the
 *    two numbers already say, so it adds a shape, not a score.
 *  - **Current step = the first `pending` one.** Same rule as the server's
 *    `currentReadingTask` (`reading_coach.go`), so the dial and 印记 can never
 *    point at different steps.
 *  - **她不操作步骤。** 盘上唯一能按的是盘自己和「定位原文」。印记 决定她走到
 *    哪一步；这里摆一个勾选框等于把阶段管理原样退还给她。
 *
 * All motion is `.mk-plandial*` in `index.css` — never an inline `animation:`,
 * which `prefers-reduced-motion: reduce` could not switch off.
 */

/** The ring geometry. `r` and the 48-unit box are shared with the CSS. */
const RING_R = 19;
const RING_C = 2 * Math.PI * RING_R;

/** How long the hover panel survives the pointer leaving, so crossing the 10px
 *  gap between the disc and the panel does not fold it mid-reach. */
const FOLD_DELAY_MS = 140;

export function ReadingPlanDial({
  tasks,
  onLocate,
}: {
  tasks: ReadingTask[];
  /** 跳到这一步说的那一段。展开视图上那颗「定位原文」调它 —— 它是从被删掉的
   *  `StepIndicator` 那一块搬过来的，别的都能从这里读出来，只有「滚过去」
   *  是房间的事。 */
  onLocate?: (blockId: string) => void;
}) {
  /** 悬停那一层：步骤清单的浮层。 */
  const [hovered, setHovered] = useState(false);
  /** 点击那一层：就地摊开的展开视图。它由点击开关，不跟着鼠标走。 */
  const [expanded, setExpanded] = useState(false);
  const discRef = useRef<HTMLButtonElement>(null);
  const foldTimer = useRef<number | null>(null);

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
  const position = current ? `第 ${index + 1} 步 / 共 ${total} 步` : `带读已完成 · 共 ${total} 步`;
  // 展开着的时候不再弹悬停浮层：同一份清单在屏幕上出现两次，第二次没有信息。
  const showHover = hovered && !expanded;

  return (
    <>
      <div
        className={`mk-plandial${showHover ? " is-open" : ""}${expanded ? " is-expanded" : ""}`}
        // Hover must NOT open on a finger: a tap fires pointerenter and then
        // click, so the panel would open and the click would immediately toggle
        // it shut again. Written as "not touch" rather than "is mouse" because
        // jsdom has no `PointerEvent` at all — an `=== "mouse"` test would be
        // dead in every test that exercises hovering. A real browser always
        // reports a pointerType, so the touch guard is exact where it matters.
        onPointerEnter={(e) => {
          if (e.pointerType === "touch" || e.pointerType === "pen") return;
          cancelFold();
          setHovered(true);
        }}
        onPointerLeave={(e) => {
          if (e.pointerType === "touch" || e.pointerType === "pen") return;
          cancelFold();
          foldTimer.current = window.setTimeout(() => setHovered(false), FOLD_DELAY_MS);
        }}
        // Keyboard reaches the disc by tab, which is not a pointer — so the
        // hover layer has to open on focus too, or the step list is mouse-only.
        onFocus={() => {
          cancelFold();
          setHovered(true);
        }}
        onBlur={() => setHovered(false)}
        onKeyDown={(e) => { if (e.key === "Escape") { setHovered(false); setExpanded(false); } }}
      >
        <button
          type="button"
          ref={discRef}
          className="mk-plandial__disc"
          aria-expanded={expanded}
          aria-controls="mk-plandial-expanded"
          // The whole point of the folded state is that the numbers are legible
          // as a shape rather than as a sentence; the sentence is here.
          aria-label={`带读进度 · ${position}`}
          // 🚨 点击只管展开视图那一层，和悬停那一层各管各的。
          //
          // 原来两层是同一个 state，于是鼠标用户点下去会把刚刚被悬停打开的
          // 面板关掉（`setOpen(v => !v)` 读到的是 hover 之后的那个值），得离开
          // 再进来才能重开 —— 手机上那张截图逐字就是这个样子。拆成两个 state
          // 之后这类竞争结构上不存在了：hover 归指针，expanded 归点击。
          onClick={() => setExpanded((v) => !v)}
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

        {showHover && (
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

      {expanded && (
        <PlanExpanded
          id="mk-plandial-expanded"
          tasks={tasks}
          current={current}
          index={index}
          onLocate={onLocate}
          onClose={() => {
            setExpanded(false);
            discRef.current?.focus();
            setHovered(false);
          }}
        />
      )}
    </>
  );
}

/**
 * 展开视图 —— 点开盘之后就地摊在页签下面的那一块。
 *
 * 内容是被删掉的 `StepIndicator` 那一块原样搬过来的：编号徽章、第几步 /
 * 共几步、这一步的标题和说明、定位原文、一排编号圆点。区别只有一个，而这个
 * 区别就是产品负责人要的那件事：**它默认不在**。
 */
function PlanExpanded({
  id,
  tasks,
  current,
  index,
  onLocate,
  onClose,
}: {
  id: string;
  tasks: ReadingTask[];
  current: ReadingTask | null;
  index: number;
  onClose: () => void;
  onLocate?: (blockId: string) => void;
}) {
  return (
    <section id={id} className="mk-planwide" aria-label="阅读任务" onKeyDown={(e) => { if (e.key === "Escape") onClose(); }}>
      <div className="mk-planwide__head">
        <span className="mk-planwide__emblem" aria-hidden="true">
          {current ? String(index + 1).padStart(2, "0") : <Icon icon={Flag} size={20} />}
        </span>
        <div className="mk-planwide__now">
          <span className="mk-planwide__position">
            {current ? `第 ${index + 1} 步 / 共 ${tasks.length} 步` : `带读已完成 · 共 ${tasks.length} 步`}
          </span>
          {current && <p className="mk-planwide__label">{current.label}</p>}
          {current?.detail && <p className="mk-planwide__detail">{current.detail}</p>}
        </div>
        <button type="button" className="mk-planwide__close" onClick={onClose} aria-label="收起阅读任务" title="收起阅读任务">
          <Icon icon={ChevronUp} size={16} />
        </button>
      </div>
      {current?.blockId && onLocate && (
        <button
          type="button"
          className="mk-planwide__locate"
          onClick={() => onLocate(current.blockId)}
        >
          <Icon icon={MapPin} size={14} />
          定位原文
        </button>
      )}
      <ol className="mk-planwide__path" aria-label="任务路线">
        {tasks.map((task, i) => (
          <li
            key={task.id}
            data-state={task.status}
            data-current={task.id === current?.id || undefined}
            aria-current={task.id === current?.id ? "step" : undefined}
            title={task.label}
          >
            <span aria-label={`任务 ${i + 1}：${task.label}`}>
              {task.status === "done" ? (
                <Icon icon={Check} size={14} />
              ) : task.status === "skipped" ? (
                <Icon icon={SkipForward} size={12} />
              ) : (
                i + 1
              )}
            </span>
          </li>
        ))}
      </ol>
    </section>
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
