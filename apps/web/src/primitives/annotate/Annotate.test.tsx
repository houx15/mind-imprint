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
});
