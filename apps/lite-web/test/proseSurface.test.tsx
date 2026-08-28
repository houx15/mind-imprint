import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ProseSurface, PROSE_TYPOGRAPHY } from "../src/writings/ProseSurface";

describe("ProseSurface", () => {
  // The highlight layer sits BEHIND the textarea and must be typeset
  // identically. If the two ever differ by a pixel the highlight lands on the
  // wrong line, so they read from one frozen object rather than two style props.
  //
  // scrollbarGutter is in the list for a reason that is invisible on macOS:
  // the textarea scrolls internally past 60vh and the layer never does, so on
  // Windows/Linux Chrome a classic scrollbar eats ~15px of the textarea's
  // content box only — different wrap points, glyphs drifting away from the
  // caret. Reserving the gutter on BOTH is what keeps their widths equal, so
  // it belongs to the same frozen object and to the same guard.
  it("typesets the textarea and the highlight layer from the same object", () => {
    const { container } = render(
      <ProseSurface value={"第一段。\n\n第二段。"} onChange={() => {}} highlight="第二段。" />,
    );
    const ta = container.querySelector("textarea")!;
    const layer = container.querySelector("[data-prose-layer]") as HTMLElement;
    for (const k of Object.keys(PROSE_TYPOGRAPHY) as (keyof typeof PROSE_TYPOGRAPHY)[]) {
      const want = String(PROSE_TYPOGRAPHY[k]);
      expect(String(layer.style[k])).toBe(want);
      expect(String(ta.style[k])).toBe(want);
    }
    // Pinned by name so a future edit cannot quietly drop the gutter and leave
    // the loop above still passing over whatever keys remain.
    expect(Object.keys(PROSE_TYPOGRAPHY)).toContain("scrollbarGutter");
    expect(PROSE_TYPOGRAPHY.scrollbarGutter).toBe("stable");
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
