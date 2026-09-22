import { expect, test, type Page } from "@playwright/test";
import { freshAccount } from "./freshAccount";

/**
 * 标注板走查 —— 写作房间里第一件她用手摆的东西，跑在真的 API 和真的库上。
 *
 * 这条 walk 存在的理由不是「板渲染出来了」这种断言，而是**闭环真的闭上了**：
 * 她摆完 → 摆的结果变成一条真的学生消息 → 印记 对着它回一轮。
 * 阅读室在这条回路上断过一次（2026-09-03 同事试用：「透镜应用完毕之后，
 * 没有响应，没有推进到下一步」），所以这条链子的终点必须被断言，不是被相信。
 *
 * 🚨 用 `freshAccount` 自己注册一个账号，不蹭种子里的 phoebe：
 * 那份前提由文件字母序在守，一个新 spec 就破（见 freshAccount.ts）。
 *
 * 🚨 它也是这块板的**眼睛**。jsdom 看不见布局、看不见 pointer 拖动，
 * 2026-08-30 的教训是 344 个测试全绿而导出的 PNG 是全白的。所以这条 walk
 * 落三张截图到 `e2e/.shots/`，要看的是那三张图。
 */

const SHOTS = "e2e/.shots";
const BOX_PLACEHOLDER = "说说你想写点什么，直接开始";

/** 印记 真正落地的那几条回复。ThinkingRow 同时带 data-role 和 aria-label，
 *  真回复只带前者——`:not([aria-label])` 分开的就是「冒了个泡」和「回来了」。 */
function assistantReplies(page: Page) {
  return page.locator('[data-role="assistant"]:not([aria-label])');
}

test("标注板：她摆完，印记 接住，而且那一轮真的回来了", async ({ browser }) => {
  test.setTimeout(8 * 60_000);

  const ctx = await freshAccount(browser, "writing-board-walk");
  const page = await ctx.newPage();

  // —— 进写作，开一篇 ——
  await page.goto("/writings");
  await page.getByPlaceholder(BOX_PLACEHOLDER).fill("学校食堂每天倒掉的饭太多了，我想写这件事");
  await page.getByRole("button", { name: "开始写作", exact: true }).click();

  // 设定弹窗：什么都不填，走「跳过」——篇幅从来不是任何一步的前提。
  const dialog = page.getByRole("dialog", { name: "开始之前" });
  await expect(dialog).toBeVisible({ timeout: 30_000 });
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/setup") && r.request().method() === "PUT"),
    dialog.getByRole("button", { name: "跳过", exact: true }).click(),
  ]);
  await expect(dialog).toHaveCount(0, { timeout: 15_000 });

  // —— 出规划，进段落 ——
  //
  // 🚨 **结构那一步是全屏的，它在房间那套外壳渲染之前就分叉了**
  // （WritingRoomHost：stage === "outline" 直接 return PlanningView），
  // 所以这一屏上**根本没有「写作四步」那条导航**。第一版这里调 jumpStage，
  // 等一个永远不会发出的 POST /stage，15 秒超时。
  // 出去的那颗按钮是「去写」，而它从第一秒就在——规划不是关卡。
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/stage") && r.request().method() === "POST"),
    page.getByRole("button", { name: /去写/ }).click(),
  ]);

  // 🚨 **规划和段落之间多了一步**（R3，2026-09-20：同事的意见 4「前期逻辑
  // 讨论的部分需要增加一个对于行文方式的思考和梳理部分」）。「去写」现在落在
  // 「行文」那一屏，不再直接到段落。
  //
  // 这条走查是 R3 之前写的，它等的是「段落」那个标题，于是在行文那一屏上
  // 干等 30 秒 —— 读起来像标注板坏了，其实是路上多了一站。
  // R3 当时只跑了 flow-stage 那一条新的，没回头跑这几条旧的。
  await expect(page.getByRole("heading", { name: "行文" })).toBeVisible({ timeout: 30_000 });
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/stage") && r.request().method() === "POST"),
    page.getByRole("button", { name: /去写段落/ }).click(),
  ]);
  await expect(page.getByRole("heading", { name: "段落" })).toBeVisible({ timeout: 30_000 });

  // —— 写一段：四句话，故意**只有主张和例子，没有一句解释** ——
  //
  // 这正是这块板要让她自己看见的那件事（qifeng 的 `evidence_not_explained`：
  // 举了例子却没有一句话说清它凭什么支持主张）。摆完之后「解释」那一格是空的，
  // 而那个空格会写进回灌里。
  const para = [
    "学校食堂每天倒掉的饭特别多。",
    "上周五我数了一下，回收桶里有六个桶是满的。",
    "光是那一天，倒掉的米饭大概能装满两个洗菜盆。",
    "我觉得这件事值得写。",
  ].join("");

  // 🚨 提纲是空的（她没经过规划就直接来写），所以段落这一页上**一个块都没有**，
  // 显示的是空状态。第一版这里直接等那个输入框，等了 60 秒——
  // 输入框只跟着提纲的块长出来。走「加一段」这条自由写的路。
  await page.getByRole("button", { name: "加一段" }).click();

  const box = page.getByPlaceholder("写这一段……").first();
  await expect(box).toBeVisible({ timeout: 60_000 });
  await box.fill(para);
  await box.blur();

  // —— 板：只有这一段真的有两句以上才出现 ——
  const open = page.getByRole("button", { name: "标一下这一段" }).first();
  await expect(open).toBeVisible({ timeout: 30_000 });
  await open.click();

  const board = page.locator(".mk-board").first();
  await expect(board).toBeVisible();
  await page.screenshot({ path: `${SHOTS}/writing-board-open.png`, fullPage: true });

  // 板上的卡片必须是**她写的那几句**，一句都不多、不少。
  const chips = board.locator(".mk-board__chip");
  await expect(chips).toHaveCount(4);
  await expect(chips.first()).toHaveText("学校食堂每天倒掉的饭特别多。");

  // 全摆完之前「标好了」必须是灰的——摆了一半就交，这块板什么也没测。
  const submit = board.locator(".mk-board__submit");
  await expect(submit).toBeDisabled();

  // —— 摆：点一张卡，再点一个格子（拖和点两条路终态一样，点这条在 CI 里稳） ——
  //
  // 故意摆成 主张 / 证据 / 证据 / 主张，一句「解释」都不给。
  //
  // 🚨 手势照抄 `e2e/readwalk/readwalk.spec.ts` 那条已经跑通的：
  // 未分类那一堆用 `.mk-board__loose .mk-board__chip` 选（**不是**
  // `[data-board-bin=""]`），点完卡再点格子，中间留一下让状态落定。
  // 第一版自己编了一套选择器，四张全摆完之后「标好了」还是灰的。
  for (const bin of ["主张", "证据", "证据", "主张"]) {
    // 每摆一张，未分类那一堆就少一张，所以永远点第一张。
    await board.locator(".mk-board__loose .mk-board__chip").first().click();
    await board.locator(".mk-board__bin").filter({ hasText: bin }).first().click();
    await page.waitForTimeout(400);
  }

  // 先拍照再断言：断言失败的时候我要看得见板当时的样子，而不是只有一行
  // 「toBeEnabled failed」。
  await page.screenshot({ path: `${SHOTS}/writing-board-placed.png`, fullPage: true });
  const loose = await board.locator(".mk-board__loose .mk-board__chip").count();
  const inBins = await board.locator(".mk-board__bin .mk-board__chip").count();
  console.log(`[board] 未分类 ${loose} 张，格子里 ${inBins} 张`);
  await expect(submit).toBeEnabled();

  // —— 闭环：她摆的结果要以**她说的话**的身份出现在对话里 ——
  const beforeReplies = await assistantReplies(page).count();
  await submit.click();

  // 🚨 只钉**不变的那一截**。前面那句话会带上这一段的标题
  // （composeRoleBoardAnswer：`我给「分论点 1」这一段的……`，2026-09-18 加的，
  // 为的是让印记知道她标的是开头还是主体段）。整句钉死的话，一个有标题的
  // 段落就永远对不上 —— 而那正是它该工作的样子。
  await expect(page.locator('[data-role="student"]', { hasText: "每一句标了它在干什么" })).toBeVisible({
    timeout: 60_000,
  });
  // 🚨 缺口那一行才是这块板真正的产出。没有它，印记 只看到她标对了什么，
  // 看不到这一段缺什么——而缺什么是下一轮该谈的事。
  await expect(page.locator('[data-role="student"]', { hasText: "这一段里没有：" })).toBeVisible();

  // 摆完之后板收起来——她刚动完手，不该马上又看见同一块板。
  await expect(board).toBeHidden({ timeout: 60_000 });

  // —— 印记 真的多回了一轮 ——
  //
  // 🚨 数的是 assistant 那几条**比摆之前多了**，不是「页面上有那段字」。
  // 2026-09-05 栽过一次假绿：断言那段话出现了，而它就在输入框里。
  await expect
    .poll(async () => await assistantReplies(page).count(), {
      timeout: 150_000,
      message: "印记 在她摆完板之后必须回一轮",
    })
    .toBeGreaterThan(beforeReplies);

  await page.screenshot({ path: `${SHOTS}/writing-board-answered.png`, fullPage: true });
  await ctx.close();
});
