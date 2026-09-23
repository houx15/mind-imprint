import { expect, test } from "@playwright/test";
import { freshAccount } from "./freshAccount";

test("a student builds a showcase, manages works, then returns to revise it", async ({ browser }) => {
  const context = await freshAccount(browser, "showcase-guide");
  const page = await context.newPage();
  try {
    await page.goto("/site");
    await expect(page.getByRole("heading", { name: "我们来搭建一个属于你的个人作品空间吧。" })).toBeVisible();
    await expect(page.locator(".showcase-preview-frame")).toHaveClass(/is-dormant/);
    await page.getByRole("button", { name: "开始制作" }).click();
    await expect(page.getByRole("heading", { name: "选择喜欢的样子" })).toBeVisible();
    await page.getByRole("button", { name: "看开场" }).click();
    await expect(page.getByRole("heading", { name: "欢迎来到你的空间" })).toBeVisible();
    await page.getByRole("button", { name: "写介绍" }).click();
    await page.getByLabel("个人简介").fill("我喜欢研究自然与设计，也会在这里记录自己的作品。");
    await page.getByRole("button", { name: "选作品" }).click();
    await page.getByRole("button", { name: "打开作品管理" }).click();
    await expect(page).toHaveURL(/\/site\/works$/);
    await expect(page.getByRole("heading", { name: "管理公开作品" })).toBeVisible();
    await page.getByRole("button", { name: "返回主页制作" }).click();
    await expect(page).toHaveURL(/\/site$/);
    await page.getByRole("button", { name: "加元素" }).click();
    await page.getByRole("button", { name: "检查主页" }).click();
    await page.getByRole("button", { name: "完成初版" }).click();
    await expect(page.getByText("你已经有一个主页版本。想修改哪里？")).toBeVisible();
    await page.reload();
    await expect(page.getByText("你已经有一个主页版本。想修改哪里？")).toBeVisible();
    await page.getByRole("button", { name: "更换首图" }).click();
    await expect(page.getByRole("heading", { name: "欢迎来到你的空间" })).toBeVisible();
    await expect(page.getByRole("button", { name: "自定义图片" })).toHaveAttribute("aria-pressed", "true");
    await page.getByRole("button", { name: "继续修改其他部分" }).click();
    await page.getByPlaceholder("告诉印记你的想法或修改要求…").fill("我想更换个人头像");
    await page.getByRole("button", { name: "发送给印记" }).click();
    await expect(page.getByRole("heading", { name: "个人照片或头像", level: 2 })).toBeVisible({ timeout: 60_000 });
    await expect(page.getByRole("button", { name: "头像" })).toHaveAttribute("aria-pressed", "true");
  } finally {
    await context.close();
  }
});
