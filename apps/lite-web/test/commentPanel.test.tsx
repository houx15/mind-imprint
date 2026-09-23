import { render, screen, cleanup, fireEvent } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CommentPanel } from "@lite/writings/CommentPanel";
import type { Comment } from "@lite/api/writingRoom";

/**
 * CommentPanel — pins the trace contract: clicking a point calls back with
 * ITS quote (a literal sentence from her own text), the summary renders
 * before the points, and every point's clickable control carries
 * `data-comment-point` (the selector the later e2e walk relies on — no other
 * task creates it).
 */

afterEach(cleanup);

const COMMENT: Comment = {
  id: "c1",
  scope: "draft",
  snippetId: null,
  summary: "整体清楚，但让步段还没有真正撞上反例。",
  points: [
    { text: "这个例子是过密，不是数量多。", quote: "两排树掘得密密麻麻" },
    { text: "这里的让步只是复述对方观点，没有正面回应。", quote: "有人会说这样成本更低" },
  ],
  sourceText: "",
  layer_verdicts: {},
  createdAt: "2026-08-28T00:00:00Z",
};

describe("CommentPanel", () => {
  it("traces a point back to the sentence it is about", () => {
    const onTrace = vi.fn();
    render(<CommentPanel comment={COMMENT} onTrace={onTrace} />);

    fireEvent.click(screen.getByRole("button", { name: /这个例子是过密/ }));

    expect(onTrace).toHaveBeenCalledWith("两排树掘得密密麻麻");
    expect(onTrace).toHaveBeenCalledOnce();
  });

  it("renders the summary before the points", () => {
    render(<CommentPanel comment={COMMENT} onTrace={() => {}} />);

    const [firstPoint] = COMMENT.points;
    const summaryEl = screen.getByText(COMMENT.summary);
    const firstPointEl = screen.getByText(firstPoint!.text);

    // DOCUMENT_POSITION_FOLLOWING means summaryEl comes before firstPointEl.
    // eslint-disable-next-line no-bitwise
    expect(
      summaryEl.compareDocumentPosition(firstPointEl) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("shows the quote alongside each point's text, not hidden behind a click", () => {
    render(<CommentPanel comment={COMMENT} onTrace={() => {}} />);

    for (const point of COMMENT.points) {
      expect(screen.getByText(point.text)).toBeTruthy();
      expect(screen.getByText(point.quote)).toBeTruthy();
    }
  });

  it("marks every point's clickable control with data-comment-point, for the e2e walk", () => {
    render(<CommentPanel comment={COMMENT} onTrace={() => {}} />);

    const marked = document.querySelectorAll("[data-comment-point]");
    expect(marked.length).toBe(COMMENT.points.length);
    marked.forEach((el) => expect(el.tagName).toBe("BUTTON"));
  });

  it("renders nothing for points when there are none, but still shows the summary", () => {
    const bare: Comment = { ...COMMENT, points: [] };
    render(<CommentPanel comment={bare} onTrace={() => {}} />);

    expect(screen.getByText(bare.summary)).toBeTruthy();
    expect(document.querySelectorAll("[data-comment-point]").length).toBe(0);
  });

  /**
   * 🚨 只用服务端**真的会发出来**的形状。
   *
   * 上一版这条写的是 {"1":"pass","2":"polish","3":"revise","4":"pass"} ——
   * 两层同时非 pass，而 validateCommentPoints 只留最上面那一层
   * （dropLowerLayer），所以那一份 payload 线上一次都不会出现。测一个产不出
   * 的形状，等于没测。
   *
   * 真实的两种形状只有：
   *   - 有 issue 的那一轮：命中的那一层给等级，其余三层是「本轮未看」。
   *   - 一条 issue 都没有：四层全「已通过」。
   */
  it("shows all four layer chips: the layer that was looked at, and the three that were not", () => {
    const withLayers: Comment = {
      ...COMMENT,
      layer_verdicts: { "1": "polish", "2": "unchecked", "3": "unchecked", "4": "unchecked" },
    };
    render(<CommentPanel comment={withLayers} onTrace={() => {}} />);

    expect(screen.getByText("立意·可优化")).toBeTruthy();
    expect(screen.getByText("材料·本轮未看")).toBeTruthy();
    expect(screen.getByText("结构·本轮未看")).toBeTruthy();
    expect(screen.getByText("字句·本轮未看")).toBeTruthy();
  });

  /**
   * 🚨 「本轮未看」不能长得像「已通过」，也不能长得像报错。
   *
   * 这一格的意思是「这一轮没看这一层」—— 服务端把下面那几层的 issue 压下去
   * 了（dropLowerLayer），所以它既不是表扬也不是错误。这里钉的是两个人眼一定
   * 会读错、而代码里看不出来的不变量：它没有沿用「已通过」的强调色，
   * 也没有沿用「需修改」的 danger 色。
   */
  it("renders 本轮未看 as neither praise nor error", () => {
    const withLayers: Comment = {
      ...COMMENT,
      layer_verdicts: { "1": "pass", "2": "unchecked", "3": "unchecked", "4": "unchecked" },
    };
    render(<CommentPanel comment={withLayers} onTrace={() => {}} />);

    const pass = screen.getByText("立意·已通过") as HTMLElement;
    const unchecked = screen.getByText("材料·本轮未看") as HTMLElement;

    expect(unchecked.style.background).not.toBe(pass.style.background);
    expect(unchecked.style.color).not.toBe(pass.style.color);
    expect(unchecked.style.border).not.toContain("danger");
    // 虚线：一眼就和有结论的那三格分开。
    expect(unchecked.style.border).toContain("dashed");
    expect(pass.style.border).toContain("solid");
  });

  it("shows four 已通过 chips only when nothing at all was flagged", () => {
    const clean: Comment = {
      ...COMMENT,
      layer_verdicts: { "1": "pass", "2": "pass", "3": "pass", "4": "pass" },
    };
    render(<CommentPanel comment={clean} onTrace={() => {}} />);

    expect(screen.getAllByText(/·已通过$/).length).toBe(4);
  });

  it("renders no layer chips when layer_verdicts is empty (old comment row)", () => {
    render(<CommentPanel comment={COMMENT} onTrace={() => {}} />);

    expect(screen.queryByText(/立意·/)).toBeNull();
  });
});
