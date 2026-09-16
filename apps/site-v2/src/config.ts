export type Lang = "zh" | "en";
export type Copy = readonly [string, string];
export const t = (lang: Lang, copy: Copy) => copy[lang === "en" ? 1 : 0];
export const path = (lang: Lang, slug = "") =>
  `${lang === "zh" ? "/zh" : ""}/${slug ? `${slug.replace(/^\/+|\/+$/g, "")}/` : ""}`;
export const proUrl = (
  import.meta.env.PUBLIC_PRO_URL || "https://mind-web.uni-robot.cn"
).replace(/\/$/, "");
export const liteUrl = (
  import.meta.env.PUBLIC_LITE_URL || "https://mind-lite.uni-robot.cn"
).replace(/\/$/, "");
export const demoUrl = `${proUrl}/?trial=1`;
const mediaBase = (
  import.meta.env.PUBLIC_ASSET_BASE_URL || "https://mind-assets.uni-robot.cn"
).replace(/\/$/, "");
export const asset = (key: string) => `${mediaBase}/${key}`;
// New assets use local preview copies until their versioned keys are uploaded.
const v2Base = (import.meta.env.PUBLIC_V2_ASSET_BASE_URL || "/media").replace(
  /\/$/,
  "",
);
export const v2Asset = (key: string) => `${v2Base}/v2/${key}`;
