import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReadingCoachSlot } from "@/studio/reading/ReadingRoom";
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
      />,
    );

    fireEvent.change(screen.getByPlaceholderText("读完这一步，跟印记说一声"), {
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

  it("shows the hunt hint only while the current step is a hunt", async () => {
    const huntTasks: ReadingTask[] = [
      { id: "t1", position: 0, kind: "hunt", label: "找一找", detail: "", blockId: "", status: "pending", completedAt: null },
    ];
    const { rerender } = render(
      <ReadingCoachPanel
        readingId="r1"
        tasks={huntTasks}
        initialMessages={STARTED_MESSAGES}
        slot={baseSlot()}
        onTasks={() => {}}
        onFocusBlock={() => {}}
      />,
    );
    expect(await screen.findByText("在文章里点出那一句，点了就会出现在这里")).toBeTruthy();

    const reflectTasks: ReadingTask[] = [
      { id: "t1", position: 0, kind: "reflect", label: "联系自己", detail: "", blockId: "", status: "pending", completedAt: null },
    ];
    rerender(
      <ReadingCoachPanel
        readingId="r1"
        tasks={reflectTasks}
        initialMessages={STARTED_MESSAGES}
        slot={baseSlot()}
        onTasks={() => {}}
        onFocusBlock={() => {}}
      />,
    );
    expect(screen.queryByText("在文章里点出那一句，点了就会出现在这里")).toBeNull();
  });
});
