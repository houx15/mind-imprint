import { expect, test, type Page } from "@playwright/test";
import { openSiteGate } from "./gate";
import { skipWhilePaused } from "./pausedSurfaces";

// 项目 / 我的主页 还没做完，这两个面上的走查先停。见 pausedSurfaces.ts。
skipWhilePaused();

/**
 * 旅程三 · 一个学生做一个项目，并且**把它做完**。
 *
 * 产品负责人 2026-09-04：「3) he/she can do a project and finish it.」
 *
 * ## 「做完」是哪一下
 *
 * `pbl_project.status` 有五档：构思中 → 进行中 → 待复盘 → 已落地 → 已归档。
 * 服务端五档都收（`pbl_projects.go` 的白名单 + `SetPblProjectStatus`），审完
 * 计划会自动进「进行中」。
 *
 * 🚨 但 2026-09-04 查下来，**界面上没有任何一处把项目往后推**：看板是只读的，
 * 整个 lite 前端只有 `NameAndCover` 调过 `updateProject`，而它只改名字和封面。
 * 也就是说她做完了也没办法说"做完了"——项目永远停在「进行中」。这条 walk 就是
 * 为了钉住那一下：**她自己把项目挪到「已落地」，刷新之后它还在那儿。**
 *
 * ## 这条 walk 借了一次接口，只借一次
 *
 * 开门（发布主页）走的是 `openSiteGate`。门本身归 `journey-website` 那条走，
 * 这一条要看的是门后面那个普通项目怎么从头做到尾。除此之外全是学生的动作。
 */

test.describe.configure({ retries: 0 });

const API = (process.env.E2E_API_BASE ?? "").replace(/\/+$/, "");
const IDEA = "我们学校每天中午剩很多饭，我想弄明白剩的到底是什么，然后做点什么。";

async function threadLen(page: Page, api: string): Promise<number> {
  const r = await page.request.get(`${api}/thread`);
  if (!r.ok()) return -1;
  const msgs = (await r.json()) as unknown[];
  return Array.isArray(msgs) ? msgs.length : -1;
}

async function say(page: Page, api: string, text: string): Promise<void> {
  const before = await threadLen(page, api);
  await page.getByPlaceholder("请输入").fill(text);
  await page.getByRole("button", { name: "发送" }).click();
  await expect
    .poll(() => threadLen(page, api), { timeout: 180_000, message: `这一轮没落库：${text}` })
    .toBeGreaterThan(before);
}

async function statusOf(page: Page, id: string): Promise<string> {
  const r = await page.request.get(`${API}/api/v1/pbl/projects`);
  if (!r.ok()) return "";
  const list = (await r.json()) as { id: string; status: string }[];
  return list.find((p) => p.id === id)?.status ?? "";
}

test("旅程三: 建一个项目 → 审计划 → 做一件工具 → 复盘 → 挪到已落地", async ({ page }) => {
  page.on("pageerror", (e) => console.log("PAGEERROR:", e.message));

  /* 0 · 门。归 journey-website 验，这里只借过。 */
  await page.goto("/projects");
  await openSiteGate(page);

  /* 1 · 她写下那句话，直接进房间。 */
  await page.goto("/projects");
  await page.getByPlaceholder("比如：", { exact: false }).fill(IDEA);
  await page.getByRole("button", { name: "开始", exact: true }).click();
  await expect(page).toHaveURL(/\/projects\/[0-9a-f-]{36}$/);
  const id = page.url().split("/").pop()!;
  const api = `${API}/api/v1/pbl/projects/${id}`;
  await expect
    .poll(() => threadLen(page, api), { timeout: 180_000, message: "开场那一轮没落库" })
    .toBeGreaterThanOrEqual(2);
  expect(await statusOf(page, id), "刚建出来该是构思中").toBe("talking");
  await page.screenshot({ path: "e2e/.shots/j3-1-room.png", fullPage: true });

  /* 2 · 聊到印记出一份计划，她审过，项目进「进行中」。 */
  //
  // 🚨 要**回答它的问题**，不是反复要一份计划。
  //
  // 第一版这里写的是三句「你帮我排一下步骤吧」，四轮下来一份计划都没有——因为
  // 印记的设计就是先把问题问清楚再谈步骤（一次只问一个）。一个真的学生是在
  // 答它的问题，答着答着计划才出来。用命令去催一个在提问的教练，测出来的是一段
  // 现实里不存在的对话。
  const approve = page.getByRole("button", { name: "审核完成，开始！" });
  const answers = [
    "中午十二点半我在食堂看过三天，第三个泔水桶每次都是满的。",
    "剩得最多的看着是米饭，不过我没称过，是看桶里的样子猜的。",
    "我问过三个同学，他们说打饭的时候阿姨给多少就是多少，自己不能选。",
    "我想先弄清楚到底剩了多少、剩的是什么，再看看能不能让同学自己选饭量。",
    "可以，就按这个顺序来。",
    "那请你把这几步写成一份计划，我来看看。",
    "就这样，写下来吧。",
  ];
  for (const n of answers) {
    if (await approve.count()) break;
    await say(page, api, n);
  }
  expect(
    await approve.count(),
    `聊了 ${answers.length} 轮（每一轮都在答它的问题），印记始终没有出一份可以审的计划`,
  ).toBeGreaterThan(0);
  await approve.click();
  await expect
    .poll(() => statusOf(page, id), { timeout: 30_000 })
    .toBe("running");
  await page.screenshot({ path: "e2e/.shots/j3-2-plan-approved.png", fullPage: true });

  /* 3 · 接着做，直到印记递一件工具过来。 */
  //
  // 🚨 干等是等不来的。递工具是**印记那一轮**里的判断（`summon_card`），她不说话
  // 就没有那一轮。第一版这里是一个 240 秒的空轮询，红了之后看起来像"印记不肯
  // 递工具"，其实是这条 walk 让学生坐在那儿一言不发地等了四分钟。
  const toolNames = async (): Promise<string[]> => {
    const r = await page.request.get(`${api}/tools`);
    if (!r.ok()) return [];
    return ((await r.json()) as { tool: string }[]).map((t) => t.tool);
  };
  for (const n of [
    "好，那我从第一步开始。我这周去食堂看了三天中午。",
    "我数了一下，第三个桶到十二点四十就满了。",
    "我想把看到的东西记下来，怎么记比较好？",
    "我还想弄清楚剩的到底是哪一类，米饭还是菜。",
  ]) {
    if ((await toolNames()).length > 0) break;
    await say(page, api, n);
  }
  expect(
    (await toolNames()).length,
    "审完计划、又聊了四轮，印记一件工具都没递 —— 那份计划就没有落到任何一件事上",
  ).toBeGreaterThan(0);
  await page.screenshot({ path: "e2e/.shots/j3-3-tool-handed.png", fullPage: true });

  /* 4 · 复盘。这是「做完」这件事的仪式。 */
  for (const n of [
    "这个项目我做得差不多了，想回头看一遍。",
    "帮我复盘一下这个项目吧。",
    "我想做项目复盘。",
  ]) {
    if ((await toolNames()).includes("lookback")) break;
    await say(page, api, n);
  }
  expect(
    (await toolNames()).includes("lookback"),
    "她连着说了三轮要复盘，印记始终没有递「项目复盘」",
  ).toBe(true);

  /* 5 · 她把项目挪到「已落地」。 */
  await page.goto("/projects");
  await page.getByRole("button", { name: "按状态" }).click();
  const card = page.getByTestId(`project-card-${id}`);
  const target = page.getByTestId("status-col-keeping");
  await expect(card).toBeVisible();
  await expect(target).toBeVisible();

  const from = (await card.boundingBox())!;
  const to = (await target.boundingBox())!;
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
  await page.mouse.down();
  await page.mouse.move(to.x + to.width / 2, to.y + 60, { steps: 20 });
  await page.mouse.up();

  await expect.poll(() => statusOf(page, id), { timeout: 30_000 }).toBe("keeping");
  await page.reload();
  await page.getByRole("button", { name: "按状态" }).click();
  await expect(
    page.getByTestId("status-col-keeping").getByTestId(`project-card-${id}`),
  ).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/j3-4-finished.png", fullPage: true });
});
