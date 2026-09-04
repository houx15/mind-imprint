import type { Page } from "@playwright/test";

/**
 * 把 spec §4 那道门打开。
 *
 * 自 2026-09-03 起，学生的第一个项目就是做她自己的主页，主页发布之前
 * `POST /api/v1/pbl/projects` 一律 409（`pbl_site.go` 的 `siteGateOpen`）。
 * 所以任何一条「她已经有别的项目」的 walk，都必须先走过这道门——**这就是产品
 * 现在真实的样子**，不是测试的权宜。
 *
 * 这里走接口而不是点界面：门本身有 `homepage-walk.spec.ts` 一整条 walk 在看，
 * 别的 walk 只是需要站在门后面。`page.request` 和页面共用同一份 cookie，所以
 * 它就是这个学生自己在操作。
 */

/**
 * 接口在哪个 host 上。
 *
 * 🚨 本地和线上不是同一个形状，这里必须分开。本地 vite 把 `/api` 代理到
 * :8080，所以相对路径能用；线上前后端是两个 host（mind-lite / mind-api），
 * 相对路径会打到静态站那台 nginx 上——它对 PUT 回 **405**，报出来的样子像是
 * 接口没了，其实是请求根本没到 Go 那边。2026-09-04 线上跑第一次就栽在这。
 */
const API = (process.env.E2E_API_BASE ?? "").replace(/\/+$/, "");
const url = (path: string) => `${API}${path}`;

export async function openSiteGate(page: Page): Promise<void> {
  const put = async (path: string, data: unknown) => {
    const res = await page.request.put(url(path), { data });
    if (!res.ok()) throw new Error(`${path} → ${res.status()} ${await res.text()}`);
  };

  await put("/api/v1/pbl/site/content", {
    headline: "一件还能修的东西，是谁决定它该被扔的？",
    role: "读 IB 的高二学生",
    about: ["我在拆家里所有还能拆的东西，然后写为什么它们修不好。"],
  });
  // 🚨 第三关那一刀（2026-09-03）把 `PUT /pbl/site/layout` 删了，换成
  // `PUT /pbl/site/look`：旧的那条要她**写一段理由**才肯收版式，那是个表单字段。
  // 新的这条收版式 + 一组配色，三个颜色都要是 #RRGGBB，服务端逐个验
  // （`pbl.ValidPalette`）——一个 "warm beige" 存进去，浏览器会把整条 CSS 丢掉，
  // 而坏掉的是她已经发布出去的那一页。
  await put("/api/v1/pbl/site/look", {
    layout: "ledger",
    palette: {
      label: "旧纸",
      why: "她那几个关键词都偏安静，底色压暗一点更像她。",
      paper: "#F6F1E7",
      ink: "#2B2A26",
      accent: "#B4553A",
    },
  });
  const res = await page.request.post(url("/api/v1/pbl/site/publish"));
  if (!res.ok()) throw new Error(`publish → ${res.status()} ${await res.text()}`);
}
