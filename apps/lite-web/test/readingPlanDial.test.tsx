import { render, screen, cleanup, fireEvent, act } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ReadingPlanDial } from "@lite/readings/ReadingPlanDial";
import type { ReadingTask } from "@lite/api/readingRoom";

/**
 * ReadingPlanDial — the whole 带读 plan, folded into a corner.
 *
 * It replaced TWO surfaces at once (`ReadingPlanRail`, the `lg:`-gated 262px
 * column, and `StepIndicator`, the bar at the top of the coach column), so it
 * inherits both of their contracts:
 *
 *   > task list as a hoverable - hover then span box. floating icon. by
 *   > default only shows 3/5 loading state, with a circle showing progress
 *   > percent. hover then expand, and move out then fold.
 *
 *  1. **The current step is the FIRST pending one** — the same rule the server
 *     uses in `currentReadingTask` (`reading_coach.go`). Not "the last one she
 *     touched", not "the one after the last done".
 *  2. **Progress, not controls.** Nothing in the step list is clickable; 印记
 *     moves her between steps. The disc is the only button, and all it does is
 *     open the panel.
 *  3. **Nothing may read as a score** (铁律②). 3 / 5 is a position in a plan;
 *     a percentage, a grade, a ✓/✗ tally or a streak is not. The percentage
 *     lives in the ring's ARC, which is a shape, not a number.
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
  task({ id: "t2", position: 2, label: "找出作者最想让你信的那一句", detail: "在第 3 段里点一句", status: "pending" }),
  task({ id: "t3", position: 3, label: "想一想：这个证据够吗？", status: "pending" }),
  task({ id: "t4", position: 4, label: "用你自己的话说说你留下了什么", status: "pending" }),
];

/**
 * 🚨 `pointerover` / `pointerout`, NOT `pointerenter` / `pointerleave`.
 *
 * `pointerenter` does not bubble, so React never listens for it: it registers
 * `pointerover`/`pointerout` at the root and SYNTHESIZES enter/leave from the
 * pair. `fireEvent.pointerEnter` therefore dispatches a real DOM event that no
 * React handler is subscribed to — the assertion fails and the component is
 * fine, which is the most expensive kind of red.
 */
function hover(el: Element) {
  fireEvent.pointerOver(el);
}
function unhover(el: Element) {
  fireEvent.pointerOut(el);
}

/** A press carrying a real `pointerType`. jsdom has no `PointerEvent`, so
 *  `fireEvent.pointerDown(el, { pointerType })` silently drops the field —
 *  it has to be pinned onto the event object React will read it off. */
function pressWith(el: Element, pointerType: string) {
  const down = new MouseEvent("pointerdown", { bubbles: true });
  Object.defineProperty(down, "pointerType", { value: pointerType });
  el.dispatchEvent(down);
}

/** One finger tap, in Chromium's real order: pointerdown → focus → click. */
function tap(el: HTMLElement) {
  pressWith(el, "touch");
  fireEvent.focus(el);
  fireEvent.click(el);
}

afterEach(cleanup);

describe("ReadingPlanDial · 折起来的时候", () => {
  it("只有一个圆盘：3 / 5 里的两个数字，没有别的字", () => {
    const { container } = render(<ReadingPlanDial tasks={PLAN} />);

    // 1 done out of 4 — the count is the folded state's entire content.
    expect(container.textContent).toBe("1/4");
    // The step labels are NOT rendered until it expands. That is the point of
    // folding: the corner costs 54px, not a column.
    for (const t of PLAN) expect(screen.queryByText(t.label)).toBeNull();
  });

  it("圆盘自己会说出它折起来没说的那句话", () => {
    render(<ReadingPlanDial tasks={PLAN} />);
    // Legible as a shape to the eye; legible as a sentence to a screen reader.
    const disc = screen.getByRole("button", { name: "带读进度 · 第 2 步 / 共 4 步" });
    expect(disc.getAttribute("aria-expanded")).toBe("false");
  });

  it("圆环画的是百分比，但屏幕上没有百分数", () => {
    const { container } = render(<ReadingPlanDial tasks={PLAN} />);
    const fill = container.querySelector(".mk-plandial__ring-fill") as SVGCircleElement;

    const c = 2 * Math.PI * 19;
    // 1 of 4 settled → three quarters of the circumference still dashed off.
    expect(Number(fill.getAttribute("stroke-dashoffset"))).toBeCloseTo(c * 0.75, 5);
    expect(container.textContent).not.toContain("%");
  });

  it("印记还没排路线时，什么都不画", () => {
    const { container } = render(<ReadingPlanDial tasks={[]} />);
    // Not 「第 0 步 / 共 0 步」 and not an empty ring: there is no plan to
    // report on yet, and an empty instrument reads as a broken one.
    expect(container.innerHTML).toBe("");
  });
});

describe("ReadingPlanDial · 展开", () => {
  it("鼠标移上去展开完整清单，移开就折回去", () => {
    vi.useFakeTimers();
    try {
      const { container } = render(<ReadingPlanDial tasks={PLAN} />);
      const root = container.firstElementChild!;

      hover(root);
      expect(screen.getByText("带读进度")).toBeTruthy();
      expect(screen.getByText("第 2 步 / 共 4 步")).toBeTruthy();
      for (const t of PLAN) expect(screen.getByText(t.label)).toBeTruthy();

      unhover(root);
      // A 140ms grace so crossing the 10px gap between disc and panel does not
      // fold it mid-reach — it is still open on the frame the pointer leaves.
      expect(screen.queryByText("带读进度")).toBeTruthy();
      act(() => void vi.advanceTimersByTime(200));
      expect(screen.queryByText("带读进度")).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  it("鼠标点在已经悬停展开的盘上，不会反手把它关掉", () => {
    // Playwright's `click()` fires pointerover (mouse) and then click, and the
    // phone screenshot came back with the panel SHUT: hover had opened it and
    // the click toggled it straight back. Worse, it could not reopen without
    // leaving and re-entering the disc — a button that looks broken.
    const { container } = render(<ReadingPlanDial tasks={PLAN} />);
    const disc = container.querySelector(".mk-plandial__disc")!;

    hover(container.firstElementChild!);
    expect(screen.getByText("带读进度")).toBeTruthy();

    pressWith(disc, "mouse");
    fireEvent.click(disc);

    expect(screen.getByText("带读进度")).toBeTruthy();
  });

  it("没有鼠标的时候，点一下也能展开——手机上没有 hover", () => {
    const { container } = render(<ReadingPlanDial tasks={PLAN} />);
    const disc = container.querySelector(".mk-plandial__disc")!;

    fireEvent.click(disc);
    expect(screen.getByText("带读进度")).toBeTruthy();
    expect(disc.getAttribute("aria-expanded")).toBe("true");

    fireEvent.click(disc);
    expect(screen.queryByText("带读进度")).toBeNull();
  });

  it("🚨 手指点一下真的展开——不会被同一次点击自己的 focus 抵消", () => {
    // The phone screenshot came back with the panel SHUT after a tap. A press
    // focuses the button, focus opens the panel, and the click that follows
    // then toggled the freshly-opened panel closed — one tap, two handlers,
    // net zero. The real event order from Chromium (captured with a probe):
    // pointerdown(touch) → focus → pointerup → click.
    const { container } = render(<ReadingPlanDial tasks={PLAN} />);
    const disc = container.querySelector(".mk-plandial__disc") as HTMLButtonElement;

    tap(disc);
    expect(screen.getByText("带读进度"), "一次点击展不开").toBeTruthy();

    // …and a second tap still closes it. Touch has no hover, so the tap is the
    // only control it has: it has to toggle both ways.
    tap(disc);
    expect(screen.queryByText("带读进度")).toBeNull();
  });

  it("键盘 tab 到圆盘也展开，否则清单只有鼠标够得到", () => {
    const { container } = render(<ReadingPlanDial tasks={PLAN} />);
    const disc = container.querySelector(".mk-plandial__disc") as HTMLButtonElement;

    fireEvent.focus(disc);
    expect(screen.getByText("带读进度")).toBeTruthy();
  });

  it("当前这一步 = 第一个 pending，和服务端 currentReadingTask 同一条规则", () => {
    const outOfOrder: ReadingTask[] = [
      task({ id: "a", position: 1, label: "通读", status: "done" }),
      task({ id: "b", position: 2, label: "跳过的那一步", status: "skipped" }),
      task({ id: "c", position: 3, label: "她现在在这一步", status: "pending" }),
      // A later step already settled must NOT drag the pointer past her.
      task({ id: "d", position: 4, label: "后面已经做完的那一步", status: "done" }),
      task({ id: "e", position: 5, label: "再后面", status: "pending" }),
    ];
    const { container } = render(<ReadingPlanDial tasks={outOfOrder} />);
    hover(container.firstElementChild!);

    expect(screen.getByText("第 3 步 / 共 5 步")).toBeTruthy();
    expect(screen.getByText("她现在在这一步").closest("li")!.className).toContain("is-current");
    expect(screen.getByText("再后面").closest("li")!.className).not.toContain("is-current");
  });

  it("只有当前这一步带着它自己的说明", () => {
    const { container } = render(<ReadingPlanDial tasks={PLAN} />);
    hover(container.firstElementChild!);

    // Shown for every step it is a wall of text she reads instead of the
    // article; shown for none she has to scroll the chat back to remember what
    // 印记 asked.
    expect(screen.getByText("在第 3 段里点一句")).toBeTruthy();
    expect(container.querySelectorAll(".mk-plandial__step-detail")).toHaveLength(1);
  });

  it("走完之后不再指着某一步，也不再有呼吸的光晕", () => {
    const finished = PLAN.map((t) => task({ ...t, status: t.id === "t3" ? "skipped" : "done" }));
    const { container } = render(<ReadingPlanDial tasks={finished} />);

    // The halo says 「这一步是活的」; with nothing pending there is nothing live
    // to point at, so it is not rendered at all rather than left breathing.
    expect(container.querySelector(".mk-plandial__halo")).toBeNull();
    hover(container.firstElementChild!);
    expect(screen.getByText("带读走完了 · 共 4 步")).toBeTruthy();
    expect(container.querySelector(".mk-plandial__dot.is-current")).toBeNull();
  });
});

describe("ReadingPlanDial · 它是进度，不是控制台", () => {
  it("清单里没有任何一个按钮——印记才是移动她的人", () => {
    const { container } = render(<ReadingPlanDial tasks={PLAN} />);
    hover(container.firstElementChild!);

    const steps = container.querySelector(".mk-plandial__steps")!;
    expect(steps.querySelectorAll("button, a, input, [role='button']")).toHaveLength(0);
    // …and the disc is the ONE button on the whole thing.
    expect(container.querySelectorAll("button")).toHaveLength(1);
  });

  it("不许出现任何读起来像分数、评级或者连胜的东西", () => {
    for (const tasks of [PLAN, PLAN.map((t) => task({ ...t, status: "done" }))]) {
      cleanup();
      const { container } = render(<ReadingPlanDial tasks={tasks} />);
      hover(container.firstElementChild!);
      // textContent AND innerHTML: an `aria-label="答对了"` is invisible to the
      // first and read out loud by a screen reader — that exact hole let a
      // scored label through a 12-assertion sweep once already.
      const text = `${container.textContent ?? ""} ${container.innerHTML}`;
      for (const banned of ["得分", "分数", "正确", "答错", "答对", "连胜", "%", "排名", "评级", "满分"]) {
        expect(text, `出现了 ${banned}`).not.toContain(banned);
      }
    }
  });

  it("动效全部挂在样式表的类上，才能被 prefers-reduced-motion 关掉", () => {
    const { container } = render(<ReadingPlanDial tasks={PLAN} />);
    hover(container.firstElementChild!);
    // An inline `animation:` could not be reduced away. The one inline style
    // that IS allowed is `--i`, the stagger index the keyframe delay reads.
    expect(container.innerHTML).not.toMatch(/animation:/);
    expect(container.querySelector(".mk-plandial__sweep")).toBeTruthy();
  });
});
