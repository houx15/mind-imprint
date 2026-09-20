import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

/**
 * 只服务 `e2e/harness` 那个看图台。
 *
 * 和产品的 `vite.config.ts` 分开，免得一个一次性的调试页把真正的构建配置
 * 弄脏；别名照抄那一份（`@/` 必须解析到 apps/web 的 src，否则 index.css
 * 第一行的 `@import "@/index.css"` 就断了）。
 */
const __dirname = fileURLToPath(new URL(".", import.meta.url));
const liteRoot = path.resolve(__dirname, "../..");
const webSrc = path.resolve(liteRoot, "../web/src");

export default defineConfig({
  root: __dirname,
  plugins: [react()],
  css: { postcss: liteRoot },
  resolve: {
    alias: [
      ...["contracts", "course-contract", "course-runtime", "course-renderer"].map((name) => ({
        find: new RegExp(`^@mind-imprint/${name}$`),
        replacement: path.resolve(liteRoot, `../../packages/${name}/src/index.ts`),
      })),
      { find: /^@\//, replacement: webSrc + "/" },
      { find: "@lite", replacement: path.resolve(liteRoot, "src") },
    ],
  },
  server: { port: 5234, strictPort: true },
});
