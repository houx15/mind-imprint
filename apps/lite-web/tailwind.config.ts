import type { Config } from "tailwindcss";
import base from "../web/tailwind.config";

export default {
  ...base,
  // 房间组件的 class 写在 apps/web 里。漏掉这一条，阅读室会渲染成没有样式的裸 DOM。
  content: ["./index.html", "./src/**/*.{ts,tsx}", "../web/src/**/*.{ts,tsx}"],
} satisfies Config;
