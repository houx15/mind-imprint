import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const webSrc = path.resolve(__dirname, "../web/src");

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: [
      // 房间组件从 apps/web 源码引入，它们内部用 "@/…" 自引用 —— 这个别名必须
      // 解析到 web 的 src，否则一进阅读室就是一片解析失败。
      { find: /^@\//, replacement: webSrc + "/" },
      { find: "@lite", replacement: path.resolve(__dirname, "src") },
    ],
  },
  optimizeDeps: { exclude: ["@mind-imprint/web", "@mind-imprint/contracts"] },
  // 端口可被 e2e 覆盖：这台机器上 8080 常被别的项目占着。
  server: {
    proxy: {
      "/api": {
        target: `http://localhost:${process.env.VITE_E2E_API_PORT ?? "8080"}`,
        changeOrigin: true,
      },
    },
  },
});
