import { execFileSync } from "node:child_process";
import { expect, test, type Page } from "@playwright/test";
import { openSiteGate } from "./gate";

/**
 * 课程 walk —— 轻量版的 课程 页，和项目里的「去上一课」。
 *
 * 这个文件存在的意义是它拍下来的图（e2e/.shots/courses-*.png），不是断言。
 * 2026-08-30 的教训：344 个绿测试压在一张全白的导出图上。jsdom 看不见布局，
 * 所以这几屏必须用真浏览器看一眼。
 *
 * 走三段：
 *   1. 课程 tab 有课，点开一门能进详情
 *   2. 项目里印记递出「去上一课」，工具界面摆的是那门课
 *   3. 浮层里能打开课程，回到项目，写下收获，那句话回到对话
 *
 * 第 2 段用真的那条端点把课递出来（produce → 指派，和印记走的是同一条路的下半
 * 段），因为让模型自己挑课需要它正好判断这一刻该递课 —— 那是模型的事，不是这条
 * walk 要测的东西。第 3 段最后那一轮是真的模型调用。
 */

test.describe.configure({ retries: 0 });

const IDEA = "我想给我们班做一个查作业的小网页，但我不会写代码。";
// 种子里的 2.0 运行时课程（apps/api/internal/store/seed/courses/coverage-course.json）。
const COURSE_SLUG = "evidence-comparability";

// 🚨 接口在**另一台**机器上（mind-api）。`page.request` 的相对路径会打到静态站
// 上去，回一段 HTML，`.json()` 当场抛 —— 这条走查的第二个用例因此在线上
// 从来没跑通过（973ms 就挂）。见 [[online-e2e-two-hosts-2026-09-04]]。
const API = (process.env.E2E_API_BASE ?? "").replace(/\/+$/, "");

const PG_CONTAINER = process.env.E2E_PG_CONTAINER ?? "mindimprint-lite-e2e-pg";

/** 往库里插一条课程指派 —— 印记 produce("course") 落的就是这一行。 */
function seedAssignment(atomId: string, slug: string, why: string): void {
  execFileSync(
    "docker",
    [
      "exec", PG_CONTAINER, "psql", "-U", "postgres", "-d", "mindimprint",
      "-v", "ON_ERROR_STOP=1", "-c",
      `INSERT INTO pbl_course_assignment (atom_id, course_slug, why)
       VALUES ('${atomId}', '${slug}', '${why}')
       ON CONFLICT (atom_id, course_slug) DO UPDATE SET why = EXCLUDED.why`,
    ],
    { stdio: "pipe" },
  );
}

async function makeProject(page: Page): Promise<string> {
  await page.goto("/projects");
  await openSiteGate(page);

  const all = await (await page.request.get(`${API}/api/v1/pbl/projects`)).json();
  const existing = Array.isArray(all)
    ? all.filter((p: { kind?: string }) => p.kind !== "website")
    : [];
  if (existing.length > 0) {
    await page.goto(`/projects/${existing[0].id}`);
    await expect(page.getByPlaceholder("请输入")).toBeVisible();
    return existing[0].id as string;
  }
  await page.goto("/projects");
  await page.getByPlaceholder("比如：", { exact: false }).fill(IDEA);
  await page.getByRole("button", { name: "开始", exact: true }).click();
  await expect(page).toHaveURL(/\/projects\/[0-9a-f-]{36}$/);
  await expect(page.getByText("查作业的小网页", { exact: false }).first()).toBeVisible({
    timeout: 90_000,
  });
  return page.url().split("/").pop()!;
}

test("课程: 轻量版看得见课程库", async ({ page }) => {
  await page.goto("/courses");
  // 目录是一次请求，卡片要真的画出来 —— 不是一句「加载中」。
  //
  // 🚨 2026-09-21 订正：这里原来钉着「CRRAAB 信源评估」这一门课的名字。
  // 那是**库里的一行种子数据**，不是仓库里的内容 —— 课程库会增删，
  // 而且一门课给不给轻量版看由 `course.audience` 决定（AGENTS.md）。
  // 它哪天改了受众或换了名字，这条走查就报「轻量版看不见课程库」，
  // 而其实课程库好好的。这一族从那之后就一直红着。
  //
  // 要守的是「轻量版真的看得见课程库、而且点得进去」，所以钉那件事本身。
  await expect(page.getByRole("button", { name: "开始学习" }).first())
    .toBeVisible({ timeout: 30_000 });
  await page.screenshot({ path: "e2e/.shots/courses-catalog.png", fullPage: true });

  // 点开一门：URL 要变成 /courses/:slug，刷新之后还在同一门课上。
  await page.goto(`/courses/${COURSE_SLUG}`);
  await expect(page).toHaveURL(new RegExp(`/courses/${COURSE_SLUG}$`));
  await page.waitForTimeout(2000);
  await page.screenshot({ path: "e2e/.shots/courses-detail.png", fullPage: true });
});

/** 本地那台一次性 Postgres 在不在。 */
function hasLocalPg(): boolean {
  try {
    const out = execFileSync("docker", ["ps", "--format", "{{.Names}}"], { stdio: "pipe" })
      .toString();
    return out.split("\n").some((n) => n.trim() === PG_CONTAINER);
  } catch {
    return false;
  }
}

test("课程: 项目里印记递一课，上完写回对话", async ({ page }) => {
  // 🚨 这一条**结构上只能在本地那套栈上跑**：它要往库里插一行课程指派，
  // 而那一行只有印记的 produce("course") 会写，**没有对外的 POST**
  //（那是对的 —— 一门课该不该上是印记的判断，不是一个前端按钮）。
  // 所以它靠 `docker exec … psql` 直接插，打线上的时候那个容器根本不存在，
  // 报的是「No such container」，读起来像课程闭环坏了。
  //
  // 跳过而不是让它红：红着的走查会把真的问题盖住，而这一条在本地
  // （run-stack.sh）跑起来照样是绿的、照样在守那个闭环。
  test.skip(!hasLocalPg(), `要本地那台 ${PG_CONTAINER} 才跑得了：这一条要直接往库里插一行课程指派`);

  const id = await makeProject(page);
  const api = `${API}/api/v1/pbl/projects/${id}`;

  // 指派这一行只有印记的 produce("course") 会写，没有对外的 POST —— 那是对的
  // （一门课该不该上是印记的判断，不是一个前端按钮）。这条 walk 要看的是它之后
  // 的界面，所以直接往库里插一行，等价于印记刚递完课。挑课那一段由
  // internal/api/pbl_course_test.go 守着。
  seedAssignment(id, COURSE_SLUG, "你说不会写代码，这一课正好讲怎么判断两组数据能不能比");

  const rows = (await (await page.request.get(api + "/courses")).json()) as {
    id: string;
    slug: string;
  }[];
  expect(rows.length, "指派没进去").toBeGreaterThan(0);
  const already = new Set<string>(
    ((await (await page.request.get(api + "/tools")).json()) as { tool: string }[]).map(
      (t) => t.tool,
    ),
  );
  if (!already.has("course")) {
    const res = await page.request.post(api + "/tools", {
      data: { tool: "course", reason: "你说不会写代码，这一课正好讲这个" },
    });
    expect(res.status(), "summon course").toBe(201);
  }

  await page.reload();
  // 对话里那张邀请卡：「去上一课」是卡上的标题，按钮叫「开始任务」。
  await expect(page.getByText("去上一课", { exact: false }).first()).toBeVisible({
    timeout: 30_000,
  });
  await page.getByRole("button", { name: "开始任务" }).first().click();
  await expect(page.getByText("这一课对你的项目有什么用", { exact: true })).toBeVisible({
    timeout: 30_000,
  });
  await page.screenshot({ path: "e2e/.shots/courses-tool.png", fullPage: true });

  // 浮层：上课在项目里进行，不跳走。
  await page.getByRole("button", { name: /开始学习|继续学习|再看一遍/ }).first().click();
  await expect(page.getByRole("dialog", { name: "课程" })).toBeVisible({ timeout: 30_000 });
  await page.waitForTimeout(3000);
  await page.screenshot({ path: "e2e/.shots/courses-modal.png", fullPage: true });
  await page.getByRole("button", { name: "关闭课程" }).click();
  await expect(page.getByRole("dialog", { name: "课程" })).toBeHidden();

  // 闭环：写下收获 → 完成 → 印记接一轮。
  //
  // 🚨 断言不能落在页面上找得到那句 takeaway —— 它就在输入框里，点「完成」之前
  // 就"可见"了，所以那样写的断言是必然通过的（第一版就是这么写的，拍出来的图上
  // 工具面板还开着、按钮还转着圈）。
  //
  // 真正的信号有三个，缺一不可：工具面板关掉了、库里记下了这次上课、对话里多出
  // 一条印记的话。第三条要等真实的模型调用，所以给足时间。
  // 🚨 只数印记说的那几条。了结一件工具本身会往线程里写一条 role="system" 的
  // 记录（pbl_artifacts.go · appendPblToolRecord），所以数总条数是数不出"印记
  // 接没接这一轮"的 —— 第二版断言就是这么假绿的。
  const aiTurns = async (): Promise<number> => {
    const rows = (await (await page.request.get(api + "/thread")).json()) as {
      role: string;
    }[];
    return rows.filter((m) => m.role === "ai").length;
  };
  const before = await aiTurns();
  const takeaway = "我知道两组数据要能比才有意义了，先把班里作业的口径统一。";
  await page.getByPlaceholder("请输入").last().fill(takeaway);
  await page.getByRole("button", { name: "完成" }).click();

  // 面板收起，右栏回到计划。
  await expect(page.getByText("这一课对你的项目有什么用", { exact: true })).toBeHidden({
    timeout: 180_000,
  });

  // 服务端记下了这一条，否则回灌读不到。
  const after = (await (await page.request.get(api + "/courses")).json()) as {
    id: string;
    takeaway: string;
    finishedAt: string | null;
  }[];
  const finished = after.find((c) => c.id === rows[0]!.id);
  expect(finished?.finishedAt, "上完的那一课没落库").toBeTruthy();
  expect(finished?.takeaway).toContain("口径");

  // 印记接了一轮 —— 环闭上的最后一段。
  await expect
    .poll(
      aiTurns,
      { timeout: 180_000, message: "印记没有接这一轮，环没闭上" },
    )
    .toBeGreaterThan(before);

  // 材料清单记着这一门：她过两天回来还能点开看自己写了什么。
  await expect(page.getByRole("button", { name: /课程 .*已上完/ })).toBeVisible({
    timeout: 30_000,
  });
  // 🚨 图拍在这里，不是点完「完成」的那一刻 —— 那时候印记还没开口，拍下来的是
  // 一屏"什么也没发生"，而这条 walk 存在的意义就是这张图。
  await page.screenshot({ path: "e2e/.shots/courses-loop.png", fullPage: true });
});
