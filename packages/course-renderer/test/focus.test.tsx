import { render } from "@testing-library/react";
import { FocusProvider, FocusTarget, focusedItemIdFor, isBlockFocused } from "../src/focus/FocusManager";
import type { TargetRef } from "@mind-imprint/course-contract";

function renderTree(focus: TargetRef | null) {
  return render(
    <FocusProvider value={focus}>
      <FocusTarget blockId="a">block a</FocusTarget>
      <FocusTarget blockId="img">img block</FocusTarget>
      <FocusTarget blockId="img" itemId="x">
        item x
      </FocusTarget>
      <FocusTarget blockId="img" itemId="y">
        item y
      </FocusTarget>
    </FocusProvider>,
  );
}

const focused = (c: HTMLElement) => [...c.querySelectorAll('[data-focused="true"]')];

describe("FocusManager", () => {
  it("block-level focus marks the matching block wrapper", () => {
    const { container } = renderTree({ blockId: "a" });
    const marked = focused(container);
    expect(marked).toHaveLength(1);
    expect(marked[0]).toHaveAttribute("data-focus-block", "a");
    expect(marked[0]).toHaveClass("course-focus-ring");
  });

  it("item focus marks only the matching item, not the block", () => {
    const { container } = renderTree({ blockId: "img", itemId: "x" });
    const marked = focused(container);
    expect(marked).toHaveLength(1);
    expect(marked[0]).toHaveAttribute("data-focus-item", "x");
  });

  it("clearFocus (null) removes all focus", () => {
    const { container } = renderTree(null);
    expect(focused(container)).toHaveLength(0);
  });

  it("pure helpers agree with the wrappers", () => {
    expect(isBlockFocused({ blockId: "a" }, "a")).toBe(true);
    expect(isBlockFocused({ blockId: "a", itemId: "x" }, "a")).toBe(false);
    expect(focusedItemIdFor({ blockId: "img", itemId: "x" }, "img")).toBe("x");
    expect(focusedItemIdFor({ blockId: "img" }, "img")).toBeUndefined();
    expect(focusedItemIdFor(null, "img")).toBeUndefined();
  });
});
