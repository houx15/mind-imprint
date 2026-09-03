import { expect, test } from "@playwright/test";

/**
 * 主页 walk — spec §4 那道门，以及主页项目**在项目房间里**这件事。
 *
 * 这个文件存在是为了**被看**。2026-08-30 的教训是 344 条绿测试压在一张全白的
 * 导出图上，所以这里真正的产出是 `e2e/.shots/` 里那几张图，不是断言。
 *
 * ## 这一版改了什么
 *
 * 上一版走的是 `SiteStudio`：三步、九个带 label 的输入框、三张版式卡。产品负责
 * 人 2026-09-03 否掉了它（"don't let students enter forms"；"you make it an
 * independent thing which is not related with pbl"），那个工作面已经退役。
 *
 * 主页项目现在开的是普通的项目房间。所以这条 walk 断言的是**房间里的东西真的
 * 在**：印记、那份五步的任务清单、材料清单、上线那一屏。
 *
 * ## 为什么中间要用 API 塞一次内容
 *
 * 第一到四关（受众、看真站、定调子、生成）每一关都要真的模型调用。这条 walk 是
 * 纯数据路径的那一条，跑得快、每次 CI 都跑，所以它跳过那四关，直接把「印记摆好
 * 了她的话」这个状态塞进去，再验后面那半程。模型那半程归 `LIVE_LLM=1` 的实测。
 */
test("主页: 门 → 项目房间（五步清单）→ 上线那一屏 → 访客看到的那一页 → 门开了", async ({
  page,
  browser,
}) => {
  // 页面崩了的时候，断言只会说「没找到元素」。把浏览器里的真实报错抬到
  // Playwright 的输出里，省掉一轮猜。
  page.on("pageerror", (e) => console.log("PAGEERROR:", e.message));
  page.on("console", (m) => {
    if (m.type() === "error") console.log("CONSOLE ERROR:", m.text());
  });

  /* 1 · 门。她还没有主页，所以这里没有自由输入框。 */
  await page.goto("/projects");
  await expect(page.getByRole("heading", { name: "先做你自己的主页。" })).toBeVisible();
  await expect(page.getByPlaceholder("比如：", { exact: false })).toHaveCount(0);
  await page.screenshot({ path: "e2e/.shots/home-1-gate.png", fullPage: true });

  /* 2 · 进主页项目。开的是项目房间，不是一个独立工作面。 */
  await page.getByRole("button", { name: /做我的主页/ }).click();
  await expect(page.getByText("我自己的主页", { exact: false }).first()).toBeVisible();
  // 驱动问题就印在房间顶上——这是「question-defined project」这件事看得见的证据。
  await expect(page.getByText("我想让谁，看见我的什么？").first()).toBeVisible();

  // 🚨 这一组断言就是这次改动本身：退役掉的那九个 label 一个都不该再出现。
  for (const field of ["首屏那句话", "开场一段", "页头几个词", "选它的理由"]) {
    await expect(page.getByText(field, { exact: true })).toHaveCount(0);
  }

  /* 3 · 预置的五步路线。她进来就看见一份真的任务清单，并且要自己审一遍。 */
  for (const step of [
    "想清楚给谁看",
    "去看真的个人网站",
    "给网站定调子",
    "我来生成，你来分工",
    "逐处审改，然后上线",
  ]) {
    await expect(page.getByText(step, { exact: false }).first()).toBeVisible();
  }
  await page.screenshot({ path: "e2e/.shots/home-2-seeded-routine.png", fullPage: true });

  await page.getByRole("button", { name: /审核完成/ }).click();

  /* 4 · 上线那一屏。她从材料清单点开——发布不是终点，这一行永远在。 */
  await page.getByRole("button", { name: /我的主页/ }).click();
  await expect(page.getByRole("heading", { name: "上线" })).toBeVisible();
  // 页面上还没有她自己的字，所以服务端不让发，界面也说清楚缺什么。
  await expect(page.getByText("还缺你自己的话")).toBeVisible();
  await expect(page.getByRole("button", { name: "上线", exact: true })).toBeDisabled();
  await page.screenshot({ path: "e2e/.shots/home-3-ship-blocked.png", fullPage: true });

  /* 5 · 站在第四关结束的那个状态上：印记已经把她说过的话摆好了。
     真实路径是 site_content 产出（要模型）；这里直接塞，理由见文件头。 */
  const seeded = await page.request.put("/api/v1/pbl/site/content", {
    data: {
      headline: "一件还能修的东西，是谁决定它该被扔的？",
      role: "读 IB 的高二学生 · 在拆东西",
      lead: "我在拆家里所有还能拆的东西，然后写为什么它们修不好。",
      about: [
        "我读高二，在 IB。三年前家里一盏灯坏了，售后说只能整只换，我第一次意识到「修不好」有时候是被设计成这样的。",
      ],
      now: "在把这一页做出来。",
      nowList: [],
      tags: [],
      motto: [],
      contact: "",
      blurbs: {},
    },
  });
  expect(seeded.ok()).toBeTruthy();

  // 第三关的落点：风格 + 配色。真实路径是「视觉基调」那件工具（配色由模型从她的
  // 关键词派生），这里直接塞一组合法的，理由同上。发布闸查的就是这一格。
  const look = await page.request.put("/api/v1/pbl/site/look", {
    data: {
      layout: "essay",
      palette: {
        label: "车间灯",
        why: "配「动手」和「不怕拆坏」",
        paper: "#F5F2EC",
        ink: "#1E1C19",
        accent: "#2F5D8A",
      },
    },
  });
  expect(look.ok()).toBeTruthy();

  /* 6 · 上线。 */
  await page.reload();
  await page.getByRole("button", { name: /我的主页/ }).click();
  await expect(page.getByText("还缺你自己的话")).toHaveCount(0);
  await page.screenshot({ path: "e2e/.shots/home-4-ship-ready.png", fullPage: true });

  await page.getByRole("button", { name: "上线", exact: true }).click();
  await expect(page.getByText("已上线")).toBeVisible({ timeout: 30_000 });
  const url = await page.getByRole("link", { name: /\/p\// }).getAttribute("href");
  expect(url).toContain("/p/");
  await page.screenshot({ path: "e2e/.shots/home-5-published.png", fullPage: true });

  /* 7 · 访客那一面：完全没有 session。
     🚨 这一张是整条线最要紧的证据。原型走到这里得到的是林知遥的页面——页头
     「林知遥 · 初二学生」压着她自己写的那一行，关于是别人的自传。那份原型
     （`src/eco/`）已经连同它的假学生一起删掉了。 */
  const visitor = await browser.newContext({ storageState: { cookies: [], origins: [] } });
  const guest = await visitor.newPage();
  await guest.goto(url!);
  await expect(guest.getByText("一件还能修的东西，是谁决定它该被扔的？")).toBeVisible();
  for (const ghost of ["林知遥", "zhiyao", "初二", "213"]) {
    await expect(guest.getByText(ghost, { exact: false })).toHaveCount(0);
  }
  // 🚨 她定下的配色必须真的到达访客那一页。
  //
  // 这条断言存在是因为肉眼在一张缩略图上分不清 #9C3B26（版式自带的锈红）和
  // #2F5D8A（她挑的靛蓝）——而这两者的差别，正是「她挑了配色」这件事是真是假。
  // 量一次计算出来的颜色，比看一眼可靠。
  const accent = await guest.evaluate(() =>
    getComputedStyle(document.querySelector(".mk-site")!).getPropertyValue("--st-accent").trim(),
  );
  expect(accent.toLowerCase()).toBe("#2f5d8a");

  await guest.screenshot({ path: "e2e/.shots/home-6-visitor-page.png", fullPage: true });

  await guest.setViewportSize({ width: 390, height: 844 });
  await guest.screenshot({ path: "e2e/.shots/home-7-visitor-phone.png", fullPage: true });
  await visitor.close();

  /* 8 · 门开了。 */
  await page.goto("/projects");
  await expect(page.getByRole("heading", { name: "最近想做点什么" })).toBeVisible();
  await expect(page.getByPlaceholder("比如：", { exact: false })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/home-8-gate-open.png", fullPage: true });
});
