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
export async function openSiteGate(page: Page): Promise<void> {
  const put = async (path: string, data: unknown) => {
    const res = await page.request.put(path, { data });
    if (!res.ok()) throw new Error(`${path} → ${res.status()} ${await res.text()}`);
  };

  await put("/api/v1/pbl/site/content", {
    headline: "一件还能修的东西，是谁决定它该被扔的？",
    role: "读 IB 的高二学生",
    about: ["我在拆家里所有还能拆的东西，然后写为什么它们修不好。"],
  });
  await put("/api/v1/pbl/site/layout", {
    layout: "ledger",
    why: "我做的东西比写的字多，索引式一屏能看到十几条。",
  });
  const res = await page.request.post("/api/v1/pbl/site/publish");
  if (!res.ok()) throw new Error(`publish → ${res.status()} ${await res.text()}`);
}
