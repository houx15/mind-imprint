import { expect, test } from "@playwright/test";

/**
 * 项目 walk — the landing page, the create flow, the board, and the room.
 *
 * This exists to be LOOKED AT. The 2026-08-30 lesson was 344 green tests
 * sitting on top of an exported PNG that was completely blank, so the point of
 * this file is the screenshots it drops in `e2e/.shots/`, not the handful of
 * assertions around them. Open them.
 *
 * No model key is needed anywhere in this walk: a student's FIRST project is
 * her homepage by rule (spec §4), so the server skips the classifier, and the
 * walk never sends a turn.
 */
test("项目: empty state → create → name and cover → room → board", async ({ page }) => {
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

  // 5 · Naming drops her straight INTO the room — she came here to start
  // something, not to find it again on a board.
  await expect(page).toHaveURL(/\/projects\/[0-9a-f-]{36}$/);
  await expect(page.getByPlaceholder("跟印记说")).toBeVisible();
  // 🚨 Wait for the project's own NAME, not just the composer. The composer and
  // 「还没有计划」 both render in the room's INITIAL state, before any data has
  // arrived — asserting on them screenshotted a half-loaded room whose header
  // still said the fallback 「项目」. The name only appears once the load
  // resolved, so it is the honest signal that the room is actually up.
  await expect(page.getByRole("heading", { name: "计划" })).toBeVisible();
  await expect(page.locator("header").getByText("剩饭去哪了")).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/projects-5-room.png", fullPage: true });

  // 6 · Back to the board, with her project on it.
  await page.getByRole("button", { name: "回到项目" }).click();
  await expect(page.getByText("剩饭去哪了")).toBeVisible();
  await expect(page.getByRole("heading", { name: "在聊" })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/projects-6-board.png", fullPage: true });

  // 7 · Phone width — the columns scroll, the page does not.
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: "e2e/.shots/projects-7-phone.png", fullPage: true });
  const overflows = await page.evaluate(
    () => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
  );
  expect(overflows, "the page itself must never scroll sideways").toBe(false);

  // 8 · The room and the board in the dark theme. Contrast on the column
  // headers, the cover glyphs and the plan panel is where a warm-paper palette
  // most often goes muddy, so these exist to be looked at.
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.evaluate(() => localStorage.setItem("mk-theme", "dark"));
  await page.reload();
  await expect(page.getByText("剩饭去哪了")).toBeVisible();

  // 🚨 Asserted, not just screenshotted: this exact pair was silently WRONG
  // once — with the theme flipped at runtime the tokens read dark at :root
  // while the card still painted white with light-theme text. Light on light,
  // unreadable, and no test would have noticed. It is correct only when the
  // theme is applied before first paint, which is what bootTheme does.
  const dark = await page.evaluate(() => {
    const card = document.querySelector("section button") as HTMLElement | null;
    return card
      ? { bg: getComputedStyle(card).backgroundColor, fg: getComputedStyle(card).color }
      : null;
  });
  expect(dark?.bg, "a project card must take the dark surface token").toBe("rgb(35, 30, 27)");
  expect(dark?.fg, "its text must take the dark ink token").toBe("rgb(240, 233, 227)");
  await page.screenshot({ path: "e2e/.shots/projects-8-dark-board.png", fullPage: true });

  await page.getByText("剩饭去哪了").click();
  await expect(page.getByPlaceholder("跟印记说")).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/projects-9-dark-room.png", fullPage: true });
});
