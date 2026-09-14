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
});
