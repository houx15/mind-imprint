import { ACCENT_PRESETS, type AccentPreset } from "../accent";

/** Lite student theme. Stable IDs keep saved account preferences compatible.
 * Literal ramps: 50–400 mix 9/15/28/45/75% base with white;
 * 600–800 retain 88/75/62% base against black. */
export const LITE_ACCENT_PRESETS: readonly AccentPreset[] = [
  { id: "teal", name: ACCENT_PRESETS.find(p => p.id === "teal")!.name, scale: { 50: "#e9f3f5", 100: "#daecee", 200: "#badbdf", 300: "#90c5cb", 400: "#469fa9", 500: "#087f8c", 600: "#07707b", 700: "#065f69", 800: "#054f57" } },
  { id: "indigo", name: ACCENT_PRESETS.find(p => p.id === "indigo")!.name, scale: { 50: "#eef1fb", 100: "#e2e7f8", 200: "#c9d2f1", 300: "#a9b7e9", 400: "#6f88da", 500: "#3f60ce", 600: "#3754b5", 700: "#2f489a", 800: "#273c80" } },
  { id: "violet", name: ACCENT_PRESETS.find(p => p.id === "violet")!.name, scale: { 50: "#f3eff9", 100: "#ebe5f5", 200: "#d9cfed", 300: "#c3b1e1", 400: "#9a7dce", 500: "#7952bd", 600: "#6a48a6", 700: "#5b3e8e", 800: "#4b3375" } },
  { id: "rose", name: ACCENT_PRESETS.find(p => p.id === "rose")!.name, scale: { 50: "#f9eef3", 100: "#f4e2eb", 200: "#ebc9da", 300: "#e0a8c3", 400: "#ca6e9b", 500: "#b93e7a", 600: "#a3376b", 700: "#8b2e5c", 800: "#73264c" } },
  { id: "vermilion", name: ACCENT_PRESETS.find(p => p.id === "vermilion")!.name, scale: { 50: "#fbefef", 100: "#f8e4e4", 200: "#f1cdcc", 300: "#e9aead", 400: "#da7876", 500: "#ce4b48", 600: "#b5423f", 700: "#9a3836", 800: "#802e2d" } },
  { id: "bamboo", name: ACCENT_PRESETS.find(p => p.id === "bamboo")!.name, scale: { 50: "#ecf4f0", 100: "#e0ede6", 200: "#c4ddd1", 300: "#a0c8b5", 400: "#62a484", 500: "#2d855b", 600: "#287550", 700: "#226444", 800: "#1c5238" } },
  { id: "clay", name: ACCENT_PRESETS.find(p => p.id === "clay")!.name, scale: { 50: "#f9f0ee", 100: "#f5e5e2", 200: "#eccfca", 300: "#e0b2a9", 400: "#cc7e70", 500: "#bb5340", 600: "#a54938", 700: "#8c3e30", 800: "#743328" } },
  { id: "tangerine", name: ACCENT_PRESETS.find(p => p.id === "tangerine")!.name, scale: { 50: "#f7f1ea", 100: "#f2e8db", 200: "#e7d4bd", 300: "#d8ba94", 400: "#be8c4d", 500: "#a96512", 600: "#955910", 700: "#7f4c0e", 800: "#693f0b" } },
];
