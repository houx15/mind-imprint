import { expect, test } from "@playwright/test";

/**
 * 主页 walk — spec §4 那道门，和 S5 她真正做出来的那一页。
 *
 * 这个文件存在是为了**被看**。2026-08-30 的教训是 344 条绿测试压在一张全白的
 * 导出图上，所以这里真正的产出是 `e2e/.shots/` 里那几张图，不是断言。
 *
 * 一次模型调用都没有：写字、挑版式、发布，全是纯数据路径。
 */
test("主页: 门 → 挑版式 → 写内容 → 发布 → 访客看到的那一页 → 门开了", async ({
  page,
  browser,
}) => {
  /* 1 · 门。她还没有主页，所以这里没有自由输入框。 */
  await page.goto("/projects");
  await expect(page.getByRole("heading", { name: "先做你自己的主页。" })).toBeVisible();
  await expect(page.getByPlaceholder("比如：", { exact: false })).toHaveCount(0);
  await page.screenshot({ path: "e2e/.shots/home-1-gate.png", fullPage: true });

  /* 2 · 进主页项目。三个版式，每一个都真画出来。 */
  await page.getByRole("button", { name: "开始做我的主页" }).click();
  await expect(page.getByRole("heading", { name: "我自己的主页" })).toBeVisible();
  await expect(page.getByText("三个版式是三个不一样的页面", { exact: false })).toBeVisible();
  // 🚨 这一张是这次改动的重点之一：原型在这一步给的是三条灰色骨架，她要为一个
  // 自己没见过的东西写理由。现在这三张卡片里是三个真的页面。
  await page.screenshot({ path: "e2e/.shots/home-2-three-real-layouts.png", fullPage: true });

  /* 3 · 挑一个。没写理由，确认按钮不生效——服务端那一道也拦着。 */
  await page.getByText("索引式", { exact: true }).click();
  await expect(page.getByText("写完理由才能确认。")).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/home-3-needs-a-reason.png", fullPage: true });

  await page
    .getByPlaceholder("比如：我做的东西比写的字多", { exact: false })
    .fill("我做的东西比写的字多，索引式一屏能看到十几条；另外两版一屏只放得下一件。");
  await page.getByRole("button", { name: "确认" }).click();

  /* 4 · 写内容。左边写，右边就是别人会看到的那一页。 */
  await expect(page.getByText("这是别人会看到的样子")).toBeVisible();
  await page
    .getByPlaceholder("一件还能修的东西，是谁决定它该被扔的？")
    .fill("一件还能修的东西，是谁决定它该被扔的？");
  await page.getByPlaceholder("读 IB 的高二学生 · 在拆东西").fill("读 IB 的高二学生 · 在拆东西");

  // 「开场一段」「关于」两个框没有 placeholder，按标签定位它们后面的那个框。
  const fieldAfter = (label: string) =>
    page.locator(`xpath=//label[normalize-space(text())="${label}"]/following::textarea[1]`);
  await fieldAfter("开场一段").fill(
    "我在拆家里所有还能拆的东西，然后写为什么它们修不好。这一页放我做过的、写过的，和我最近在想的问题。",
  );
  await fieldAfter("关于").fill(
    "我读高二，在 IB。三年前家里一盏灯坏了，售后说只能整只换，我第一次意识到「修不好」有时候是被设计成这样的。",
  );
  await fieldAfter("现在").fill("在把这一页做出来。");

  await page.screenshot({ path: "e2e/.shots/home-4-words-and-live-preview.png", fullPage: true });
  await page.getByRole("button", { name: "保存" }).click();
  await expect(page.getByText("已保存")).toBeVisible();

  /* 5 · 发布。 */
  await page.getByRole("button", { name: "发布" }).first().click();
  await expect(page.getByRole("heading", { name: "已经在线上了" })).toBeVisible({
    timeout: 30_000,
  });
  await page.screenshot({ path: "e2e/.shots/home-5-published.png", fullPage: true });

  const url = await page.locator('input[readonly]').inputValue();
  expect(url).toContain("/p/");

  /* 6 · 访客那一面：完全没有 session。
     🚨 这一张是整次改动最要紧的证据。原型走到这里得到的是林知遥的页面——页头
     「林知遥 · 初二学生」压着她自己写的那一行，关于是别人的自传。 */
  const visitor = await browser.newContext({ storageState: { cookies: [], origins: [] } });
  const guest = await visitor.newPage();
  await guest.goto(url);
  await expect(guest.getByText("一件还能修的东西，是谁决定它该被扔的？")).toBeVisible();
  // 原型那些人的痕迹，一个都不该在。
  for (const ghost of ["林知遥", "zhiyao", "初二", "213"]) {
    await expect(guest.getByText(ghost, { exact: false })).toHaveCount(0);
  }
  await guest.screenshot({ path: "e2e/.shots/home-6-visitor-page.png", fullPage: true });

  await guest.setViewportSize({ width: 390, height: 844 });
  await guest.screenshot({ path: "e2e/.shots/home-7-visitor-phone.png", fullPage: true });
  await visitor.close();

  /* 7 · 门开了。 */
  await page.goto("/projects");
  await expect(page.getByRole("heading", { name: "最近想做点什么" })).toBeVisible();
  await expect(page.getByPlaceholder("比如：", { exact: false })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/home-8-gate-open.png", fullPage: true });
});
