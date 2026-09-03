import { render, screen, cleanup, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { ReadingCoachSlot } from "@lite/readings/ReadingRoom";
import type { ReadingTask } from "../src/api/readingRoom";

/**
 * 读法走完之后，屏幕上必须出现一条通往「完成」的路。
 *
 * 🚨 为什么这值得一个测试。这条不是「渲染了某个元素」那类断言——它是一条
 * **线上已经付过代价**的不变量。在这之前，「每一步都做完了」的全部体现是
 * 输入框的 placeholder 换了一句话，而 完成这篇 是文章工具栏上一颗从第一秒
 * 就在那儿的小按钮。结果：生产库里 23 篇阅读有 17 篇状态永远是 active，
 * lite_report 最后一次触发是 08-31。**没有人走到「完成」**，于是报告那一整
 * 条链路（含它自己的两个 bug）在生产里从来没被走通过。
 *
 * 所以这里钉两件事，而且都不是文案：
 *
 *  1. 计划做完 → 有一个能点的东西，点下去会调 `onFinish`；
 *  2. 计划**没**做完 → 它不在。提前出现比不出现更糟：她会在读到一半时
 *     把这一篇终结掉，而完成是不可逆的。
 *
 * 断言按 `onFinish` 有没有被调用来写，不按按钮上的字——文案是产品负责人
 * 的，她改一个词不该弄红一个测试。
 */

const postTurn = vi.fn();
vi.mock("../src/api/readingRoom", async (orig) => ({
  ...(await orig<Record<string, unknown>>()),
  postReadingCoachTurn: (...args: unknown[]) => postTurn(...args),
}));

const { ReadingCoachPanel } = await import("../src/readings/ReadingCoachPanel");

function slot(): ReadingCoachSlot {
  return { locked: false, quotes: [], removeQuote: () => {}, clearQuotes: vi.fn() };
}

function task(id: string, status: ReadingTask["status"]): ReadingTask {
  return {
    id,
    position: Number(id),
    kind: "read",
    label: `第${id}步`,
    detail: "",
    blockId: "",
    status,
    completedAt: status === "pending" ? null : "2026-09-04T02:00:00Z",
  };
}

function panel(tasks: ReadingTask[], onFinish = vi.fn()) {
  render(
    <ReadingCoachPanel
      readingId="r1"
      tasks={tasks}
      initialMessages={[{ seq: 1, role: "ai", content: "我们看第二段。", createdAt: "" }]}
      slot={slot()}
      onTasks={() => {}}
      onFocusBlock={() => {}}
      lensDone={null}
      onLensDoneSent={() => {}}
      onFinish={onFinish}
    />,
  );
  return onFinish;
}

beforeEach(() => postTurn.mockReset());
afterEach(cleanup);

it("每一步都结算之后，给她一个通往完成的按钮", () => {
  const onFinish = panel([task("1", "done"), task("2", "done")]);
  const button = screen.getByRole("button", { name: /完成阅读/ });
  fireEvent.click(button);
  expect(onFinish).toHaveBeenCalledTimes(1);
});

it("跳过的步骤也算结算过——跳过是被记录的选择，不是欠着的事", () => {
  // 铁律④：跳过被转化成信号，而不是被消灭。一篇每一步都「已跳过」的阅读
  // 一样是走完了的，不该把她永远关在房间里。
  const onFinish = panel([task("1", "done"), task("2", "skipped")]);
  fireEvent.click(screen.getByRole("button", { name: /完成阅读/ }));
  expect(onFinish).toHaveBeenCalledTimes(1);
});

it("🚨 还有待办的步骤时，这个按钮不许出现", () => {
  // 完成是不可逆的（透镜、批注、对话都停在那里）。提前把这颗按钮摆出来，
  // 就是邀请她在读到一半时把这一篇终结掉。
  panel([task("1", "done"), task("2", "pending")]);
  expect(screen.queryByRole("button", { name: /完成阅读/ })).toBeNull();
});

it("还没有读法清单的时候也不出现", () => {
  // `tasks` 为空既可能是「计划还没生成」，也可能是「刚进房间」。两种都不是
  // 「走完了」——`every` 对空数组返回 true，所以这一条是真的会写错的地方。
  panel([]);
  expect(screen.queryByRole("button", { name: /完成阅读/ })).toBeNull();
});
