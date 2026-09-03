import { render, waitFor, cleanup } from "@testing-library/react";
import { StrictMode } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { ReadingCoachSlot } from "@lite/readings/ReadingRoom";
import type { ReadingLensDone } from "../src/api/readingRoom";

/**
 * 她做完一副透镜 → 印记 必须回应一次，而且**只回应一次**。
 *
 * 这个文件测的是「读代码看不出对错」的那一类东西，不是渲染：
 *
 *  1. 一副做完的透镜真的会变成一轮 `postReadingCoachTurn`，而且带着 `lensDone`
 *     ——原本的 bug 就是这一轮**根本没有发生**（共用的 `confirm()` 把「已保存」
 *     写进了 lite 不渲染的 `loop.messages`）。
 *  2. 🚨 **StrictMode 下只发一次。** 这是这个文件真正的理由。`main.tsx` 用
 *     StrictMode 渲染，effect 会跑两遍；而 `turn` 开头的 `if (busy)` 挡不住第二
 *     遍——第二遍读到的是同一个 render 闭包里的旧 `busy`（false）。挡住它的是
 *     `lensSentRef`。这是一次旗舰调用，发两次就是收两条回复、付两次钱，
 *     而它在页面上看不出来（两条回复看着就像 印记 话多）。
 *  3. `text` 必须是空的、`lensDone` 必须是满的。服务端靠这个组合分辨这一轮
 *     不是「她刚点了开始」——反了的话 印记 会从头把读法清单再介绍一遍
 *     （PBL 房间踩过的那颗雷）。
 */

const postTurn = vi.fn();
vi.mock("../src/api/readingRoom", async (orig) => ({
  ...(await orig<Record<string, unknown>>()),
  postReadingCoachTurn: (...args: unknown[]) => postTurn(...args),
}));

const { ReadingCoachPanel } = await import("../src/readings/ReadingCoachPanel");

const LENS: ReadingLensDone = {
  cardName: "溯源体检",
  quote: "但人均排放仍低于多数发达国家。",
  finding: "这一句把总量和人均分开了，是一次口径切换。",
};

function slot(): ReadingCoachSlot {
  return { locked: false, quotes: [], removeQuote: () => {}, clearQuotes: vi.fn() };
}

function panel(lensDone: ReadingLensDone | null, onLensDoneSent = () => {}) {
  return (
    <ReadingCoachPanel
      readingId="r1"
      tasks={[]}
      initialMessages={[{ seq: 1, role: "ai", content: "我们看第二段。", createdAt: "" }]}
      slot={slot()}
      onTasks={() => {}}
      onFocusBlock={() => {}}
      lensDone={lensDone}
      onLensDoneSent={onLensDoneSent}
      onFinish={() => {}}
    />
  );
}

beforeEach(() => {
  postTurn.mockReset();
  postTurn.mockResolvedValue({
    reply: "你挑的这句正好是换口径的地方。",
    tasks: [],
    currentTaskId: "",
    focusBlock: "",
    tool: "",
    finished: false,
    card: null,
    nudge: "",
    coachCard: null,
    thinking: "",
  });
});
afterEach(cleanup);

it("一副做完的透镜变成一轮带 lensDone 的对话", async () => {
  render(panel(LENS));
  await waitFor(() => expect(postTurn).toHaveBeenCalled());

  const [id, text, picks, cardAnswer, lensDone] = postTurn.mock.calls[0];
  expect(id).toBe("r1");
  // 🚨 空文本 + 满的 lensDone。见文件头第 3 点。
  expect(text).toBe("");
  expect(picks).toEqual([]);
  expect(cardAnswer).toBeNull();
  expect(lensDone).toEqual(LENS);
});

it("🚨 StrictMode 下这一轮只买一次", async () => {
  render(<StrictMode>{panel(LENS)}</StrictMode>);
  await waitFor(() => expect(postTurn).toHaveBeenCalled());
  // effect 跑两遍、`busy` 在第二遍读到的是旧值——挡住第二次调用的只有
  // `lensSentRef`。删掉那个 ref，这里会变成 2。
  expect(postTurn).toHaveBeenCalledTimes(1);
});

it("送完之后通知房间清掉它", async () => {
  const sent = vi.fn();
  render(panel(LENS, sent));
  await waitFor(() => expect(sent).toHaveBeenCalledTimes(1));
});

it("这一轮失败了也要通知房间，不然它会一直重投", async () => {
  // 成果本身早就存下来了；对着一个挂掉的服务端无限重试，只会让她每次
  // 重新打开这篇文章都再挨一次失败。
  postTurn.mockRejectedValue(new Error("boom"));
  const sent = vi.fn();
  render(panel(LENS, sent));
  await waitFor(() => expect(sent).toHaveBeenCalledTimes(1));
});

it("没有透镜就不发", async () => {
  render(panel(null));
  // 给 effect 一次真正跑完的机会，再断言什么都没发生。
  await waitFor(() => expect(postTurn).not.toHaveBeenCalled());
});
