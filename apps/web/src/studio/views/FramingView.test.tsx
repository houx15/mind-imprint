import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { FramingView } from "./FramingView";
import type { FramingFx } from "../state";

// Hand-built FramingFx (rather than STUDIO_FIXTURE, whose framing panel is
// already mid-project) so these assertions are independent of fixture drift —
// mirrors OnboardingView.test.tsx's makeData() convention.
function makeData(overrides: Partial<FramingFx> = {}): FramingFx {
  return {
    researchQuestion: "中国在多大程度上让世界变得更具环境可持续性？",
    terms: [],
    answers: [],
    searchPlan: [],
    ...overrides,
  };
}

describe("FramingView (S1 立题)", () => {
  it("renders the research question banner", () => {
    const data = makeData();
    render(<FramingView data={data} />);
    expect(screen.getByText(data.researchQuestion)).toBeInTheDocument();
    expect(screen.getByText("先定义，再动笔")).toBeInTheDocument();
  });

  // The chip is the design's own rune-count rule (dc.html:2153). CJK must
  // count as runes, not bytes/UTF-16 units — a 15-character Chinese
  // definition is 可检验, and this deliberately checks BOTH sides of that
  // boundary with real Chinese text (not long-vs-short ASCII, which would
  // pass under a wrong counting method and protect nothing).
  it("shows 待定义 / 偏模糊，再具体点 / ✓ 可检验 by rune-counted definition length", () => {
    const just14 = "一二三四五六七八九十一二三四"; // 14 runes — below threshold
    const just15 = "一二三四五六七八九十一二三四五"; // 15 runes — at threshold
    const data = makeData({
      terms: [
        { term: "空的", definition: "" },
        { term: "偏模糊", definition: just14 },
        { term: "可检验", definition: just15 },
      ],
    });
    render(<FramingView data={data} />);
    expect(screen.getByText("待定义")).toBeInTheDocument();
    expect(screen.getByText("偏模糊，再具体点")).toBeInTheDocument();
    expect(screen.getByText("✓ 可检验")).toBeInTheDocument();
  });

  it("lets her add and remove a key term", () => {
    const data = makeData({ terms: [{ term: "可持续性", definition: "" }] });
    render(<FramingView data={data} />);
    expect(screen.getAllByLabelText("关键词")).toHaveLength(1);

    fireEvent.click(screen.getByText("添加一个关键词"));
    expect(screen.getAllByLabelText("关键词")).toHaveLength(2);

    fireEvent.click(screen.getAllByLabelText("删除这个关键词")[0]!);
    expect(screen.getAllByLabelText("关键词")).toHaveLength(1);
  });

  it("reports the gate count off the 15-rune threshold", () => {
    const long = "这是一句足够长、可以被判定为可检验的定义内容超过十五个字";
    const data = makeData({
      terms: [
        { term: "a", definition: long },
        { term: "b", definition: long },
        { term: "c", definition: "太短" },
      ],
    });
    render(<FramingView data={data} />);
    expect(screen.getByText("本环节门禁 · 已定义 2/3")).toBeInTheDocument();
  });

  // Minor 2 (whole-branch review): the gate only ever asks for 3 — five good
  // definitions must still cap the numerator at 3, not overshoot to 5/3.
  it("caps the gate count numerator at 3 even with more than 3 good definitions", () => {
    const long = "这是一句足够长、可以被判定为可检验的定义内容超过十五个字";
    const data = makeData({
      terms: [
        { term: "a", definition: long },
        { term: "b", definition: long },
        { term: "c", definition: long },
        { term: "d", definition: long },
        { term: "e", definition: long },
      ],
    });
    render(<FramingView data={data} />);
    expect(screen.getByText("本环节门禁 · 已定义 3/3")).toBeInTheDocument();
  });

  // I4 (whole-branch review IMPORTANT): the server silently drops any term
  // row whose trimmed `term` is blank. Before this fix, that row stayed on
  // screen with her definition and its quality chip, looking saved, with no
  // record it was ever dropped.
  it("drops a term row with a blank term name from the screen after a successful save", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    const data = makeData({
      terms: [
        { term: "", definition: "写了很长的定义但忘记写关键词本身叫什么名字" },
        { term: "可持续性", definition: "这个我写完整了" },
      ],
    });
    render(<FramingView data={data} onSubmit={onSubmit} />);
    expect(screen.getByDisplayValue("写了很长的定义但忘记写关键词本身叫什么名字")).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "记下我的立题" }));
    });

    expect(screen.queryByDisplayValue("写了很长的定义但忘记写关键词本身叫什么名字")).not.toBeInTheDocument();
    expect(screen.getByDisplayValue("这个我写完整了")).toBeInTheDocument();
  });

  it("saves partial work — a single filled term submits fine", async () => {
    const data = makeData({ terms: [{ term: "可持续性", definition: "只写了一半" }] });
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<FramingView data={data} onSubmit={onSubmit} />);
    const button = screen.getByRole("button", { name: "记下我的立题" });
    expect(button).toBeEnabled();
    await act(async () => {
      fireEvent.click(button);
    });
    expect(onSubmit).toHaveBeenCalledWith({
      terms: [{ term: "可持续性", definition: "只写了一半" }],
      answers: [],
      searchPlan: [],
    });
  });

  it("never disables the save button for incompleteness — only while submitting", async () => {
    const data = makeData();
    let resolveSubmit: () => void = () => {};
    const onSubmit = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          resolveSubmit = resolve;
        }),
    );
    render(<FramingView data={data} onSubmit={onSubmit} />);
    const button = screen.getByRole("button", { name: "记下我的立题" });
    expect(button).toBeEnabled(); // completely empty, still enabled

    fireEvent.click(button);
    expect(await screen.findByRole("button", { name: "记录中…" })).toBeDisabled();

    await act(async () => {
      resolveSubmit();
    });
    expect(screen.getByRole("button", { name: "记下我的立题" })).toBeEnabled();
  });

  it("lets her add/edit/remove a candidate argument and a search-plan item", () => {
    const data = makeData();
    render(<FramingView data={data} />);

    fireEvent.click(screen.getByText("添加一条论点"));
    const argBox = screen.getByLabelText("核心论点") as HTMLTextAreaElement;
    fireEvent.change(argBox, { target: { value: "初步判断：有进展但存量问题未解决。" } });
    expect(argBox.value).toBe("初步判断：有进展但存量问题未解决。");
    fireEvent.click(screen.getByLabelText("删除这条论点"));
    expect(screen.queryByLabelText("核心论点")).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("添加一条检索方向"));
    const searchBox = screen.getByLabelText("检索方向") as HTMLInputElement;
    fireEvent.change(searchBox, { target: { value: "查 NASA 卫星数据" } });
    expect(searchBox.value).toBe("查 NASA 卫星数据");
    fireEvent.click(screen.getByLabelText("删除这条检索方向"));
    expect(screen.queryByLabelText("检索方向")).not.toBeInTheDocument();
  });
});
