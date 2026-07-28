import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import type { MaterialSource } from "@mind-imprint/contracts";
import { ReadingRoom } from "@/studio/reading/ReadingRoom";

const SOURCE: MaterialSource = {
  id: "m1", title: "NASA 气候报告", sourceUrl: "", kind: "article", origin: "nasa.gov",
  blocks: [
    { id: "b0", text: "过去二十年，卫星图显示地球在变绿。" },
    { id: "b1", text: "因此这项政策必然失败。" },
  ],
  locked: false, role: "", tier: "", takeaway: "", anchors: [], timeSpentS: 0,
  lateralRead: false, isLateralInstrument: false, siftSkipped: false, lateralRelation: "", lateralJudgment: "",
};

const EXAMPLE_ANCHOR = {
  id: "ex0",
  material_id: "m1",
  block_id: "b1",
  start: 0,
  end: 10,
  quote: "因此这项政策必然失败",
  dimension: "argument-map",
  author: "ai" as const,
  question: "先看这处示范：结论没给证据",
  answer: "",
};

function makeFakeApi() {
  return {
    readTurn: vi.fn(async function* () {
      yield {
        type: "card" as const,
        cardInstanceId: "ci1",
        cardId: "argument-map",
        nudgeText: "这句像结论没给证据",
        anchors: [EXAMPLE_ANCHOR],
        materialId: "m1",
      };
      yield { type: "done" as const };
    }),
    summonCard: vi.fn(async function* () {
      yield {
        type: "card" as const,
        cardInstanceId: "ci1",
        cardId: "argument-map",
        nudgeText: "先看这处示范，再换你自己的一句",
        anchors: [EXAMPLE_ANCHOR],
        materialId: "m1",
      };
      yield { type: "done" as const };
    }),
    activateProjectCard: vi.fn(async () => {}),
    evaluateCardSelection: vi.fn(async () => ({
      verdict: "rethink" as const,
      verdictLabel: "暂不匹配",
      verdictReason: "这句是概括，不是证据。",
      checks: [],
      finding: "f",
      judgment: "",
      support: "",
      caveat: "",
      nextStep: "再找一句更具体的证据",
      spanIds: ["sel0"],
    })),
    submitProjectCard: vi.fn(async function* () {
      yield { type: "done" as const, cardStatus: "completed" };
    }),
    skipProjectCard: vi.fn(async () => {}),
  };
}

// Mirrors Annotate.test.tsx's own DOM-selection technique: construct a real
// Range over a block's first text run and fire the mouseUp Annotate's
// select-mode listens on, so the whole pick goes through the actual
// selection→span pipeline rather than calling an internal handler directly.
function selectTextInBlock(container: HTMLElement, blockId: string, start: number, end: number) {
  const blockEl = container.querySelector(`[data-block-id="${blockId}"]`)!;
  const textNode = blockEl.firstChild!.firstChild as Text;
  const range = document.createRange();
  range.setStart(textNode, start);
  range.setEnd(textNode, end);
  const sel = window.getSelection()!;
  sel.removeAllRanges();
  sel.addRange(range);
  fireEvent.mouseUp(blockEl.parentElement!);
}

describe("ReadingRoom — read-together loop", () => {
  it("send → example → pick → feedback → confirm → outcome lands in 阅读成果", async () => {
    const fakeApi = makeFakeApi();
    const { container } = render(<ReadingRoom projectId="p1" source={SOURCE} onBack={() => {}} api={fakeApi as any} />);

    // The 阅读成果 tab starts at 0.
    expect(screen.getByRole("tab", { name: /阅读成果/ })).toHaveTextContent("0");

    // Send a coach turn — the fake readTurn yields a "card" event.
    fireEvent.change(screen.getByPlaceholderText(/说说你对哪一句有疑问/), { target: { value: "这段怪怪的" } });
    fireEvent.click(screen.getByLabelText("发送"));

    // The example appears (proposed): the hanging card's own copy, plus the
    // student's own turn is now in the dialogue log.
    await screen.findByText("看懂示范，开始选句");
    expect(screen.getByText("这段怪怪的")).toBeInTheDocument();
    expect(fakeApi.readTurn).toHaveBeenCalledWith("p1", "m1", { student_text: "这段怪怪的", focused_spans: [] });

    // Start pick → active: the article enters select-mode.
    fireEvent.click(screen.getByText("看懂示范，开始选句"));
    await screen.findByText(/在文章里选出你要用来回答「论证地图卡/);
    expect(screen.queryByText(/argument-map/)).not.toBeInTheDocument();
    expect(fakeApi.activateProjectCard).toHaveBeenCalledWith("p1", "ci1");

    // Select a DIFFERENT sentence than the example (block b0, not b1).
    await act(async () => {
      selectTextInBlock(container, "b0", 0, 5); // "过去二十年"
    });

    await screen.findByText("暂不匹配");
    expect(fakeApi.evaluateCardSelection).toHaveBeenCalledWith("p1", "ci1", {
      block_id: "b0", start: 0, end: 5, quote: "过去二十年", dimension: "argument-map",
    });
    expect(screen.getByText("这句是概括，不是证据。")).toBeInTheDocument();

    // Confirm — submits the student's anchor and retires the card.
    fireEvent.click(screen.getByText("记下这条发现"));
    await act(async () => {
      await Promise.resolve();
    });

    expect(fakeApi.submitProjectCard).toHaveBeenCalledWith("p1", "ci1", {
      field_values: {},
      event_trace: [],
      anchors: [
        {
          id: "sel0", material_id: "m1", block_id: "b0", start: 0, end: 5,
          quote: "过去二十年", dimension: "argument-map", author: "student", question: "", answer: "",
        },
      ],
    });
    // The card is gone from the article…
    expect(screen.queryByText("记下这条发现")).not.toBeInTheDocument();
    // …and the 阅读成果 tab count incremented to 1.
    expect(screen.getByRole("tab", { name: /阅读成果/ })).toHaveTextContent("1");

    // Switching to the 阅读成果 view shows the saved finding + selected quote.
    fireEvent.click(screen.getByRole("tab", { name: /阅读成果/ }));
    expect(screen.getByText("我的阅读成果")).toBeInTheDocument();
    expect(screen.getByText("f")).toBeInTheDocument(); // outcome.finding
    expect(screen.getByText("过去二十年")).toBeInTheDocument(); // the selected quote
    expect(screen.getByText("回到原文继续思考")).toBeInTheDocument();
  });

  it("ignores a pick that exactly matches the example sentence", async () => {
    const fakeApi = makeFakeApi();
    const { container } = render(<ReadingRoom projectId="p1" source={SOURCE} onBack={() => {}} api={fakeApi as any} />);

    fireEvent.change(screen.getByPlaceholderText(/说说你对哪一句有疑问/), { target: { value: "这段怪怪的" } });
    fireEvent.click(screen.getByLabelText("发送"));
    await screen.findByText("看懂示范，开始选句");
    fireEvent.click(screen.getByText("看懂示范，开始选句"));
    await screen.findByText(/在文章里选出你要用来回答「论证地图卡/);

    // Same block + same range as the example anchor (b1, 0-10).
    await act(async () => {
      selectTextInBlock(container, "b1", 0, 10);
    });

    expect(fakeApi.evaluateCardSelection).not.toHaveBeenCalled();
    // Still in select-mode — no feedback rendered.
    expect(screen.queryByText("暂不匹配")).not.toBeInTheDocument();
    expect(screen.getByText(/在文章里选出你要用来回答/)).toBeInTheDocument();
  });

  it("clicking a sentence references it, shows the chip, and sends it as focused_spans", async () => {
    const fakeApi = makeFakeApi();
    const { container } = render(<ReadingRoom projectId="p1" source={SOURCE} onBack={() => {}} api={fakeApi as any} />);

    // Idle by default — the hint invites referencing.
    expect(screen.getByText("点击句子可引用原文")).toBeInTheDocument();
    expect(screen.queryByText(/正在引用/)).not.toBeInTheDocument();

    const block0 = container.querySelector('[data-block-id="b0"]')!;
    fireEvent.click(block0);

    // The chip appears with a count of 1, and the hint updates.
    expect(screen.getByText(/正在引用/)).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("已引用 1 处 · 再点可取消")).toBeInTheDocument();

    // Reference a second block too.
    const block1 = container.querySelector('[data-block-id="b1"]')!;
    fireEvent.click(block1);
    expect(screen.getByText("已引用 2 处 · 再点可取消")).toBeInTheDocument();

    // Sending includes both referenced blocks as focused_spans, then clears them.
    fireEvent.change(screen.getByPlaceholderText(/说说你对哪一句有疑问/), { target: { value: "这段怪怪的" } });
    fireEvent.click(screen.getByLabelText("发送"));

    await screen.findByText("看懂示范，开始选句");
    expect(fakeApi.readTurn).toHaveBeenCalledWith("p1", "m1", {
      student_text: "这段怪怪的",
      focused_spans: [
        { block_id: "b0", quote: "过去二十年，卫星图显示地球在变绿。" },
        { block_id: "b1", quote: "因此这项政策必然失败。" },
      ],
    });

    // The chip is gone after sending — the reference set was cleared.
    expect(screen.queryByText(/正在引用/)).not.toBeInTheDocument();
  });

  it("清除 clears referenced blocks without sending", async () => {
    const fakeApi = makeFakeApi();
    const { container } = render(<ReadingRoom projectId="p1" source={SOURCE} onBack={() => {}} api={fakeApi as any} />);

    const block0 = container.querySelector('[data-block-id="b0"]')!;
    fireEvent.click(block0);
    expect(screen.getByText(/正在引用/)).toBeInTheDocument();

    fireEvent.click(screen.getByText("清除"));
    expect(screen.queryByText(/正在引用/)).not.toBeInTheDocument();
    expect(screen.getByText("点击句子可引用原文")).toBeInTheDocument();
    expect(fakeApi.readTurn).not.toHaveBeenCalled();
  });

  it("透镜库: browsing lists the deck, and picking a card summons it onto the article", async () => {
    const fakeApi = makeFakeApi();
    render(<ReadingRoom projectId="p1" source={SOURCE} onBack={() => {}} api={fakeApi as any} />);

    // Opening the library lists the reading deck, grouped — a source-check
    // card (CRAAP) and a deep-reading card (论证地图) are both visible.
    fireEvent.click(screen.getByRole("button", { name: /透镜库/ }));
    expect(screen.getByRole("dialog", { name: "透镜库" })).toBeInTheDocument();
    expect(screen.getByText("信源辨识卡 CRAAP / CRRAAB")).toBeInTheDocument();
    expect(screen.getByText("论证地图卡（结构 + 谬误）")).toBeInTheDocument();

    // Picking a card calls summonCard and closes the library.
    fireEvent.click(screen.getByText("论证地图卡（结构 + 谬误）"));
    expect(fakeApi.summonCard).toHaveBeenCalledWith("p1", "m1", "argument-map");
    expect(screen.queryByRole("dialog", { name: "透镜库" })).not.toBeInTheDocument();

    // The loop goes to "proposed" — the hanging card appears, same as a
    // router-proposed summon would render.
    await screen.findByText("看懂示范，开始选句");
  });

  it("透镜库 button is disabled while a card is in flight", async () => {
    const fakeApi = makeFakeApi();
    render(<ReadingRoom projectId="p1" source={SOURCE} onBack={() => {}} api={fakeApi as any} />);

    fireEvent.change(screen.getByPlaceholderText(/说说你对哪一句有疑问/), { target: { value: "这段怪怪的" } });
    fireEvent.click(screen.getByLabelText("发送"));
    await screen.findByText("看懂示范，开始选句");

    expect(screen.getByRole("button", { name: /透镜库/ })).toBeDisabled();
  });
});
