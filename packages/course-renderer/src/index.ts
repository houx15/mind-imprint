export const COURSE_RENDERER_VERSION = "0.0.0";

export type { BlockRendererProps, BlockRenderer, TextBlock, ImagesBlock, ImageItem } from "./blocks/types";
export { blockRenderers, getBlockRenderer } from "./blocks/registry";
export { NotImplementedRenderer } from "./blocks/NotImplementedRenderer";
export { TextRenderer } from "./blocks/TextRenderer";
export { ImagesRenderer } from "./blocks/ImagesRenderer";
export { LayoutRenderer } from "./layout/LayoutRenderer";
export type { LayoutRendererProps, BlockId } from "./layout/LayoutRenderer";
