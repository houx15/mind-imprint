import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReadingCoachSlot } from "@lite/readings/ReadingRoom";
import type { ReadingTask } from "../src/api/readingRoom";

/**
 * ReadingCoachPanel — pointing vs. typing.
 *
 * Task 6 taught the server to tell a sentence she POINTED AT in the article
 * apart from one she merely typed. This is the frontend half: the quote
 * chips already carry which paragraph each one came from (the drag-select
 * handler in ReadingRoom.tsx knows that), so `send()` should hand it to the
 * coach endpoint as structured `picks` — WITHOUT dropping the existing
 * blockquote-inlined text a server that ignores `picks` still needs to see.
 */

const postTurn = vi.fn();
vi.mock("../src/api/readingRoom", async (orig) => ({
  ...(await orig<Record<string, unknown>>()),
  postReadingCoachTurn: (...args: unknown[]) => postTurn(...args),
}));

const { ReadingCoachPanel } = await import("../src/readings/ReadingCoachPanel");

const STARTED_MESSAGES = [{ seq: 1, role: "ai", content: "开始吧，我们先看第一段。", createdAt: "" }];

function baseSlot(overrides: Partial<ReadingCoachSlot> = {}): ReadingCoachSlot {
  return {
    locked: false,
    quotes: [],
    removeQuote: () => {},
    clearQuotes: vi.fn(),
    ...overrides,
  };
}

beforeEach(() => {
  postTurn.mockReset();
  postTurn.mockResolvedValue({
    reply: "好，接着说。",
    tasks: [],
    currentTaskId: "",
    focusBlock: "",
    tool: "",
    finished: false,
    card: null,
    nudge: "",
  });
});

afterEach(cleanup);

describe("ReadingCoachPanel — picks", () => {
  it("sends the quoted sentences as structured picks, and still inlines them in the text", async () => {
    const slot = baseSlot({
      quotes: [{ key: "q1", quote: "碳排放总量位居世界第一", blockId: "b2" }],
    });
    render(
      <ReadingCoachPanel
        readingId="r1"
        tasks={[]}
        initialMessages={STARTED_MESSAGES}
        slot={slot}
        onTasks={() => {}}
        onFocusBlock={() => {}}
        lensDone={null}
        onLensDoneSent={() => {}}
        onFinish={() => {}}
      />,
    );

    fireEvent.change(screen.getByPlaceholderText("请输入你的回答或问题"), {
      target: { value: "我觉得是这句" },
    });
    fireEvent.click(screen.getByLabelText("发送"));

    await waitFor(() => expect(postTurn).toHaveBeenCalled());
    const [id, text, picks] = postTurn.mock.calls[0] as [string, string, unknown];
    expect(id).toBe("r1");
    expect(text).toContain("> 碳排放总量位居世界第一");
    expect(text).toContain("我觉得是这句");
    expect(picks).toEqual([{ blockId: "b2", quote: "碳排放总量位居世界第一" }]);
  });

  it("sends on an EMPTY composer when a quote chip is present — pointing alone must count", async () => {
    const slot = baseSlot({
      quotes: [{ key: "q1", quote: "碳排放总量位居世界第一", blockId: "b2" }],
    });
    render(
      <ReadingCoachPanel
        readingId="r1"
        tasks={[]}
        initialMessages={STARTED_MESSAGES}
        slot={slot}
        onTasks={() => {}}
        onFocusBlock={() => {}}
        lensDone={null}
        onLensDoneSent={() => {}}
        onFinish={() => {}}
      />,
    );

    // No typing at all — the composer stays empty.
    fireEvent.click(screen.getByLabelText("发送"));

    await waitFor(() => expect(postTurn).toHaveBeenCalled());
    const [id, text, picks] = postTurn.mock.calls[0] as [string, string, unknown];
    expect(id).toBe("r1");
    expect(text.trim()).not.toBe("");
    expect(text).toContain("碳排放总量位居世界第一");
    expect(picks).toEqual([{ blockId: "b2", quote: "碳排放总量位居世界第一" }]);
  });

  it("F1: prefixes EVERY line of a multi-line quote, not just the first", async () => {
    // A drag-selection across a hard-wrapped paragraph (no blank line
    // between its lines) can return a quote whose text contains an internal
    // "\n". If only the first line carries "> ", the article's later lines
    // sit in the stored message unprefixed, where the server's
    // stripQuotedLines (report_facts.go) — which only strips lines already
    // starting with "> " — cannot catch them, and they can reach a "her
    // words only" corpus. See report_facts.go's stripArticleLines for the
    // server-side half of this same fix.
    const slot = baseSlot({
      quotes: [{ key: "q1", quote: "中国的碳排放总量位居世界第一。\n但人均排放仍低于多数发达国家。", blockId: "b2" }],
    });
    render(
      <ReadingCoachPanel
        readingId="r1"
        tasks={[]}
        initialMessages={STARTED_MESSAGES}
        slot={slot}
        onTasks={() => {}}
        onFocusBlock={() => {}}
        lensDone={null}
        onLensDoneSent={() => {}}
        onFinish={() => {}}
      />,
    );

    fireEvent.click(screen.getByLabelText("发送"));

    await waitFor(() => expect(postTurn).toHaveBeenCalled());
    const [, text] = postTurn.mock.calls[0] as [string, string, unknown];
    // Every line of the payload's quote block must carry "> " — none of the
    // article's lines may reach the server unprefixed.
    const quoteLines = text.split("\n").filter((l) => l.trim() !== "");
    for (const line of quoteLines) {
      expect(line.startsWith("> ")).toBe(true);
    }
    expect(text).toContain("> 中国的碳排放总量位居世界第一。");
    expect(text).toContain("> 但人均排放仍低于多数发达国家。");
  });

  /**
   * R4 (4) —— 「划选那一句」这条指令**属于卡片**，不属于面板。
   *
   * 面板上原来常驻着一条几乎一模一样的话（「在文章里点出那一句，点了就会出现在
   * 这里」），它跟着 hunt 这一步走：pick_in_article 卡片开着的时候，同一句话在
   * 屏幕上下叠着出现两遍；而她答完卡片之后，没有任何一张卡片还在等她点，那条
   * 指令却还赖在输入框上面。指令跟着卡片走，才会在卡片收走时一起收走。
   */
  it("does not carry a pointing instruction of its own when no card is open", async () => {
    const huntTasks: ReadingTask[] = [
      { id: "t1", position: 0, kind: "hunt", label: "找一找", detail: "", blockId: "", status: "pending", completedAt: null },
    ];
    render(
      <ReadingCoachPanel
        readingId="r1"
        tasks={huntTasks}
        initialMessages={STARTED_MESSAGES}
        slot={baseSlot()}
        onTasks={() => {}}
        onFocusBlock={() => {}}
        lensDone={null}
        onLensDoneSent={() => {}}
        onFinish={() => {}}
      />,
    );
    await screen.findByText("开始吧，我们先看第一段。");
    // 当前这一步就是 hunt，而这一轮没有卡片 —— 屏幕上不该有任何一句在要她点。
    expect(screen.queryByText(/划选那一句/)).toBeNull();
  });

  it("leaves the pointing instruction to the pick_in_article card, exactly once", async () => {
    const huntTasks: ReadingTask[] = [
      { id: "t1", position: 0, kind: "hunt", label: "找一找", detail: "", blockId: "", status: "pending", completedAt: null },
    ];
    render(
      <ReadingCoachPanel
        readingId="r1"
        tasks={huntTasks}
        initialMessages={[
          ...STARTED_MESSAGES,
          {
            seq: 2,
            role: "ai",
            content: "来，找一句。",
            createdAt: "",
            payload: { card: { type: "pick_in_article", prompt: "哪一句让你改了主意？" } },
          },
        ]}
        slot={baseSlot()}
        onTasks={() => {}}
        onFocusBlock={() => {}}
        lensDone={null}
        onLensDoneSent={() => {}}
        onFinish={() => {}}
      />,
    );
    expect(await screen.findAllByText(/划选那一句/)).toHaveLength(1);
  });
});
