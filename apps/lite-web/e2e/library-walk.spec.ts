import { test, expect, type Locator } from "@playwright/test";
import { freshAccount } from "./freshAccount";

/** 分级阅读库一页几篇。和 ReadingLibraryPage 的 LIBRARY_PAGE_SIZE 一致。 */
const LIBRARY_PAGE_SIZE = 12;

/**
 * 分级阅读库的走查 —— 从「不知道读什么」到一篇带照片的文章。
 *
 * 这条 walk 用 `freshAccount`：她的树是空的，所以推荐位走的是冷启动那一支
 * （补位的四篇，不说「因为你关心」）。排序本身由
 * apps/api/internal/library/library_test.go 覆盖，这里验的是**看得见的那一层**：
 * 照片真的出现在页面上、筛选真的筛得动、点进去的是那一档。
 *
 * 🚨 断言里有「图片真的有像素」这一条（naturalWidth > 0）。2026-08-30 的教训是
 * jsdom 里 344 个测试全绿而导出的 PNG 是全白的 —— 一张 404 的 <img> 在 DOM 里
 * 长得和一张好图一模一样，只有真浏览器分得出来。
 */

/** 一个作用域里已经取完的图有几张（complete 且有像素）。懒加载的图要先滚到，
 *  再等它取完，所以断言前用它轮询而不是直接数。 */
async function settledImages(scope: Locator): Promise<number> {
  return scope.locator("img").evaluateAll(
    (imgs) => imgs.filter((i) => (i as HTMLImageElement).complete && (i as HTMLImageElement).naturalWidth > 0).length,
  );
}

test.describe("分级阅读", () => {
  test("推荐 → 全部文章 → 读一篇带照片的", async ({ browser }, testInfo) => {
    const ctx = await freshAccount(browser, "library-walk");
    const page = await ctx.newPage();

    await page.goto("/readings");
    await expect(page.getByText("不知道读什么？")).toBeVisible();

    // 推荐位：四张卡，每张有照片、中文标题、一句理由和一档说明。
    const shelf = page.locator("section", { hasText: "不知道读什么？" });
    const cards = shelf.locator("article");
    await expect(cards).toHaveCount(4);
    await expect(cards.first().locator("img")).toBeVisible();
    await expect(shelf.getByText(/入门|基础|进阶|高阶|原文/).first()).toBeVisible();
    // 树是空的，所以不许出现「因为你关心」—— 把补位说成按兴趣挑的是在骗人。
    await expect(page.getByText(/因为你关心/)).toHaveCount(0);
    await testInfo.attach("shelf.png", { body: await page.screenshot({ fullPage: true }), contentType: "image/png" });

    // 每张封面都真的有像素。
    //
    // 🚨 封面是 loading="lazy" 的，而推荐位在首屏之下：不先滚到它、不等它把图
    // 取完就断言，四张里会有三张 naturalWidth 是 0 —— 那是这条测试跑得太快，
    // 不是图坏了。先滚过去，再等每一张 complete。
    await cards.last().scrollIntoViewIfNeeded();
    await expect
      .poll(async () => await settledImages(shelf), { timeout: 30_000 })
      .toBe(await shelf.locator("img").count());
    const broken = await shelf.locator("img").evaluateAll((imgs) =>
      imgs.filter((i) => !(i as HTMLImageElement).naturalWidth).map((i) => (i as HTMLImageElement).src),
    );
    expect(broken, "封面图没有像素").toEqual([]);

    await page.getByRole("button", { name: /查看全部 \d+ 篇/ }).click();
    await expect(page).toHaveURL(/\/readings\/library$/);
    await expect(page.getByRole("heading", { name: "分级阅读" })).toBeVisible();
    // 🚨 这里原来钉的是 `toHaveCount(20)` —— 库只有二十篇的那会儿。
    // 库现在是 48 篇，所以这条**在这次改动之前就已经是红的**
    // （又一次：[[inserting-a-step-rots-every-older-walk-2026-09-21]]）。
    // 2026-09-21 这一页又加了页码条，所以钉的换成两件不会随库大小改变的事：
    // 第一页满页，总数在页面上说得出来。
    await expect(page.locator("article")).toHaveCount(LIBRARY_PAGE_SIZE);
    await expect(page.getByRole("navigation", { name: "分页" })).toBeVisible();
    await testInfo.attach("library.png", { body: await page.screenshot({ fullPage: true }), contentType: "image/png" });

    // 搜索：打一个学科名，列表应该只剩挂着它的那几篇。
    await page.getByPlaceholder("搜索标题、话题或学科").fill("天文");
    const found = await page.locator("article").count();
    expect(found).toBeGreaterThan(0);
    expect(found, "搜一个学科名之后还是满满一页，等于没筛").toBeLessThan(LIBRARY_PAGE_SIZE);
    await page.getByPlaceholder("搜索标题、话题或学科").fill("");

    // 按主枝筛。
    await page.getByRole("button", { name: "科学与自然" }).click();
    const science = await page.locator("article").count();
    expect(science).toBeGreaterThan(0);
    expect(science).toBeLessThanOrEqual(LIBRARY_PAGE_SIZE);
    await page.getByRole("button", { name: "全部学科" }).click();
    await expect(page.locator("article")).toHaveCount(LIBRARY_PAGE_SIZE);

    // 「默认难度」这一排要真的改到卡片上。走查第一遍发现按钮亮了、卡片上的字
    // 一个都没动 —— 卡片的选中档是 useState 的初始值，不换 key 就不会重读。
    const firstCard = page.locator("article").first();
    await page.getByRole("button", { name: "入门", exact: true }).click();
    await expect(firstCard.getByText(/入门 · \d+L · \d+ 词/)).toBeVisible();
    await page.getByRole("button", { name: "高阶", exact: true }).click();
    await expect(firstCard.getByText(/高阶 · \d+L · \d+ 词/)).toBeVisible();

    // 挑一篇，换到原文那一档，读进去。
    const card = page.locator("article").first();
    const zhTitle = (await card.getByRole("heading").innerText()).trim();
    await card.getByRole("button", { name: "换一档" }).click();
    await card.getByRole("button", { name: "原文" }).click();
    await expect(card.getByText(/原文 · \d+ 词/)).toBeVisible();
    await card.getByRole("button", { name: "读这一篇" }).click();

    await expect(page).toHaveURL(/\/readings\/[0-9a-f-]{36}$/);
    // 阅读室里：正文有段落，题图在正文之前，图注带署名。
    const figures = page.locator("figure.mk-reading-figure");
    await expect(figures.first()).toBeVisible();
    // 🚨 这个表要跟着库走。库里实际有八种标记词（Photo: 468、
    // Photo credit: 50、Map: 15、Graphic: 15、Photos: 10、Image: 5、
    // Illustration: 5、Art: 5），而这里原来只认四种 —— 新一批文章进来之后
    // 它就红了，**在这次改动之前**。
    //
    // 要守的是「读者分得出自己在看照片还是看图表」，所以认的是「某某：」
    // 这个形状，不是某几个具体的词。
    await expect(figures.first().locator("figcaption")).toContainText(
      /(Photo|Photos|Photo credit|Graphic|Map|Image|Illustration|Art):/,
    );
    await figures.last().scrollIntoViewIfNeeded();
    const roomFigures = page.locator("figure.mk-reading-figure");
    await expect
      .poll(async () => await settledImages(roomFigures), { timeout: 30_000 })
      .toBe(await roomFigures.locator("img").count());
    const brokenInRoom = await page.locator("figure.mk-reading-figure img").evaluateAll((imgs) =>
      imgs.filter((i) => !(i as HTMLImageElement).naturalWidth).map((i) => (i as HTMLImageElement).src),
    );
    expect(brokenInRoom, "正文里的图没有像素").toEqual([]);
    // 小标题渲染成小标题，而不是一行 "## The Mission"。
    await expect(page.getByText(/^##\s/)).toHaveCount(0);
    await testInfo.attach("room.png", { body: await page.screenshot({ fullPage: true }), contentType: "image/png" });

    // 回到书架，这一篇停在**她打开的那一档**上，按钮是「继续读」。第一遍走查
    // 时它显示的是「读这一篇」—— 点下去会开出同一篇文章的第二条阅读记录。
    await page.goto("/readings/library");
    // 🚨 书架现在是分页的（一页 12 篇，共 48 篇），所以**不能假设她那一篇
    // 还在第一页**。先搜出来再断言 —— 这正是搜索框存在的理由，
    // 也正是「她回头找刚才那一篇」的真实走法。
    await page.getByPlaceholder("搜索标题、话题或学科").fill(zhTitle);
    const opened = page.locator("article", { hasText: zhTitle });
    await expect(opened).toHaveCount(1);
    // 🚨 她开着的那一档现在写在**按钮**上（「继续读你开着的那一档 · 原文」），
    // 不在卡片那行元信息里 —— 那一行显示的是「阅读难度」筛选器选中的档。
    // 这条原来钉的是那行元信息，在这次改动之前就已经对不上了。
    //
    // 要守的不变量没变：卡片记得她打开的是哪一档，所以按钮是「继续读」
    // 而不是「读这一篇」——第一遍走查时它显示「读这一篇」，点下去会开出
    // 同一篇文章的第二条阅读记录。
    const resume = opened.getByRole("button", { name: "继续读" });
    await expect(resume).toBeVisible();
    await expect(resume).toContainText("原文");

    await ctx.close();
  });
});
