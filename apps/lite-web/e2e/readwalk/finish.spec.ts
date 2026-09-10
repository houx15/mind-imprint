import { test, expect } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

/**
 * 「完成这篇」这条路本身通不通。
 *
 * 🚨 它和模拟学生那条 walk 问的**不是同一件事**。那一条问的是「她会不会被带到
 * 这儿」；这一条问的是「到了这儿，按下去有没有用」。走查没走到终点的时候，
 * 这两种原因长得一模一样，而修法完全不同 —— 所以先把这一条钉死。
 *
 * 跑法：
 *   cd apps/lite-web && npx playwright test --config e2e/readwalk/playwright.config.ts finish
 */

const API = process.env.E2E_API_BASE ?? "https://mind-api.uni-robot.cn";
const BASE = process.env.E2E_BASE_URL ?? "https://mind-lite.uni-robot.cn";
const JOIN = process.env.E2E_JOIN_CODE ?? "G624-UXFE";
const OUT = process.env.READWALK_OUT ?? "e2e/.readwalk";
const SLUG = process.env.READWALK_SLUG ?? "aid-groups-israel-hamas-war";

test("完成这篇 → 阅读报告", async ({ browser }) => {
  test.setTimeout(5 * 60_000);
  fs.mkdirSync(OUT, { recursive: true });

  const tag = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`;
  const email = `fin-${tag}@demo.mindimprint.local`;
  const password = `fin-${tag}-pass`;
  const ctx = await browser.newContext({ baseURL: BASE, viewport: { width: 1440, height: 1000 } });

  const up = await ctx.request.post(`${API}/api/v1/auth/signup`, {
    data: { email, password, display_name: "收尾走查", join_code: JOIN },
  });
  expect(up.ok(), `signup ${up.status()}`).toBeTruthy();
  await ctx.request.post(`${API}/api/v1/auth/signin`, { data: { email, password } });
  const started = await ctx.request.post(`${API}/api/v1/library/${SLUG}/levels/3`);
  expect(started.ok(), `start ${started.status()}`).toBeTruthy();
  const { id } = await started.json();

  const page = await ctx.newPage();
  await page.goto(`/readings/${id}`);
  await page.locator(".mk-reading-room__article").first().waitFor({ timeout: 30_000 });

  // 完成这篇 从第一秒起就在顶栏上 —— 她任何时候都够得着，不必先走完读法。
  const finish = page.getByRole("button", { name: "完成这篇" });
  await expect(finish, "顶栏上没有「完成这篇」").toBeVisible();
  await finish.click();

  const dialog = page.getByRole("dialog", { name: "完成这篇" });
  await expect(dialog, "确认框没出来").toBeVisible();
  await page.screenshot({ path: path.join(OUT, "fin-1-confirm.png"), fullPage: true });

  await dialog.getByRole("button", { name: "完成，看报告" }).click();

  // 完成之后房间该换成报告。
  //
  // 🚨 按**结构**认，不要按文案里的字认。这一条第一版写的是
  // `getByText(/阅读报告/)` —— 它当场就假绿了：那四个字**就在确认框自己那句话
  // 里**（「你走过的每一步会变成一份阅读报告」）。于是断言在对话框还开着、
  // 按钮还在转的时候就通过了，截图拍下来的是一个转圈的确认框，而我差点把它
  // 当成「报告出来了」。`.mk-rp-stat` 只在报告上有。
  await expect(page.locator(".mk-rp-stat").first()).toBeVisible({ timeout: 90_000 });
  await expect(dialog).toBeHidden();
  await page.screenshot({ path: path.join(OUT, "fin-2-report.png"), fullPage: true });
  console.log("✅ 完成这篇 → 报告，这条路本身是通的");

  // 再打开一次：完成过的阅读不该再回到房间（终态就是终态）。
  await page.goto(`/readings/${id}`);
  await expect(page.locator(".mk-rp-stat").first()).toBeVisible({ timeout: 60_000 });
  await expect(page.locator(".mk-reading-room__article")).toHaveCount(0);
  console.log("✅ 完成过的阅读重新打开，进的是报告不是房间");

  await ctx.close();
});
