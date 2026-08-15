import type { BlockRenderer } from "./types";

// Placeholder — real implementation lands in Task 4.
export const ImagesRenderer: BlockRenderer = ({ block, visible }) => (
  <div data-block-id={block.id} data-block-type="images" hidden={!visible} />
);
