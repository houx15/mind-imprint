import { test, expect } from "@playwright/test";

/**
 * 「这一篇按什么文体在教」—— 在真浏览器里换一次。
 *
 * 产品负责人 2026-09-23：「for writing, maybe we need to let the students
 * select/talk with ai about what genre they are going to write.」
 *
 * 跑法：
 *   E2E_HARNESS_PORT=5249 npx vite --config e2e/harness/vite.config.ts
 *   E2E_HARNESS_PORT=5249 npx playwright test --config e2e/harness/playwright.config.ts genre-harness
 */

const OUT = "e2e/harness/.shots";

test("推断和她自己定的分得开，换完之后整屋跟着变", async ({ page }) => {
  await page.goto("/genre.html");

  // 🚨 一开始是**推断**的：说「印记按议论文在教这一篇」，不说「你定的是」。
  // 把推断说成是她的选择，是替她做主之后再赖给她。
  await expect(page.locator("[data-genre-line]")).toHaveText("印记按议论文在教这一篇");
  // 房间那一层已经拿到了同一个答案（判定只有一个点）。
  await expect(page.locator("[data-room-genre]")).toHaveText("argument");
  await page.screenshot({ path: `${OUT}/genre-00-inferred.png`, fullPage: true });

  await page.getByRole("button", { name: "换一种" }).click();

  // 三种都在，而且每一种后面跟的是「什么时候选它」，不是定义。
  for (const [label, blurb] of [
    ["议论文", "要说清一个看法"],
    ["记叙文", "写一件真实发生过的事"],
    ["书信", "写给一个具体的人"],
  ]) {
    const card = page.getByRole("button", { name: new RegExp(`^${label}`) });
    await expect(card).toBeVisible();
    await expect(card).toContainText(blurb);
  }
  // 🚨 说清楚换文体不动她的东西，否则她不敢点。
  await expect(page.getByText("图上已有的内容一条都不会动")).toBeVisible();
  await page.screenshot({ path: `${OUT}/genre-01-open.png`, fullPage: true });

  await page.getByRole("button", { name: /^书信/ }).click();

  // 换完之后：措辞变成「你定的是」，房间那一层也拿到了新的。
  await expect(page.locator("[data-genre-line]")).toHaveText("你定的是书信");
  await expect(page.locator("[data-room-genre]")).toHaveText("letter");
  await page.screenshot({ path: `${OUT}/genre-02-chosen.png`, fullPage: true });
});
