// @ts-check
import { defineConfig } from "astro/config";

// 思维印记 marketing site — static, bilingual (zh default, en under /en/).
// zh lives at /, /evaluation, /about; en mirrors under /en/*.
export default defineConfig({
  site: "https://mindimprint.example",
  i18n: {
    defaultLocale: "zh",
    locales: ["zh", "en"],
    routing: {
      prefixDefaultLocale: false,
    },
  },
});
