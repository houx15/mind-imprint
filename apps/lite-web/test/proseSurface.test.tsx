import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ProseSurface, PROSE_TYPOGRAPHY } from "../src/writings/ProseSurface";

describe("ProseSurface", () => {
  // The highlight layer sits BEHIND the textarea and must be typeset
  // identically. If the two ever differ by a pixel the highlight lands on the
  // wrong line, so they read from one frozen object rather than two style props.
  it("typesets the textarea and the highlight layer from the same object", () => {
    const { container } = render(
      <ProseSurface value={"第一段。\n\n第二段。"} onChange={() => {}} highlight="第二段。" />,
    );
    const ta = container.querySelector("textarea")!;
    const layer = container.querySelector("[data-prose-layer]") as HTMLElement;
    for (const k of ["fontSize", "lineHeight", "letterSpacing", "padding"] as const) {
      expect(layer.style[k]).toBe(ta.style[k]);
      expect(layer.style[k]).toBe(String(PROSE_TYPOGRAPHY[k]));
    }
  });

  it("marks the highlighted substring and nothing else", () => {
    render(<ProseSurface value={"第一段。第二段。"} onChange={() => {}} highlight="第二段。" />);
    expect(screen.getByText("第二段。", { selector: "mark" })).toBeTruthy();
  });

  it("renders no mark when there is no highlight", () => {
    const { container } = render(<ProseSurface value="第一段。" onChange={() => {}} highlight={null} />);
    expect(container.querySelector("mark")).toBeNull();
  });
});
