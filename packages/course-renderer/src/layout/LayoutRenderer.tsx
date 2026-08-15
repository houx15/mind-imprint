import type { CSSProperties, ReactNode } from "react";
import type { LayoutDefinition, SplitRatio } from "@mind-imprint/course-contract";

/** Block ids are plain lower-case-hyphenated strings in the contract (no branded type). */
export type BlockId = string;

export interface LayoutRendererProps {
  layout: LayoutDefinition;
  /** Mounts the block renderers for one slot. Supplied by SlicePlayer. */
  renderSlot: (slotId: string, blockIds: BlockId[]) => ReactNode;
}

/** `2:1` → `2fr 1fr` etc. Split ratios only ever have two tracks. */
function ratioTracks(ratio: SplitRatio | undefined): string {
  switch (ratio) {
    case "2:1":
      return "2fr 1fr";
    case "1:2":
      return "1fr 2fr";
    case "1:1":
    default:
      return "1fr 1fr";
  }
}

/**
 * §10 / §17.5 — owns only the grid/flex frame that positions slots; the block
 * content is supplied by `renderSlot`. Maps the four presets to stable desktop
 * CSS grid:
 * - `full` → a single `main` region;
 * - `split-horizontal` → two columns from the ratio (`left` | `right`);
 * - `split-vertical` → two rows from the ratio (`top` | `bottom`);
 * - `grid` → a 2-column grid of `cell-1..N`.
 *
 * Every slot in the layout is rendered exactly once, in authored order. Hidden
 * blocks inside a slot keep their box (they render but `hidden`), so revealing a
 * block never reflows its siblings.
 */
export function LayoutRenderer({ layout, renderSlot }: LayoutRendererProps) {
  const style: CSSProperties = { display: "grid" };
  switch (layout.preset) {
    case "full":
      style.gridTemplateColumns = "1fr";
      break;
    case "split-horizontal":
      style.gridTemplateColumns = ratioTracks(layout.ratio);
      break;
    case "split-vertical":
      style.gridTemplateRows = ratioTracks(layout.ratio);
      break;
    case "grid":
      style.gridTemplateColumns = "repeat(2, 1fr)";
      break;
  }

  return (
    <div className={`course-layout course-layout--${layout.preset}`} data-preset={layout.preset} style={style}>
      {layout.slots.map((slot) => (
        <div key={slot.id} data-slot={slot.id} className={`course-layout__slot course-layout__slot--${slot.id}`}>
          {renderSlot(slot.id, slot.blockIds)}
        </div>
      ))}
    </div>
  );
}
