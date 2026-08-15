import type { BlockType } from "@mind-imprint/course-contract";
import type { BlockRenderer } from "./types";
import { NotImplementedRenderer } from "./NotImplementedRenderer";
import { TextRenderer } from "./TextRenderer";
import { ImagesRenderer } from "./ImagesRenderer";
import { FillBlankRenderer } from "./assessment/FillBlankRenderer";

/**
 * §17.6 — the single source of truth mapping a `BlockType` to its renderer.
 * Adding a block type later = registering a renderer here; nothing else in the
 * renderer needs to change (registry/layout/SlicePlayer stay untouched).
 */
export const blockRenderers: Record<BlockType, BlockRenderer<any>> = {
  text: TextRenderer,
  images: ImagesRenderer,
  pdf: NotImplementedRenderer,
  video: NotImplementedRenderer,
  interactiveHtml: NotImplementedRenderer,
  fillBlank: FillBlankRenderer,
  singleChoice: NotImplementedRenderer,
};

/**
 * Looks up the renderer for a block type. Throws for an unknown type so a
 * malformed course fails BEFORE playback rather than rendering a blank slot
 * (§17.6).
 */
export function getBlockRenderer(type: string): BlockRenderer<any> {
  const renderer = blockRenderers[type as BlockType];
  if (!renderer) throw new Error(`getBlockRenderer: no renderer registered for block type '${type}'`);
  return renderer;
}
