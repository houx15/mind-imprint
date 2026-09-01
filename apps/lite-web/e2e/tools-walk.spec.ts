import { expect, test, type Page } from "@playwright/test";

/**
 * 工具 walk —— 七个阶段的七块界面，一块一块打开看。
 *
 * 这个文件存在的意义是**它拍下来的图**，不是断言。2026-08-30 的教训是 344 个
 * 绿测试压在一张全白的导出图上；所以看 e2e/.shots/tools-*.png，别只看这里绿了
 * 没有。
 *
 * 没有模型 key，所以印记不会自己递工具。这里用**真的那个端点**把工具递出来
 * （POST /tools 就是印记递工具走的同一条路），然后走真的界面。绕开的只有模型
 * 那一步。
 */

const TOOLS: { tool: string; reason: string }[] = [
  { tool: "board", reason: "你刚一口气说了三件不太一样的事，先摊开看看" },
  { tool: "reframe", reason: "「大家」是谁？先落到一个你真的见过的人身上" },
  { tool: "ideas", reason: "现在只有一个办法，太早了" },
  { tool: "decide", reason: "这两条路走下去很不一样，值得停一下" },
  { tool: "structure", reason: "动手写之前，先看看整体分成几块" },
  { tool: "split", reason: "这一步有几格其实该你自己做" },
  { tool: "lookback", reason: "这个项目走完了，回头看一遍" },
  { tool: "keep", reason: "网站放出去两周了，该看看发生了什么" },
  { tool: "observe", reason: "你说的都是猜的，先去看三天中午" },
];

async function makeProject(page: Page): Promise<string> {
  await page.goto("/projects");
  await page.getByPlaceholder("比如：", { exact: false }).fill(
    "我们学校每天剩好多饭，我想弄明白这些饭最后去哪了，能不能少一点。",
  );
  await page.getByRole("button", { name: "开始" }).click();
  await expect(page.getByRole("heading", { name: "给它起个名字" })).toBeVisible();
  await page.getByPlaceholder("你想叫它什么").fill("剩饭去哪了");
  await page.getByRole("button", { name: "就这样" }).click();
  await expect(page).toHaveURL(/\/projects\/[0-9a-f-]{36}$/);
  const id = page.url().split("/").pop()!;
  // 🚨 等项目自己的名字出现，别等输入框——输入框在数据到达之前就画出来了。
  await expect(page.locator("header").getByText("剩饭去哪了")).toBeVisible();
  return id;
}

test("工具: 七个阶段的界面各打开一次", async ({ page }) => {
  const id = await makeProject(page);
  const api = `/api/v1/pbl/projects/${id}`;

  // 印记 递工具走的就是这条路。
  for (const t of TOOLS) {
    const res = await page.request.post(api + "/tools", { data: t });
    expect(res.status(), `summon ${t.tool}`).toBe(201);
  }

  // 审阅要有东西可审；分工要有一份计划。
  const artifact = await page.request.post(api + "/artifacts", {
    data: {
      kind: "draft",
      title: "给食堂的一页建议",
      payload: {
        body:
          "中午十二点半，第三个泔水桶已经装满了。学校每天剩下的饭菜里，米饭占了大概一半。\n\n" +
          "如果打饭的时候能让同学自己选饭量，剩下的应该会少很多。别的学校试过这个办法。",
      },
      guessed: ["我猜剩的主要是米饭"],
      admits: ["没算过每天到底剩多少斤", "「别的学校试过」我没查到具体是哪一所"],
    },
  });
  expect(artifact.status()).toBe(201);
  const aid = (await artifact.json()).id as string;
  expect(
    (
      await page.request.post(`${api}/artifacts/${aid}/review`, {
        data: {
          marks: [
            {
              part: "论证",
              partNote: "这一段要看证据撑不撑得住",
              quote: "米饭占了大概一半",
              question: "「大概一半」是你数出来的，还是估的？",
            },
            {
              quote: "别的学校试过这个办法",
              question: "哪一所？你能查到吗？",
            },
          ],
          dimensions: [
            { prompt: "每个数字都说得出哪来的吗", why: "查不到出处的数字等于没有" },
            { prompt: "有没有把你的话改成它自己的说法", why: "这份要交出去的是你的判断" },
          ],
        },
      })
    ).status(),
  ).toBe(200);

  const plan = await page.request.post(api + "/plan", {
    data: {
      summary: "先弄清楚剩的到底是什么",
      reason: "她批准的",
      steps: [
        { title: "去食堂看三天", decide: "你判断剩得最多的是哪一类" },
        { title: "问三个同学", decide: "你决定问谁" },
      ],
    },
  });
  expect(plan.status()).toBe(201);
  const versionId = (await plan.json()).versionId as string;
  expect((await page.request.post(api + "/plan/approve", { data: { versionId } })).status()).toBe(200);

  const steps = (await (await page.request.get(api + "/plan")).json()).plan.steps as {
    id: string;
  }[];
  expect(
    (
      await page.request.post(`${api}/steps/${steps[0]!.id}/substeps`, {
        data: {
          substeps: [
            { title: "先说清楚要看什么", owner: "both", reason: "只有你知道你想看什么" },
            { title: "整理三天的数字", owner: "yinji", reason: "重复的活，我来快一些" },
            { title: "判断哪一类最多", owner: "student", reason: "这是这一步真正要你判断的" },
          ],
        },
      })
    ).status(),
  ).toBe(200);

  await page.reload();
  await expect(page.locator("header").getByText("剩饭去哪了")).toBeVisible();

  // 1 · 对话末尾的那一叠邀请：每一张都写着为什么是现在。
  await expect(page.getByText("你刚一口气说了三件不太一样的事", { exact: false })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-1-invites.png", fullPage: true });

  // 出门做的那件长得不一样：不弹面板，只把事情交给她。
  await expect(page.getByText("这件要离开屏幕做，回来再说。")).toBeVisible();

  // 2 · 便签板：贴几张，归一堆。
  await openTool(page, "便签板");
  await expect(page.getByText("把看到的、听到的、猜的、想问的都摊到板上")).toBeVisible();
  await page.getByPlaceholder("写一条，回车贴上去").fill("中午十二点半，第三个桶已经满了");
  await page.keyboard.press("Enter");
  await page.getByRole("button", { name: "别人说的" }).first().click();
  await page.getByPlaceholder("写一条，回车贴上去").fill("阿姨说「每天都这样」");
  await page.keyboard.press("Enter");
  await expect(page.getByText("阿姨说「每天都这样」")).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-2-board.png", fullPage: true });

  // 3 · 把问题说清楚：一次只问一句。
  await openTool(page, "把问题说清楚");
  await expect(page.getByText("这件事里，具体是谁？")).toBeVisible();
  await expect(page.getByText("落到一个具体的人", { exact: false })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-3-reframe.png", fullPage: true });

  // 4 · 想办法：先多想几个。
  await openTool(page, "想办法");
  await expect(page.getByText("先多想几个，别急着挑第一个。")).toBeVisible();
  for (const idea of ["让同学自己选饭量", "把剩饭称一称贴出来", "问阿姨能不能少做一点"]) {
    await page.getByPlaceholder("一个办法，回车记下").fill(idea);
    await page.keyboard.press("Enter");
  }
  await page.screenshot({ path: "e2e/.shots/tools-4-ideas.png", fullPage: true });

  // 5 · 审一遍：划出来的句子高亮在原文里。
  await openTool(page, "审一遍");
  await expect(page.getByText("我猜剩的主要是米饭", { exact: false })).toBeVisible();
  await expect(page.getByText("「大概一半」是你数出来的，还是估的？")).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-5-review.png", fullPage: true });

  // 6 · 做决定：一段一段解锁。
  await openTool(page, "做决定");
  await expect(page.getByText("你在定什么？")).toBeVisible();
  await page.getByPlaceholder("比如：这个建议先给食堂还是先发在班群里").fill(
    "这份建议先给食堂，还是先发在班群里",
  );
  await page.getByRole("button", { name: "开始" }).click();
  await expect(page.getByText("有哪些选择")).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-6-decide.png", fullPage: true });

  // 7 · 先看结构：缩进就是层级。
  await openTool(page, "先看结构");
  for (const block of ["我看到了什么", "这说明什么", "我建议怎么做"]) {
    await page.getByPlaceholder("再分一块出来").fill(block);
    await page.keyboard.press("Enter");
  }
  await expect(page.getByText("想说的都在里面了吗？")).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-7-structure.png", fullPage: true });

  // 8 · 分工：印记领了几格，在她还能改的时候说出来。
  await openTool(page, "分工");
  await expect(page.getByText("3 格里，印记领了 1 格。")).toBeVisible();
  await expect(page.getByText("重复的活，我来快一些")).toBeVisible();
  await page.getByRole("button", { name: "改成你做" }).first().click();
  await expect(page.getByText("改成「你做」，为什么？")).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-8-split.png", fullPage: true });

  // 9 · 复盘：问题从真的发生过的事里长出来。
  await openTool(page, "复盘");
  await expect(page.getByText("这些问题是从你这个项目里发生过的事写出来的。")).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-9-lookback.png", fullPage: true });

  // 10 · 上线之后：彩色的四步圈。
  await openTool(page, "上线之后");
  await expect(page.getByRole("button", { name: "读出意思" })).toBeVisible();
  await page.getByPlaceholder("比如：这周有 12 个人打开过").fill("这周有 12 个人打开过");
  await page.keyboard.press("Enter");
  await expect(page.getByRole("button", { name: "想一想这条" })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-10-keep.png", fullPage: true });
});

test("工具: 深色，和手机上", async ({ page }) => {
  const id = await makeProject(page);
  await page.request.post(`/api/v1/pbl/projects/${id}/tools`, {
    data: { tool: "board", reason: "你刚一口气说了三件不太一样的事，先摊开看看" },
  });

  // 深色。BackgroundProvider 把这些变量写成行内样式，所以主题必须在首屏之前
  // 生效，而且 --mk-paper 那几个要 !important——不然就是浅底浅字。
  await page.emulateMedia({ colorScheme: "dark" });
  await page.reload();
  await expect(page.locator("header").getByText("剩饭去哪了")).toBeVisible();
  await openTool(page, "便签板");
  await page.getByPlaceholder("写一条，回车贴上去").fill("中午十二点半，第三个桶已经满了");
  await page.keyboard.press("Enter");
  await page.screenshot({ path: "e2e/.shots/tools-dark-board.png", fullPage: true });

  // 手机：右边这一栏在 lg 以下是收起来的，所以这里看的是对话和那叠邀请卡。
  await page.emulateMedia({ colorScheme: "light" });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.reload();
  await expect(page.getByText("你刚一口气说了三件不太一样的事", { exact: false })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-phone-invite.png", fullPage: true });
});

/** 打开一件工具：点邀请卡上的「打开」，再切到它的标签页。 */
async function openTool(page: Page, label: string) {
  const invite = page.locator("div").filter({ hasText: new RegExp(`^${label}`) });
  const openButton = invite.getByRole("button", { name: "打开" }).first();
  if (await openButton.isVisible().catch(() => false)) {
    await openButton.click();
  } else {
    await page.getByRole("button", { name: label, exact: true }).first().click();
  }
  await expect(page.getByRole("heading", { name: label })).toBeVisible();
}
