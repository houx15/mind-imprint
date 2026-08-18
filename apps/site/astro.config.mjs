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
  // The old single evaluation page folded into /product#evaluation.
  redirects: {
    "/evaluation": "/product",
    "/en/evaluation": "/en/product",
  },
});
