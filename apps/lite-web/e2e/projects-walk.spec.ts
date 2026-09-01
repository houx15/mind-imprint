import { expect, test } from "@playwright/test";

/**
 * 项目 S1 walk — the landing page, the create flow, and the board.
 *
 * This exists to be LOOKED AT. The 2026-08-30 lesson was 344 green tests
 * sitting on top of an exported PNG that was completely blank, so the point of
 * this file is the screenshots it drops in `e2e/.shots/`, not the handful of
 * assertions around them. Open them.
 *
 * No model key is needed: a student's FIRST project is her homepage by rule
 * (spec §4), and the server skips the classifier entirely in that case.
 */
test("项目: empty state → create → name and cover → board", async ({ page }) => {
  await page.goto("/projects");

  // 1 · The empty state: the box, and one line about the first project.
  await expect(page.getByRole("heading", { name: "最近想做点什么" })).toBeVisible();
  await expect(page.getByText("你的第一个项目是做一个属于你自己的主页", { exact: false })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/projects-1-empty.png", fullPage: true });

  // 2 · Write an idea and start.
  const box = page.getByPlaceholder("比如：", { exact: false });
  await box.fill("我们学校每天剩好多饭，我想弄明白这些饭最后去哪了，能不能少一点。");
  await page.screenshot({ path: "e2e/.shots/projects-2-typed.png", fullPage: true });
  await page.getByRole("button", { name: "开始" }).click();

  // 3 · The modal opens on the project the server just made.
  await expect(page.getByRole("heading", { name: "给它起个名字" })).toBeVisible();
  // Her own sentence is quoted back; the name field is EMPTY on purpose.
  await expect(page.getByPlaceholder("你想叫它什么")).toHaveValue("");
  await page.screenshot({ path: "e2e/.shots/projects-3-modal.png", fullPage: true });

  // 4 · Name it, pick a different ground, confirm.
  await page.getByPlaceholder("你想叫它什么").fill("剩饭去哪了");
  await page.getByRole("button", { name: "matcha" }).click();
  await page.screenshot({ path: "e2e/.shots/projects-4-modal-filled.png", fullPage: true });
  await page.getByRole("button", { name: "就这样" }).click();

  // 5 · The board, with her project in it.
  await expect(page.getByRole("heading", { name: "给它起个名字" })).toBeHidden();
  await expect(page.getByText("剩饭去哪了")).toBeVisible();
  await expect(page.getByRole("heading", { name: "在聊" })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/projects-5-board.png", fullPage: true });

  // 6 · Phone width — the columns scroll, the page does not.
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: "e2e/.shots/projects-6-phone.png", fullPage: true });
  const overflows = await page.evaluate(
    () => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
  );
  expect(overflows, "the page itself must never scroll sideways").toBe(false);

  // 7 · The same board in the dark theme. Contrast on the column headers and
  // the cover glyphs is where a warm-paper palette most often goes muddy, so
  // this shot exists to be looked at, not asserted on.
  await page.setViewportSize({ width: 1280, height: 720 });
  // Store the preference and RELOAD, which is how a student actually gets the
  // dark theme: `bootTheme()` applies it in main.tsx before the first render.
  await page.evaluate(() => localStorage.setItem("mk-theme", "dark"));
  await page.reload();
  await expect(page.getByText("剩饭去哪了")).toBeVisible();
  // 🚨 The card is asserted, not just screenshotted, because this exact pair
  // was silently WRONG once: with the theme flipped at runtime the tokens read
  // dark at :root while the card still painted white with light-theme text —
  // light on light, unreadable, and no test would have noticed. It is correct
  // when the theme is applied before first paint, which is what bootTheme does.
  const dark = await page.evaluate(() => {
    const card = document.querySelector("section button") as HTMLElement | null;
    return card
      ? { bg: getComputedStyle(card).backgroundColor, fg: getComputedStyle(card).color }
      : null;
  });
  expect(dark?.bg, "a project card must take the dark surface token").toBe("rgb(35, 30, 27)");
  expect(dark?.fg, "its text must take the dark ink token").toBe("rgb(240, 233, 227)");
  await page.screenshot({ path: "e2e/.shots/projects-7-dark.png", fullPage: true });
  await page.getByText("剩饭去哪了").click();
  await expect(page.getByRole("heading", { name: "给它起个名字" })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/projects-8-dark-modal.png", fullPage: true });
});
