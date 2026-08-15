export const COURSE_RENDERER_VERSION = "0.0.0";

export type { BlockRendererProps, BlockRenderer, TextBlock, ImagesBlock, ImageItem } from "./blocks/types";
export { blockRenderers, getBlockRenderer } from "./blocks/registry";
export { NotImplementedRenderer } from "./blocks/NotImplementedRenderer";
export { TextRenderer } from "./blocks/TextRenderer";
export { ImagesRenderer } from "./blocks/ImagesRenderer";
export { LayoutRenderer } from "./layout/LayoutRenderer";
export type { LayoutRendererProps, BlockId } from "./layout/LayoutRenderer";
export { AudioEngineProvider, HtmlAudioEngine, useAudioEngine } from "./narration/audioEngine";
export type { AudioEngine } from "./narration/audioEngine";
export { NarrationController, NarrationPlayer } from "./narration/NarrationPlayer";
export type { NarrationPlayerProps, ActiveNarration } from "./narration/NarrationPlayer";
export { FocusProvider, FocusTarget, useCurrentFocus, isBlockFocused, focusedItemIdFor } from "./focus/FocusManager";
export type { FocusTargetProps } from "./focus/FocusManager";
export { SlicePlayer } from "./slice/SlicePlayer";
export type { SlicePlayerProps, Scheduler } from "./slice/SlicePlayer";
