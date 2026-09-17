import { render, screen, fireEvent, waitFor, cleanup, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReadingCoachSlot } from "@lite/readings/ReadingRoom";
import type { LiteMessage } from "../src/api/readingRoom";

/**
 * 答过的卡片可以改答案（产品负责人 2026-09-17）。
 *
 * 这里测的是**配对**：改过的答案要挂回它自己那张卡上，而不能走「挂到最新那张
 * 没答的卡」那条退路 —— 那样会把一张她还没碰过的卡当成已答收起来。这件事读
 * `ReadingCoachPanel` 的 useMemo 是看不出对错的。
 */

const postTurn = vi.fn();
vi.mock("../src/api/readingRoom", async (orig) => ({
  ...(await orig<Record<string, unknown>>()),
  postReadingCoachTurn: (...args: unknown[]) => postTurn(...args),
}));

const { ReadingCoachPanel } = await import("../src/readings/ReadingCoachPanel");

const slot: ReadingCoachSlot = { locked: false, quotes: [], removeQuote: () => {}, clearQuotes: () => {} };

const CARD_A = {
  type: "choose_span",
  prompt: "哪一句最能说明羽毛的作用？",
  options: [
    { blockId: "b1", quote: "These feathers likely helped insulate the birds." },
    { blockId: "b2", quote: "Plants were unable to photosynthesize." },
  ],
};
const CARD_B = { type: "short_text", prompt: "用一句话说说你的判断。" };

function panel(messages: LiteMessage[]) {
  return render(
    <ReadingCoachPanel
      readingId="r1"
      tasks={[]}
      initialMessages={messages}
      slot={slot}
      onTasks={() => {}}
      onFocusBlock={() => {}}
      lensDone={null}
      onLensDoneSent={() => {}}
      onFinish={() => {}}
    />,
  );
}

beforeEach(() => {
  postTurn.mockReset();
  postTurn.mockResolvedValue({ reply: "好。", tasks: [], currentTaskId: "", focusBlock: "", tool: "", finished: false });
});
afterEach(cleanup);

const ai = (seq: number, card: unknown): LiteMessage =>
  ({ seq, role: "ai", content: "请看这张卡。", createdAt: "", payload: { card } }) as LiteMessage;
const her = (seq: number, answer: unknown): LiteMessage =>
  ({ seq, role: "student", content: "", createdAt: "", payload: { answer } }) as LiteMessage;

describe("改答案", () => {
  it("改过的答案挂回它自己那张卡，显示最新的那一份", () => {
    panel([
      ai(1, CARD_A),
      her(2, { type: "choose_span", prompt: CARD_A.prompt, choice: CARD_A.options[1]!.quote }),
      her(3, { type: "choose_span", prompt: CARD_A.prompt, choice: CARD_A.options[0]!.quote, revised: true }),
    ]);
    const card = document.querySelector('[data-coach-card="answered"]') as HTMLElement;
    expect(card).toBeTruthy();
    const mine = within(card).getByText("你选的").parentElement as HTMLElement;
    expect(mine.textContent).toContain(CARD_A.options[0]!.quote);
  });

  it("🚨 改过的答案不会把后面那张还没答的卡当成已答", () => {
    panel([
      ai(1, CARD_A),
      her(2, { type: "choose_span", prompt: CARD_A.prompt, choice: CARD_A.options[1]!.quote }),
      ai(3, CARD_B),
      her(4, { type: "choose_span", prompt: CARD_A.prompt, choice: CARD_A.options[0]!.quote, revised: true }),
    ]);
    // 卡 B 仍然敞开：写一句的那个框还在。
    expect(screen.getByRole("textbox", { name: CARD_B.prompt })).toBeTruthy();
    expect(document.querySelectorAll('[data-coach-card="open"]')).toHaveLength(1);
  });

  it("只有最新的那张卡能改", () => {
    panel([
      ai(1, CARD_A),
      her(2, { type: "choose_span", prompt: CARD_A.prompt, choice: CARD_A.options[1]!.quote }),
      ai(3, { ...CARD_B, type: "short_text" }),
      her(4, { type: "short_text", prompt: CARD_B.prompt, choice: "我觉得证据够。" }),
    ]);
    expect(screen.getAllByRole("button", { name: "修改答案" })).toHaveLength(1);
    // 那一颗在卡 B 上。
    const b = screen.getAllByText(CARD_B.prompt)[0]!.closest("[data-coach-card]") as HTMLElement;
    expect(within(b).getByRole("button", { name: "修改答案" })).toBeTruthy();
  });

  it("改答案：选项重新点得动，发出去的那一份带着 revised", async () => {
    panel([
      ai(1, CARD_A),
      her(2, { type: "choose_span", prompt: CARD_A.prompt, choice: CARD_A.options[1]!.quote }),
    ]);
    fireEvent.click(screen.getByRole("button", { name: "修改答案" }));
    fireEvent.click(screen.getByRole("button", { name: new RegExp(CARD_A.options[0]!.quote.slice(0, 20)) }));
    await waitFor(() => expect(postTurn).toHaveBeenCalled());
    const cardAnswer = postTurn.mock.calls[0]![3] as { choice: string; revised?: boolean; prompt: string };
    expect(cardAnswer.choice).toBe(CARD_A.options[0]!.quote);
    expect(cardAnswer.revised).toBe(true);
    expect(cardAnswer.prompt).toBe(CARD_A.prompt);
  });

  it("写一句的那种：改的时候框里是她上一次写的", () => {
    panel([ai(1, CARD_B), her(2, { type: "short_text", prompt: CARD_B.prompt, choice: "我觉得证据够。" })]);
    fireEvent.click(screen.getByRole("button", { name: "修改答案" }));
    expect((screen.getByRole("textbox", { name: CARD_B.prompt }) as HTMLTextAreaElement).value).toBe("我觉得证据够。");
    // 取消就回到答过的样子。
    fireEvent.click(screen.getByRole("button", { name: "取消修改" }));
    expect(screen.queryByRole("textbox", { name: CARD_B.prompt })).toBeNull();
  });
});
