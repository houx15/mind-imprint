import { render, screen, cleanup, fireEvent, waitFor } from "@testing-library/react";
import { useState } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { Writing } from "@lite/api/writings";
import type { WritingOutlineItem } from "@lite/api/writingRoom";
import type { LiteMessage } from "@lite/api/readingRoom";

/**
 * 印记 说「够写了」的那一刻，屏幕上要出现一条真的邀请。
 *
 * 🚨 产品负责人的原话（2026-09-04，回答「永远不会带领学生真正开启写作吗？」）：
 *
 *   > until student click the logic is good, ai never auto triggers and
 *   > guides students to start writing.
 *
 * 「去写」一直在页眉里可点、从来不是关卡——缺的是**有人提议**。系统提示词里
 * 本来就写着「有时候答案是什么都不缺，让她去写」，但那个判断没有任何通道能说
 * 出口，于是每一轮都被丢掉，学生只好继续回答下一个问题。
 *
 * 这里钉的是那条通道两端都接上了，而且**不越界**：
 *  1. ready → 出现一个能点的东西，点下去就是去写作那一步；
 *  2. 没 ready → 它不出现（不然它就成了每轮都在喊的噪音）；
 *  3. 🚨 它**不自动跳转**。结构不是关卡，把她推进段落是同一个错误的镜像。
 *  4. 说过一次就一直算数——她可能想再补一条理由再走。
 *
 * 断言按行为写（onDone 有没有被调用），不按按钮上的字：文案是产品负责人的。
 */

const postPlanTurn = vi.fn();
const postOpening = vi.fn();
vi.mock("../src/api/writingRoom", async (orig) => ({
  ...(await orig<Record<string, unknown>>()),
  postWritingPlanTurn: (...a: unknown[]) => postPlanTurn(...a),
  postWritingOpening: (...a: unknown[]) => postOpening(...a),
}));

const { PlanningView } = await import("../src/writings/PlanningView");

const WRITING = {
  id: "w1",
  title: "该不该把上学时间往后推？",
  lang: "zh",
  stage: "outline",
  targetWords: null,
  structureKey: "",
  setupAt: "2026-09-01T00:00:00Z",
  status: "active",
  createdAt: "2026-09-01T00:00:00Z",
  updatedAt: "2026-09-01T00:00:00Z",
  finishedAt: null,
} as unknown as Writing;

/**
 * PlanningView is CONTROLLED: `messages` comes from the host, so a no-op
 * `onMessages` means 印记's reply never reaches the screen. This tiny stateful
 * host is what makes the assertions about replies meaningful at all.
 */
function Host({ onDone }: { onDone: () => void }) {
  const [messages, setMessages] = useState<LiteMessage[]>([
    { seq: 1, role: "ai", content: "先说说你更倾向哪一边？", createdAt: "" },
  ]);
  const [outline, setOutline] = useState<WritingOutlineItem[]>([]);
  return (
    <PlanningView
      writing={WRITING}
      messages={messages}
      outline={outline}
      onMessages={setMessages}
      onOutline={setOutline}
      onDone={onDone}
      onBack={() => {}}
    />
  );
}

function view(onDone = vi.fn()) {
  render(<Host onDone={onDone} />);
  return onDone;
}

async function say(text: string) {
  fireEvent.change(screen.getByPlaceholderText("说说你的想法"), { target: { value: text } });
  fireEvent.click(screen.getByLabelText("发送"));
}

beforeEach(() => {
  postPlanTurn.mockReset();
  postOpening.mockReset();
  postOpening.mockResolvedValue({ reply: "", generated: false });
});
afterEach(cleanup);

it("印记 说够写了 → 出现邀请，点它就去写作", async () => {
  const onDone = view();
  postPlanTurn.mockResolvedValue({ reply: "这三条已经站得住了。", outline: [], addedIds: [], ready: true });

  await say("我想说三条理由。");

  const start = await screen.findByRole("button", { name: /开始写作/ });
  // 🚨 它没有自己跳走：结构不是关卡，反向也不是。
  expect(onDone).not.toHaveBeenCalled();

  fireEvent.click(start);
  expect(onDone).toHaveBeenCalledTimes(1);
});

it("还没 ready 就不出现", async () => {
  view();
  postPlanTurn.mockResolvedValue({ reply: "第二条理由能再具体一点吗？", outline: [], addedIds: [], ready: false });

  await say("我先说一条。");

  await screen.findByText("第二条理由能再具体一点吗？");
  expect(screen.queryByRole("button", { name: /开始写作/ })).toBeNull();
});

it("说过一次就一直算数——她还想再补一条，邀请不该被收回去", async () => {
  view();
  postPlanTurn.mockResolvedValueOnce({ reply: "够写了。", outline: [], addedIds: [], ready: true });
  await say("三条理由。");
  await screen.findByRole("button", { name: /开始写作/ });

  postPlanTurn.mockResolvedValueOnce({ reply: "这条也记下了。", outline: [], addedIds: [], ready: false });
  await say("我再补一条。");
  await screen.findByText("这条也记下了。");

  await waitFor(() => expect(screen.queryByRole("button", { name: /开始写作/ })).not.toBeNull());
});
