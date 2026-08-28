import { render, screen, cleanup, fireEvent } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GuideBox } from "@lite/writings/GuideBox";
import type { WritingBlockGuide } from "@lite/api/writingRoom";

/**
 * GuideBox — pinned against the product verdict that reshaped it (Task 9):
 * *"our snippets is not real guidance, it is even not good as pro version"*
 * and *"a large paragraph of small texts is not easy to read."* These tests
 * pin the fix: four visually distinct parts, questions at reading size, and
 * a borrowed example that is unmistakably borrowed.
 */

afterEach(cleanup);

const GUIDE: WritingBlockGuide = {
  job: "这一段要让读者相信「便宜」这个说法不成立。",
  methods: [
    {
      name: "正反",
      definition: "一正一反两个例子放在一起。",
      examples: [{ topic: "两个菜市场", text: "东街留了装卸区…" }],
      patterns: [],
    },
  ],
  questions: ["你见过哪条街上的树长不开？", "这跟成本有什么关系？"],
};

describe("GuideBox", () => {
  it("renders all four parts, and questions at reading size", () => {
    render(<GuideBox guide={GUIDE} onDismiss={() => {}} onDeepen={() => {}} />);

    expect(screen.getByText(/这一段要做的事/)).toBeTruthy();
    expect(screen.getByText(/常见的几种写法/)).toBeTruthy();
    expect(screen.getByText("正反")).toBeTruthy();
    expect(screen.getByText(/一正一反两个例子/)).toBeTruthy();
    expect(screen.getByText(/想一想/)).toBeTruthy();
    expect(screen.getByRole("button", { name: /看几个例子/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: /深入一层/ })).toBeTruthy();

    // The old box put everything at mk-body 14px, which is what made it unreadable.
    expect(screen.getByText("你见过哪条街上的树长不开？").className).toContain("text-mk-body-lg");
    expect(screen.getByText("这跟成本有什么关系？").className).toContain("text-mk-body-lg");
  });

  it("keeps 收起 wired to onDismiss", () => {
    const onDismiss = vi.fn();
    render(<GuideBox guide={GUIDE} onDismiss={onDismiss} onDeepen={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: "收起" }));
    expect(onDismiss).toHaveBeenCalledOnce();
  });

  it("wires 深入一层 to onDeepen", () => {
    const onDeepen = vi.fn();
    render(<GuideBox guide={GUIDE} onDismiss={() => {}} onDeepen={onDeepen} />);
    fireEvent.click(screen.getByRole("button", { name: /深入一层/ }));
    expect(onDeepen).toHaveBeenCalledOnce();
  });

  it("shows the borrowed example only after she asks, with its topic named", () => {
    render(<GuideBox guide={GUIDE} onDismiss={() => {}} onDeepen={() => {}} />);

    // Not painted open by default — a borrowed example next to a live
    // question list would read as a suggestion unless she asked for it.
    expect(screen.queryByText(/东街留了装卸区/)).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: /看几个例子/ }));

    expect(screen.getByText(/东街留了装卸区/)).toBeTruthy();
    // The topic MUST be named beside the example — that is the whole
    // guarantee: an example about 两个菜市场 can never be mistaken for a
    // suggestion about her own piece.
    expect(screen.getByText(/两个菜市场/)).toBeTruthy();
  });

  it("does not render a method picker — no radio inputs, no selectable role for a method", () => {
    render(<GuideBox guide={GUIDE} onDismiss={() => {}} onDeepen={() => {}} />);
    expect(screen.queryAllByRole("radio")).toHaveLength(0);
    expect(screen.queryAllByRole("checkbox")).toHaveLength(0);
  });

  it("shows a method's sentence patterns, which is all an English method carries", () => {
    // The English half of vocab's library (en_concession / en_qualify /
    // en_evidence) ships FRAMES rather than worked examples. Gating 例子 on
    // `examples` alone left an English block with a button that never
    // appeared and teaching that never reached her.
    const english: WritingBlockGuide = {
      job: "Concede the strongest objection before answering it.",
      methods: [
        {
          name: "Conceding, then turning",
          definition: "Grant what is true, then say what it does not settle.",
          examples: [],
          patterns: [{ label: "Admit then limit", frame: "While it is true that ___, this does not mean ___." }],
        },
      ],
      questions: ["What is the strongest thing someone could say against you?"],
    };
    render(<GuideBox guide={english} onDismiss={() => {}} onDeepen={() => {}} />);

    fireEvent.click(screen.getByRole("button", { name: /看几个例子/ }));
    expect(screen.getByText(/While it is true that ___/)).toBeTruthy();
    // The blanks are the 铁律① line: the frame says what SHAPE the sentence
    // takes, and the part it will not write is exactly the part she fills in.
    expect(screen.getByText(/横线上的内容要你自己填/)).toBeTruthy();
  });

  it("omits parts that have nothing to show, rather than rendering an empty section", () => {
    const sparse: WritingBlockGuide = { job: "", methods: [], questions: ["只有一个问题？"] };
    render(<GuideBox guide={sparse} onDismiss={() => {}} onDeepen={() => {}} />);
    expect(screen.queryByText(/这一段要做的事/)).toBeNull();
    expect(screen.queryByText(/常见的几种写法/)).toBeNull();
    expect(screen.queryByRole("button", { name: /看几个例子/ })).toBeNull();
    expect(screen.getByText("只有一个问题？")).toBeTruthy();
  });
});
