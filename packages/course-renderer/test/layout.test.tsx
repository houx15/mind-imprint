import { render } from "@testing-library/react";
import { LayoutRenderer } from "../src/layout/LayoutRenderer";
import type { LayoutDefinition } from "@mind-imprint/course-contract";

const renderSlot = (slotId: string, blockIds: string[]) => (
  <span data-slot-content={slotId}>{blockIds.join(",")}</span>
);

describe("LayoutRenderer", () => {
  it("full → one region with data-slot='main'", () => {
    const layout: LayoutDefinition = { preset: "full", slots: [{ id: "main", blockIds: ["a"] }] };
    const { container } = render(<LayoutRenderer layout={layout} renderSlot={renderSlot} />);
    const slots = container.querySelectorAll("[data-slot]");
    expect(slots).toHaveLength(1);
    expect(slots[0]).toHaveAttribute("data-slot", "main");
  });

  it("split-horizontal 2:1 → left/right regions with 2fr 1fr columns", () => {
    const layout: LayoutDefinition = {
      preset: "split-horizontal",
      ratio: "2:1",
      slots: [
        { id: "left", blockIds: ["a"] },
        { id: "right", blockIds: ["b"] },
      ],
    };
    const { container } = render(<LayoutRenderer layout={layout} renderSlot={renderSlot} />);
    const frame = container.querySelector("[data-preset]") as HTMLElement;
    expect(frame.style.gridTemplateColumns).toBe("2fr 1fr");
    const ids = [...container.querySelectorAll("[data-slot]")].map((n) => n.getAttribute("data-slot"));
    expect(ids).toEqual(["left", "right"]);
  });

  it("split-vertical 1:2 → top/bottom rows with 1fr 2fr rows", () => {
    const layout: LayoutDefinition = {
      preset: "split-vertical",
      ratio: "1:2",
      slots: [
        { id: "top", blockIds: ["a"] },
        { id: "bottom", blockIds: ["b"] },
      ],
    };
    const { container } = render(<LayoutRenderer layout={layout} renderSlot={renderSlot} />);
    const frame = container.querySelector("[data-preset]") as HTMLElement;
    expect(frame.style.gridTemplateRows).toBe("1fr 2fr");
    const ids = [...container.querySelectorAll("[data-slot]")].map((n) => n.getAttribute("data-slot"));
    expect(ids).toEqual(["top", "bottom"]);
  });

  it("grid with 3 cells → cell-1..3 each appearing once", () => {
    const layout: LayoutDefinition = {
      preset: "grid",
      slots: [
        { id: "cell-1", blockIds: ["a"] },
        { id: "cell-2", blockIds: ["b"] },
        { id: "cell-3", blockIds: ["c"] },
      ],
    };
    const { container } = render(<LayoutRenderer layout={layout} renderSlot={renderSlot} />);
    const ids = [...container.querySelectorAll("[data-slot]")].map((n) => n.getAttribute("data-slot"));
    expect(ids).toEqual(["cell-1", "cell-2", "cell-3"]);
    const frame = container.querySelector("[data-preset]") as HTMLElement;
    expect(frame.style.gridTemplateColumns).toBe("repeat(2, 1fr)");
  });

  it("passes each slot's blockIds to renderSlot", () => {
    const layout: LayoutDefinition = { preset: "full", slots: [{ id: "main", blockIds: ["a", "b"] }] };
    const { container } = render(<LayoutRenderer layout={layout} renderSlot={renderSlot} />);
    expect(container.querySelector('[data-slot-content="main"]')).toHaveTextContent("a,b");
  });
});
