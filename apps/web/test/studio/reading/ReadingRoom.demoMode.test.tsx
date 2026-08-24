import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { MaterialSource } from "@mind-imprint/contracts";
import { ReadingRoom } from "@/studio/reading/ReadingRoom";
import { demoReadingTranscript } from "@/tour/fixtures/demoReadingTranscript";

const SOURCE: MaterialSource = {
  id: "m1", title: "Chen et al. 2019 — Nature Sustainability", sourceUrl: "", kind: "article", origin: "nature.com",
  blocks: [{ id: "b0", text: "全球平均叶面积指数净增约 5%。" }, { id: "b1", text: "中国与印度合计贡献超过三分之一。" }],
  locked: false, role: "", tier: "", takeaway: "", anchors: [], timeSpentS: 0,
  lateralRead: false, isLateralInstrument: false, siftSkipped: false, lateralRelation: "", lateralJudgment: "",
};

// A NOOP api slice — the demo-mode test never lets any of these fire (that's
// exactly what it's verifying), so every call throws if actually invoked.
const NOOP_API = {
  readTurn: async function* () {
    throw new Error("readTurn must not fire in demoMode");
  },
  summonCard: async function* () {
    throw new Error("summonCard must not fire in demoMode");
  },
  activateProjectCard: async () => {
    throw new Error("not used in this test");
  },
  evaluateCardSelection: async () => {
    throw new Error("not used in this test");
  },
  submitProjectCard: async function* () {},
  skipProjectCard: async () => {},
  putReadingBrief: vi.fn(async () => {
    throw new Error("putReadingBrief must not fire in demoMode");
  }),
  getTakeawayDraft: async () => {
    throw new Error("not used in this test");
  },
  postFinalizeReading: async () => {
    throw new Error("not used in this test");
  },
};

describe("ReadingRoom — demo replay (guided tour P6, Task 4)", () => {
  it("renders the seeded transcript instead of the live GREETING", () => {
    render(
      <ReadingRoom
        projectId="p1"
        referenceId="r1"
        source={SOURCE}
        onBack={() => {}}
        api={NOOP_API}
        initialMessages={demoReadingTranscript}
        demoMode
      />,
    );

    // The seeded transcript renders — not the live GREETING ("文章已经准备好了").
    expect(screen.getByText(/这个说法能直接信吗/)).toBeInTheDocument();
    expect(screen.getByText(/农业集约化（约 32%）\+ 人工造林（约 42%）/)).toBeInTheDocument();
    expect(screen.getByText(/得写成「Chen et al\. 发现中国的植被覆盖净增长/)).toBeInTheDocument();
    expect(screen.queryByText(/文章已经准备好了/)).toBeNull();
  });

  it("disables the compose input and send button so no readTurn can fire", () => {
    render(
      <ReadingRoom
        projectId="p1"
        referenceId="r1"
        source={SOURCE}
        onBack={() => {}}
        api={NOOP_API}
        initialMessages={demoReadingTranscript}
        demoMode
      />,
    );

    const textarea = screen.getByLabelText("输入你的问题");
    const sendBtn = screen.getByRole("button", { name: "发送" });
    expect(textarea).toBeDisabled();
    expect(sendBtn).toBeDisabled();

    // The starter prompts and 透镜库 (which drive summonCard) are disabled too.
    expect(screen.getByRole("button", { name: "这条来源可信吗？" })).toBeDisabled();
    expect(screen.getByRole("button", { name: /透镜库/ })).toBeDisabled();
  });

  it("disables the 完成这篇 finalize button and the 我的笔记 textarea", () => {
    render(
      <ReadingRoom
        projectId="p1"
        referenceId="r1"
        source={SOURCE}
        onBack={() => {}}
        api={NOOP_API}
        initialMessages={demoReadingTranscript}
        onSaveNote={async () => {
          throw new Error("onSaveNote must not fire in demoMode");
        }}
        demoMode
      />,
    );

    expect(screen.getByRole("button", { name: "完成这篇" })).toBeDisabled();

    // 我的笔记 is collapsed by default — open it, then check the textarea.
    fireEvent.click(screen.getByRole("button", { name: /我的笔记/ }));
    expect(screen.getByPlaceholderText(/随手记下你自己的想法/)).toBeDisabled();
  });

  it("seeds 我的笔记 from readingNote and keeps it read-only (P8 Task 7 demo carry-through)", () => {
    const seededNote =
      "读到这里先记一笔：论文用 NASA MODIS 2000–2017 的数据说全球绿叶面积净增约 5%……（migration 0091 seed）";
    render(
      <ReadingRoom
        projectId="p1"
        referenceId="r1"
        source={SOURCE}
        onBack={() => {}}
        api={NOOP_API}
        initialMessages={demoReadingTranscript}
        readingNote={seededNote}
        onSaveNote={async () => {
          throw new Error("onSaveNote must not fire in demoMode");
        }}
        demoMode
      />,
    );

    // A non-empty readingNote opens 我的笔记 by default (no click needed) and
    // the textarea shows the REAL seeded text, not a blank placeholder.
    const noteTextarea = screen.getByPlaceholderText(/随手记下你自己的想法/);
    expect(noteTextarea).toHaveValue(seededNote);
    expect(noteTextarea).toBeDisabled();
  });

  it("falls back to the live GREETING when initialMessages is omitted (non-demo default unchanged)", () => {
    render(<ReadingRoom projectId="p1" referenceId="r1" source={SOURCE} onBack={() => {}} api={NOOP_API} />);
    expect(screen.getByText(/文章已经准备好了/)).toBeInTheDocument();
    expect(screen.getByLabelText("输入你的问题")).not.toBeDisabled();
  });
});
