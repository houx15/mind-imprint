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

// 🚨 重跑帮不上忙：这条 walk 的第一步就是"手边有几张还没打开的邀请卡"，而第一
// 次跑完它们就都打开了。所以关掉重试，让失败报出真正的原因。
test.describe.configure({ retries: 0 });

const TOOLS: { tool: string; reason: string }[] = [
  { tool: "board", reason: "你刚一口气说了三件不太一样的事，先摊开看看" },
  { tool: "reframe", reason: "「大家」是谁？先落到一个你真的见过的人身上" },
  { tool: "ideas", reason: "现在只有一个办法，太早了" },
  { tool: "review", reason: "我写了一版，你先看看哪里不对" },
  { tool: "decide", reason: "这两条路走下去很不一样，值得停一下" },
  { tool: "structure", reason: "动手写之前，先看看整体分成几块" },
  { tool: "split", reason: "这一步有几格其实该你自己做" },
  { tool: "lookback", reason: "这个项目走完了，回头看一遍" },
  { tool: "keep", reason: "网站放出去两周了，该看看发生了什么" },
  { tool: "observe", reason: "你说的都是猜的，先去看三天中午" },
];

const IDEA = "我们学校每天剩好多饭，我想弄明白这些饭最后去哪了，能不能少一点。";

async function makeProject(page: Page): Promise<string> {
  // 重跑时复用已经建好的那个：这条 walk 只需要一个项目。
  const existing = await (await page.request.get("/api/v1/pbl/projects")).json();
  if (Array.isArray(existing) && existing.length > 0) {
    await page.goto(`/projects/${existing[0].id}`);
    await expect(page.getByPlaceholder("请输入")).toBeVisible();
    return existing[0].id as string;
  }
  await page.goto("/projects");
  await page.getByPlaceholder("比如：", { exact: false }).fill(IDEA);
  // 不再弹命名窗（产品负责人 2026-09-02）：写完那句话直接进房间，而那句话
  // 就是她对印记说的第一句。
  await page.getByRole("button", { name: "开始" }).click();
  await expect(page).toHaveURL(/\/projects\/[0-9a-f-]{36}$/);
  const id = page.url().split("/").pop()!;
  // 🚨 等她那句话真的出现在对话里——这是"第一轮跑通了"唯一诚实的信号。
  // 这一轮要真的调模型，所以给足时间。
  await expect(page.getByText("我想弄明白这些饭最后去哪了", { exact: false }).first()).toBeVisible({
    timeout: 90_000,
  });
  return id;
}

test("工具: 七个阶段的界面各打开一次", async ({ page }) => {
  const id = await makeProject(page);
  const api = `/api/v1/pbl/projects/${id}`;

  // 印记 递工具走的就是这条路。重跑时不重复递——同一件工具递两次，界面上就
  // 真的会出现两张邀请卡，那是对的行为，只是会把这条 walk 的定位搞乱。
  const already = new Set<string>(
    ((await (await page.request.get(api + "/tools")).json()) as { tool: string }[]).map(
      (t) => t.tool,
    ),
  );
  for (const t of TOOLS) {
    if (already.has(t.tool)) continue;
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
  await expect(page.getByPlaceholder("请输入")).toBeVisible();

  // 1 · 对话末尾的那一叠邀请：每一张都写着为什么是现在。
  await expect(page.getByText("你刚一口气说了三件不太一样的事", { exact: false })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-1-invites.png", fullPage: true });

  // 出门做的那件长得不一样：不弹面板，只把事情交给她。
  await expect(page.getByText("请在合适的地方完成这项任务").first()).toBeVisible();

  // 2 · 用真界面打开一件：点邀请卡上的「打开」。
  await page.getByTestId("tool-invite-board").first().getByRole("button", { name: "开始任务" }).click();
  await expect(page.getByRole("heading", { name: "头脑风暴" })).toBeVisible();

  // 其余几件通过端点接受——这条 walk 要看的是八块界面，不是同一个点击重复八遍。
  const open = await (await page.request.get(api + "/tools")).json();
  for (const t of open) {
    if (t.status === "summoned" && t.kind === "thinking") {
      expect((await page.request.post(`${api}/tools/${t.id}/accept`)).status()).toBe(200);
    }
  }
  await page.reload();
  await expect(page.getByRole("heading", { name: "计划" })).toBeVisible();
  await openTool(page, "头脑风暴");
  await expect(page.getByText("再把有关系的挪到一起", { exact: false })).toBeVisible();
  await page.getByPlaceholder("写一条，回车贴上去").fill("中午十二点半，第三个桶已经满了");
  await page.keyboard.press("Enter");
  await page.getByRole("button", { name: "实际观察" }).first().click();
  await page.getByPlaceholder("写一条，回车贴上去").fill("阿姨说「每天都这样」");
  await page.keyboard.press("Enter");
  await page.getByPlaceholder("写一条，回车贴上去").fill("我猜是米饭剩得最多");
  await page.keyboard.press("Enter");
  await expect(page.getByText("阿姨说「每天都这样」")).toBeVisible();
  // 板上摆得动：点两张，归成一堆。
  await page.getByText("阿姨说「每天都这样」").click();
  await page.getByText("中午十二点半，第三个桶已经满了").click();
  await expect(page.getByText("选了 2 张")).toBeVisible();
  await page.getByRole("button", { name: "归成一堆" }).click();
  await page.getByPlaceholder("这几张是一回事，因为……").fill("打饭那一会儿");
  await page.getByRole("button", { name: "确认", exact: true }).click();
  await expect(page.getByText("已经归了 1 堆", { exact: false })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-2-board.png", fullPage: true });

  // 3 · 问题识别：一次只问一句。
  await openTool(page, "问题识别");
  await expect(page.getByText("这件事里，具体是谁？")).toBeVisible();
  await expect(page.getByText("落到一个具体的人", { exact: false })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-3-reframe.png", fullPage: true });

  // 4 · 解决方案：先多想几个。
  await openTool(page, "解决方案");
  await expect(page.getByText("先多想几个，别急着挑第一个。")).toBeVisible();
  for (const idea of ["让同学自己选饭量", "把剩饭称一称贴出来", "问阿姨能不能少做一点"]) {
    await page.getByPlaceholder("一个办法，回车记下").fill(idea);
    await page.keyboard.press("Enter");
  }
  await page.screenshot({ path: "e2e/.shots/tools-4-ideas.png", fullPage: true });

  // 5 · 审核助手：划出来的句子高亮在原文里。
  await openTool(page, "审核助手");
  await expect(page.getByText("我猜剩的主要是米饭", { exact: false })).toBeVisible();
  await expect(page.getByText("「大概一半」是你数出来的，还是估的？")).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-5-review.png", fullPage: true });

  // 6 · 理性决策：印记摆出几条路，她选一条，再答两个小问题。
  const decisionRes = await page.request.post(api + "/decisions", {
    data: {
      subject: "这份建议先给食堂，还是先发在班群里",
      options: [
        { label: "先给食堂", description: "他们能直接改菜量，但要等排期" },
        { label: "先发班群", description: "当天就有反馈，但改不了任何事" },
      ],
    },
  });
  expect(decisionRes.status()).toBe(201);
  const decisionId = (await decisionRes.json()).id as string;
  await openTool(page, "理性决策");
  await expect(page.getByText("他们能直接改菜量，但要等排期")).toBeVisible();
  await page.getByRole("button", { name: /^先给食堂/ }).click();
  await expect(page.getByText("为什么不选别的")).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-6-decide.png", fullPage: true });

  // 7 · 结构审查：一张能拖的图。
  await openTool(page, "结构审查");
  for (const block of ["我看到了什么", "这说明什么", "我建议怎么做"]) {
    await page.getByPlaceholder("加一块").fill(block);
    await page.keyboard.press("Enter");
  }
  await expect(page.getByText("拖到另一块上面就挂到它下面", { exact: false })).toBeVisible();
  await expect(page.getByText("这个框架是否覆盖了所有应当呈现的内容？")).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-7-structure.png", fullPage: true });

  // 8 · 分工：印记领了几格，在她还能改的时候说出来。
  await openTool(page, "分工建议");
  await expect(page.getByText("共 3 项，其中 1 项由印记完成。")).toBeVisible();
  await expect(page.getByText("重复的活，我来快一些")).toBeVisible();
  // 名字标签：两个人一起做的那一格挂两个。
  await expect(page.getByRole("cell").filter({ hasText: "印记" }).first()).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-8-split.png", fullPage: true });

  // 复盘之前，先让项目里真的发生两件事：一个定下来的决定，和一份被退回去的
  // 东西——复盘的问题是印记按这些事写出来的。
  // （放在审阅之后：成果一旦定了，就不在待审的那一摞里了。）
  expect(
    (
      await page.request.post(`${api}/artifacts/${aid}/settle`, {
        data: { verdict: "revise", why: "第二段把我的话改成了它自己的说法" },
      })
    ).status(),
  ).toBe(200);
  expect(
    (
      await page.request.post(`${api}/decisions/${decisionId}/settle`, {
        data: {
          choice: "先给食堂",
          why: "只有他们能真的把菜量改了",
          whyNot: "班群反馈快，但同学说了也改不了食堂的量",
        },
      })
    ).status(),
  ).toBe(200);

  // 9 · 复盘：六段骨架，段里的问题由印记按真发生过的事现写。
  //     问题的内容来自真实模型，不可预测，所以这里只确认骨架立起来了。
  await openTool(page, "项目复盘");
  // 🚨 这一屏要真的调一次模型。它可能被限流——那时候正确的行为是把后台原话
  // 显示出来，而不是编一份通用问卷。所以这里断言的是"两种诚实状态之一"：
  // 六段立起来了，或者红字说清了为什么没有。挂在外部服务上的断言不该让整条
  // walk 变成看运气。
  // 🚨 只在工具面板里找。第一版没限定范围，结果 or 分支匹配到了左边聊天区
  // 一条无关的红字，测试绿了，而面板其实还在转圈——一个断言范围没收住，整条
  // walk 就在替我说谎。
  const panel = page.getByRole("complementary");
  await expect(
    panel.getByText("做了什么").or(panel.getByText("后台错误：", { exact: false })),
    // 旗舰模型要把整个项目读一遍再写六段问题，一分多钟是常态；120 秒不够，
    // 上一轮就是卡在这个数上，看着像失败其实只是还没回来。
  ).toBeVisible({ timeout: 240_000 });
  await page.screenshot({ path: "e2e/.shots/tools-9-lookback.png", fullPage: true });

  // 10 · 长期迭代：彩色的四步圈。
  await openTool(page, "长期迭代");
  // 先讲清为什么值得做，再要数据。
  await expect(page.getByText("东西做出来只是开始", { exact: false })).toBeVisible();
  await expect(page.getByRole("button", { name: "数据分析" })).toBeVisible();
  // 常见指标点开能看见它是什么。
  await page.getByRole("button", { name: "留存率" }).click();
  await expect(page.getByText("上次来过的人，这次还回来的比例。")).toBeVisible();
  await page.getByPlaceholder("这周的一个数字，以及它是从哪看到的").fill("这周有 12 个人打开过");
  await page.keyboard.press("Enter");
  await expect(page.getByRole("button", { name: "深入讨论" })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-10-keep.png", fullPage: true });

  // 11 · 深色。
  //
  // 🚨 深色是**主动选的**，不跟系统走：CSS 里只有 :root[data-theme="dark"]，
  // 没有 prefers-color-scheme。所以这里存偏好，和以后那个开关做的事一样；
  // 用 emulateMedia 拍出来的会是一张浅色图，而且测试照样绿。
  await page.evaluate(() => localStorage.setItem("mk-theme", "dark"));
  await page.reload();
  // 主题要在首屏之前生效（main.tsx 的 bootTheme），不然卡片会先画一遍浅色。
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.getByPlaceholder("请输入")).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-11-dark.png", fullPage: true });

  // 12 · 手机。右边这一栏在 lg 以下收起来，所以这里看的是对话和那叠邀请卡。
  await page.evaluate(() => localStorage.removeItem("mk-theme"));
  await page.setViewportSize({ width: 390, height: 844 });
  await page.reload();
  await expect(page.getByText("你说的都是猜的，先去看三天中午")).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/tools-12-phone.png", fullPage: true });
});

/**
 * 切到这件工具的标签页。
 *
 * 「打开」那个动作在第 1 步用真界面走过一次了；剩下八件在那之前已经通过端点
 * 接受，因为这条 walk 要看的是**八块界面长什么样**，不是把同一个点击重复八遍。
 */
async function openTool(page: Page, label: string) {
  await page.getByRole("button", { name: label, exact: true }).first().click();
  await expect(page.getByRole("heading", { name: label })).toBeVisible();
}
