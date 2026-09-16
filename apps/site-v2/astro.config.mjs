import { defineConfig } from "astro/config";
export default defineConfig({
  site: process.env.SITE_URL || "https://mind.uni-robot.cn",
  output: "static",
  devToolbar: { enabled: false },
  trailingSlash: "always",
});
