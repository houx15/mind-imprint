import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { Annotate } from "./Annotate";
import type { AnnotateState } from "@mind-imprint/contracts";

const blocks = [{ id: "b1", text: "abcXXXXdef" }];
const state: AnnotateState = { material_id: "m1",
  spans: [{ id: "s1", block_ref: "b1", range: { start: 3, end: 7 }, tag: "权威性", note: "这条往上追，原始出处是谁？", author: "ai" }] };

describe("Annotate", () => {
  it("renders a clickable AI-highlighted span and selects it", () => {
    const onSelect = vi.fn();
    render(<Annotate blocks={blocks} state={state} activeSpanId={null} onSelectSpan={onSelect} />);
    fireEvent.click(screen.getByText("XXXX"));
    expect(onSelect).toHaveBeenCalledWith("s1");
  });
  it("reveals the active span's dimension + question", () => {
    render(<Annotate blocks={blocks} state={state} activeSpanId="s1" onSelectSpan={() => {}} />);
    expect(screen.getByText("权威性")).toBeInTheDocument();
    expect(screen.getByText(/原始出处是谁/)).toBeInTheDocument();
  });

  it("tags each block with data-block-id", () => {
    const { container } = render(<Annotate blocks={blocks} state={state} activeSpanId={null} onSelectSpan={() => {}} />);
    expect(container.querySelector('[data-block-id="b1"]')).toBeInTheDocument();
  });

  it("with selectMode/onCreateSpan absent, renders no hint bar and behaves as today", () => {
    const onSelect = vi.fn();
    render(<Annotate blocks={blocks} state={state} activeSpanId={null} onSelectSpan={onSelect} />);
    expect(screen.queryByText("取消")).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("XXXX"));
    expect(onSelect).toHaveBeenCalledWith("s1");
  });

  it("select-mode: renders an instructional hint bar naming the dimension, plus a cancel control", () => {
    const onCancel = vi.fn();
    render(
      <Annotate
        blocks={blocks}
        state={state}
        activeSpanId={null}
        onSelectSpan={() => {}}
        selectMode={{ dimension: "权威性", onCancel }}
      />
    );
    expect(screen.getByText(/权威性/)).toBeInTheDocument();
    const cancelButton = screen.getByText("取消");
    fireEvent.click(cancelButton);
    expect(onCancel).toHaveBeenCalled();
  });

  it("select-mode: does not render any level name, badge, or celebratory copy", () => {
    render(
      <Annotate
        blocks={blocks}
        state={state}
        activeSpanId={null}
        onSelectSpan={() => {}}
        selectMode={{ dimension: "权威性", onCancel: () => {} }}
      />
    );
    expect(screen.queryByText(/L1|L2|L3|解锁|恭喜|太棒了/)).not.toBeInTheDocument();
  });

  it("select-mode: a mouseup with a real text selection calls onCreateSpan with the created span", () => {
    const onCreateSpan = vi.fn();
    const { container } = render(
      <Annotate
        blocks={blocks}
        state={state}
        activeSpanId={null}
        onSelectSpan={() => {}}
        selectMode={{ dimension: "权威性", onCancel: () => {} }}
        onCreateSpan={onCreateSpan}
      />
    );
    const blockEl = container.querySelector('[data-block-id="b1"]')!;
    const textNode = blockEl.firstChild!.firstChild as Text; // first run "abc"
    const range = document.createRange();
    range.setStart(textNode, 0);
    range.setEnd(textNode, 3);
    const sel = window.getSelection()!;
    sel.removeAllRanges();
    sel.addRange(range);

    fireEvent.mouseUp(blockEl.parentElement!);

    expect(onCreateSpan).toHaveBeenCalledWith({ blockId: "b1", start: 0, end: 3, text: "abc" });
  });

  it("without select-mode, no mouseup handler is attached and onCreateSpan is never invoked", () => {
    const onCreateSpan = vi.fn();
    const { container } = render(
      <Annotate blocks={blocks} state={state} activeSpanId={null} onSelectSpan={() => {}} onCreateSpan={onCreateSpan} />
    );
    const blockEl = container.querySelector('[data-block-id="b1"]')!;
    const textNode = blockEl.firstChild!.firstChild as Text;
    const range = document.createRange();
    range.setStart(textNode, 0);
    range.setEnd(textNode, 3);
    const sel = window.getSelection()!;
    sel.removeAllRanges();
    sel.addRange(range);

    fireEvent.mouseUp(blockEl.parentElement!);

    expect(onCreateSpan).not.toHaveBeenCalled();
  });
});
