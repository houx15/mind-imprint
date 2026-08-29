import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { StepIndicator } from "@lite/readings/StepIndicator";
import type { ReadingTask } from "@lite/api/readingRoom";

/**
 * StepIndicator — 「我现在在第几步」, and nothing more.
 *
 * It fills the slot lite's forked room left empty where pro still carries
 * 「你读这篇是为了：点击填写…」 — a bar that in lite was write-only. The ask
 * that replaced it, verbatim:
 *
 *   > change this: 你读这篇是为了：点击填写… to an active current step, which
 *   > has some animation.
 *
 * Two things this file is here to hold still:
 *
 *  1. **The current step is the FIRST pending one** — the same rule the server
 *     uses in `currentReadingTask` (`reading_coach.go`). Not "the last one she
 *     touched", not "the one after the last done": if a step was skipped and a
 *     later one somehow finished, the first `pending` is still where she is.
 *  2. **Nothing here may read as a score** (铁律②). 第 N 步 / 共 M 步 is a
 *     position in a plan; a percentage, a grade, a streak or a ✓/✗ is not, and
 *     the sweep at the bottom refuses all of them.
 */

function task(over: Partial<ReadingTask> & { id: string }): ReadingTask {
  return {
    position: 0,
    kind: "read",
    label: "通读一遍",
    detail: "",
    blockId: "",
    status: "pending",
    completedAt: null,
    ...over,
  } as ReadingTask;
}

const PLAN: ReadingTask[] = [
  task({ id: "t1", position: 1, label: "先通读一遍，别停下来查词", status: "done" }),
  task({ id: "t2", position: 2, label: "找出作者最想让你信的那一句", status: "pending" }),
  task({ id: "t3", position: 3, label: "想一想：这个证据够吗？", status: "pending" }),
  task({ id: "t4", position: 4, label: "用你自己的话说说你留下了什么", status: "pending" }),
];

afterEach(cleanup);

describe("StepIndicator", () => {
  it("names where she is: 第 N 步 / 共 M 步 plus the step's own label", () => {
    render(<StepIndicator tasks={PLAN} />);

    expect(screen.getByText("第 2 步 / 共 4 步")).toBeTruthy();
    expect(screen.getByText("找出作者最想让你信的那一句")).toBeTruthy();
    // The steps she is not on are the rail's job, not this bar's.
    expect(screen.queryByText("想一想：这个证据够吗？")).toBeNull();
    expect(screen.queryByText("先通读一遍，别停下来查词")).toBeNull();
  });

  it("takes the FIRST pending step, the same rule as the server's currentReadingTask", () => {
    const outOfOrder: ReadingTask[] = [
      task({ id: "a", position: 1, label: "通读", status: "done" }),
      task({ id: "b", position: 2, label: "跳过的那一步", status: "skipped" }),
      task({ id: "c", position: 3, label: "她现在在这一步", status: "pending" }),
      // A later step already settled must NOT drag the pointer past her.
      task({ id: "d", position: 4, label: "后面已经做完的那一步", status: "done" }),
      task({ id: "e", position: 5, label: "再后面", status: "pending" }),
    ];

    render(<StepIndicator tasks={outOfOrder} />);

    expect(screen.getByText("第 3 步 / 共 5 步")).toBeTruthy();
    expect(screen.getByText("她现在在这一步")).toBeTruthy();
    expect(screen.queryByText("再后面")).toBeNull();
  });

  it("stops pointing at a step once every step is settled", () => {
    const finished = PLAN.map((t) => task({ ...t, status: t.id === "t3" ? "skipped" : "done" }));

    render(<StepIndicator tasks={finished} />);

    expect(screen.queryByText(/第 \d+ 步/)).toBeNull();
    for (const t of finished) expect(screen.queryByText(t.label)).toBeNull();
    expect(screen.getByText(/带读走完了/)).toBeTruthy();
    expect(screen.getByText(/共 4 步/)).toBeTruthy();
  });

  it("says nothing at all before 印记 has planned anything", () => {
    const { container } = render(<StepIndicator tasks={[]} />);
    // Not 「第 0 步 / 共 0 步」 and not an empty bordered box: there is no plan
    // to report on, and the coach column below is already inviting her to
    // start one.
    expect(container.innerHTML).toBe("");
  });

  it("is visible on a narrow screen — it is the FIRST step surface there", () => {
    // `ReadingPlanRail` lives in an `lg:`-gated aside, so below that breakpoint
    // this bar is the only place she can see where she is. A responsive-hiding
    // utility anywhere in this subtree would take that away.
    const { container } = render(<StepIndicator tasks={PLAN} />);

    const classes = Array.from(container.querySelectorAll("*")).flatMap((el) =>
      Array.from(el.classList),
    );
    expect(
      classes.filter(
        (c) => c === "hidden" || /^(sm|md|lg|xl|2xl):/.test(c) || /^(sm|md|lg|xl|2xl):hidden$/.test(c),
      ),
    ).toEqual([]);
  });

  it("carries its motion with a prefers-reduced-motion escape, not inline keyframes", () => {
    // The animation is a stylesheet class (`mk-stepnow*`), which is what lets
    // `@media (prefers-reduced-motion: reduce)` in index.css switch it off.
    // An inline `animation:` on the element could not be reduced away.
    const { container } = render(<StepIndicator tasks={PLAN} />);
    const root = container.firstElementChild as HTMLElement;

    expect(root.className).toMatch(/mk-stepnow/);
    expect(container.innerHTML).not.toMatch(/animation:/);
    expect(container.querySelector(".mk-stepnow__label")).toBeTruthy();
  });

  it("keys the label on the step id so a new step actually re-animates", () => {
    // Without the key React would reuse the same <p> and only swap its text —
    // the CSS animation would never replay and the change would be silent.
    const { container, rerender } = render(<StepIndicator tasks={PLAN} />);
    const first = container.querySelector(".mk-stepnow__label");

    const advanced = PLAN.map((t) => task({ ...t, status: t.id === "t2" ? "done" : t.status }));
    rerender(<StepIndicator tasks={advanced} />);
    const second = container.querySelector(".mk-stepnow__label");

    expect(screen.getByText("想一想：这个证据够吗？")).toBeTruthy();
    expect(second).not.toBe(first);
  });

  it("never renders anything that reads as a score, a grade or a streak", () => {
    for (const tasks of [PLAN, PLAN.map((t) => task({ ...t, status: "done" }))]) {
      cleanup();
      const { container } = render(<StepIndicator tasks={tasks} />);
      const text = container.textContent ?? "";
      for (const banned of ["✓", "✗", "√", "×", "分", "得分", "正确", "错", "连胜", "%", "排名"]) {
        expect(text).not.toContain(banned);
      }
    }
  });
});
