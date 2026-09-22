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

const HARNESS_PORT = Number(process.env.E2E_HARNESS_PORT ?? 5234);

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
  // 🚨 端口可以用环境变量换掉，而且**必须**能换。
  //
  // 2026-09-22：两个会话各自在自己的 worktree 里开了这台看图台，端口都写死
  // 5234、都 strictPort。第二台起不来（「Port 5234 is already in use」），
  // 而 playwright 照样连上了**第一台** —— 于是我跑的是另一个 worktree 的代码，
  // 报回来三条红，看起来像这一轮改坏了三块界面。日志里那行 EADDRINUSE 是
  // 唯一的线索，而它在后台文件里没人看。
  //
  // 换个端口就能并存：
  //   E2E_HARNESS_PORT=5235 npx vite --config e2e/harness/vite.config.ts
  //   E2E_HARNESS_PORT=5235 npx playwright test --config e2e/harness/playwright.config.ts
  server: { port: HARNESS_PORT, strictPort: true },
});
