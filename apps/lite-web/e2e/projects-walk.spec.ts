import { expect, test } from "@playwright/test";
import { openSiteGate } from "./gate";
import { skipWhilePaused } from "./pausedSurfaces";

// 项目 / 我的主页 还没做完，这两个面上的走查先停。见 pausedSurfaces.ts。
skipWhilePaused();

/**
 * 项目 walk — the landing page, the create flow, the board, and the room.
 *
 * This exists to be LOOKED AT. The 2026-08-30 lesson was 344 green tests
 * sitting on top of an exported PNG that was completely blank, so the point of
 * this file is the screenshots it drops in `e2e/.shots/`, not the handful of
 * assertions around them. Open them.
 *
 * No model key is needed anywhere in this walk: building a project never calls
 * a model (migration 0112 — the category is hers), and the walk never sends a
 * turn.
 *
 * 🚨 这条 walk 先开门。spec §4 的门自 2026-09-03 起是真的了：主页发布之前，
 * 自由项目一律 409。所以「一个正在开第二个项目的学生」必然是一个主页已经在线
 * 上的学生——`openSiteGate` 把她放到那个位置上。门本身由 `homepage-walk.spec.ts`
 * 从头到尾看一遍。
 */
test("项目: empty state → create → name and cover → room → board", async ({ page }) => {
  // 先把主页做完发布——门后面才有「开第二个项目」这件事。
  await page.goto("/projects");
  await openSiteGate(page);
  await page.goto("/projects");

  // 1 · The empty state: the box is here because her page is live.
  await expect(page.getByRole("heading", { name: "最近想做点什么" })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/projects-1-empty.png", fullPage: true });

  // 2 · Write an idea and start.
  const box = page.getByPlaceholder("比如：", { exact: false });
  await box.fill("我们学校每天剩好多饭，我想弄明白这些饭最后去哪了，能不能少一点。");
  await page.screenshot({ path: "e2e/.shots/projects-2-typed.png", fullPage: true });
    // 🚨 `exact: true`：看板上的项目卡片本身是个 button，可访问名里带着
  // 「开始于 2026-09-03」，默认的子串匹配会同时命中它和输入框旁边的「开始」。
  // 看板一有卡片就撞——homepage-walk 先跑过之后就是这样。
  await page.getByRole("button", { name: "开始", exact: true }).click();

  // 3 · 不再弹命名窗（产品负责人 2026-09-02）：她写完那句话就直接进房间，
  //     而那句话就是她对印记说的第一句。名字先由服务端给一个短的，她随时改。
  await expect(page).toHaveURL(/\/projects\/[0-9a-f-]{36}$/);
  // 🚨 等她那句话真的出现在对话里——房间的输入框在数据到达之前就画出来了，
  // 等它等于什么都没等。这一轮要真的调模型，所以给足时间。
  await expect(
    page.getByText("我想弄明白这些饭最后去哪了", { exact: false }).first(),
  ).toBeVisible({ timeout: 90_000 });
  await page.screenshot({ path: "e2e/.shots/projects-3-room.png", fullPage: true });

  // 4 · 回到项目列表，卡片上有她刚开的这个。
  await page.getByRole("button", { name: "回到项目" }).click();
  await expect(page.getByText("我们学校每天剩好多饭").first()).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/projects-4-cards.png", fullPage: true });

  // 5 · 按状态看——看板现在是一个视图，不是默认那一个。
  await page.getByRole("button", { name: "按状态" }).click();
  await expect(page.getByRole("heading", { name: "构思中" })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/projects-5-board.png", fullPage: true });
  await page.getByRole("button", { name: "全部" }).click();

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
  await expect(page.getByText("我们学校每天剩好多饭").first()).toBeVisible();

  // 🚨 Asserted, not just screenshotted: this exact pair was silently WRONG
  // once — with the theme flipped at runtime the tokens read dark at :root
  // while the card still painted white with light-theme text. Light on light,
  // unreadable, and no test would have noticed. It is correct only when the
  // theme is applied before first paint, which is what bootTheme does.
  const dark = await page.evaluate(() => {
    const card = document.querySelector('[class*="grid"] button') as HTMLElement | null;
    return card
      ? { bg: getComputedStyle(card).backgroundColor, fg: getComputedStyle(card).color }
      : null;
  });
  expect(dark?.bg, "a project card must take the dark surface token").toBe("rgb(35, 30, 27)");
  expect(dark?.fg, "its text must take the dark ink token").toBe("rgb(240, 233, 227)");
  await page.screenshot({ path: "e2e/.shots/projects-8-dark-board.png", fullPage: true });

  await page.getByText("我们学校每天剩好多饭").first().click();
  await expect(page.getByPlaceholder("请输入")).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/projects-9-dark-room.png", fullPage: true });
});
