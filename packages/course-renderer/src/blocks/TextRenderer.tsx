import type { BlockRenderer } from "./types";

// Placeholder — real implementation lands in Task 3.
export const TextRenderer: BlockRenderer = ({ block, visible }) => (
  <div data-block-id={block.id} data-block-type="text" hidden={!visible} />
);
