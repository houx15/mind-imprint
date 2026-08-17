// @ts-check
import { defineConfig } from "astro/config";

// 思维印记 marketing site — static, bilingual (zh default, en under /en/).
// zh lives at /, /product, /algorithm, /about; en mirrors under /en/*.
export default defineConfig({
  site: "https://mindimprint.example",
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
