// @ts-check
import { defineConfig } from "astro/config";

// 思维印记 marketing site — static, bilingual (zh default, en under /en/).
// zh lives at /, /product, /algorithm, /about; en mirrors under /en/*.
export default defineConfig({
  // Public origin, used for canonical + hreflang alternates. Baked at build
  // time (see apps/site/Dockerfile); falls back to the production host so a
  // plain `pnpm build` still emits correct absolute URLs.
  site: process.env.SITE_URL ?? "https://mind.uni-robot.cn",
  i18n: {
    defaultLocale: "zh",
    locales: ["zh", "en"],
    routing: {
      prefixDefaultLocale: false,
    },
  },
  // Old routes. /evaluation predates the product split; the tab anchors on
  // /product became real pages, so the deep links move with them.
  redirects: {
    "/evaluation": "/product/evaluation",
    "/en/evaluation": "/en/product/evaluation",
  },
});
