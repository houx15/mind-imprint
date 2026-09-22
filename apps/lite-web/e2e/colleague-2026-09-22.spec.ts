import { test, expect } from "@playwright/test";
import { freshAccount } from "./freshAccount";

/**
 * 同事 2026-09-22 那七条，线上逐条验一遍。
 *
 * 跑法：
 *   npx playwright test -c e2e/online.config.ts colleague-2026-09-22
 *
 * 🚨 为什么线上还要再验一遍：本地绿不等于线上对。这一族的三条链子各自会断 ——
 * 题库那份 JSON 要经过 go:embed（编译产物没上线的话本地干净、线上是旧的）；
 * 结构那几张卡要经过 HTTP 和前端的 normalizer；题目那一栏要真的在**构思**那一屏
 * 上渲染出来，而它和段落那两页走的是不同的分支（各自 early-return）。
 *
 * 意见 2 那一半（开场问哪个问题、判断不许摆成论据）归 Go 的 LIVE_LLM 测试
 *（`writing_classify_live_test.go`）—— 那是提示词的行为，要的是重复跑几次看
 * 稳不稳，不该在一条线上走查里判一次就下结论。
 */

const API = (process.env.E2E_API_BASE ?? "https://mind-api.uni-robot.cn").replace(/\/+$/, "");

/** 库里仍然带着「任选」的那三道 —— 它们是**对的**：那是题目自己的话。 */
const KEEPS_RENXUAN = new Set(["ZKYW-029", "ZKYW-032", "ZKYW-043"]);

test("意见 6：题库里「任选一题」不再说谎，多题的拆开了", async ({ browser }) => {
  test.setTimeout(240_000);
  const ctx = await freshAccount(browser, "r5-prompts");

  const all = await (await ctx.request.get(`${API}/api/v1/writing-prompts`)).json();
  console.log(`整库 ${all.total} 道`);
  // 705 → 728：拆开了 18 道多选题。编译产物没上线的话这里还是 705。
  expect(all.total, "库还是旧的 —— 拆开的那份 JSON 没上线").toBeGreaterThanOrEqual(728);

  // 拆出来的那几道：id 带后缀，而且题面里不该再留着标号。
  const split = await (await ctx.request.get(`${API}/api/v1/writing-prompts?q=${encodeURIComponent("语言的滋味")}`)).json();
  expect(split.total, "找不到拆出来的那道「语言的滋味」").toBeGreaterThan(0);
  const one = split.items[0];
  console.log(`拆出来的一道：${one.id} —— ${one.text.slice(0, 40)}…`);
  expect(one.id, "id 上没有拆出来的后缀").toMatch(/-\d$/);
  expect(one.text, "题面还顶着「任选一题」").not.toContain("任选一题");
  expect(one.text, "题面还顶着「题目一」那个标号").not.toMatch(/^\s*题目\s*[一二三]/);
  // 另一个题目不该混在同一张卡上。
  expect(one.text, "另一道题混在同一张卡上").not.toContain("攀登是幸福的");

  // 整库扫一遍：说谎的引子和卷面管理那几句都不该有了。
  let checked = 0;
  for (let page = 1; ; page++) {
    const res = await (await ctx.request.get(`${API}/api/v1/writing-prompts?pageSize=100&page=${page}`)).json();
    if (res.items.length === 0) break;
    for (const it of res.items) {
      checked++;
      const baseId = String(it.id).replace(/-\d$/, "");
      if (!KEEPS_RENXUAN.has(baseId)) {
        expect(it.text, `${it.id} 还留着「任选一题」`).not.toMatch(/任选一[题个篇道]|选做一题/);
      }
      expect(it.text, `${it.id} 还留着答题卡那句`).not.toContain("答题卡");
      expect(it.text, `${it.id} 还留着「不透露所在区」`).not.toMatch(/[不勿][得要]?(在文中)?(泄露|透露)/);
    }
    if (page > 12) break;
  }
  console.log(`扫过 ${checked} 道：引子、答题卡、别写真名，都没有了`);

  await ctx.close();
});

test("意见 7：英文那一篇拿到的是英文写作课的结构，不是语文课的四张卡", async ({ browser }) => {
  test.setTimeout(240_000);
  const ctx = await freshAccount(browser, "r5-english");

  // 开一篇英文议论文。
  const created = await ctx.request.post(`${API}/api/v1/writings`, {
    data: { idea: "Should schools start later?", lang: "en" },
  });
  expect(created.ok(), `创建写作失败：${created.status()}`).toBeTruthy();
  const id = (await created.json()).id as string;
  await ctx.request.put(`${API}/api/v1/writings/${id}/setup`, {
    data: { lang: "en", targetWords: 500, note: "" },
  });
  await ctx.request.put(`${API}/api/v1/writings/${id}/outline`, {
    data: {
      outline: [
        { text: "Schools should start an hour later", kind: "thesis", depth: 0 },
        { text: "Students do not get enough sleep", kind: "point", depth: 1 },
      ],
    },
  });

  const body = (await (await ctx.request.get(`${API}/api/v1/writings/${id}/flow/structures`)).json()) as {
    structures: { id: string; name: string; definition: string; example: string }[];
  };
  const names = body.structures.map((s) => s.name);
  console.log("英文议论文的结构：", names.join(" / "));

  // 🚨 语文课那四条一个都不该在。
  for (const zh of ["总分式", "并列式", "层进式", "对照式"]) {
    expect(names, `英文议论文里还摆着语文课的「${zh}」—— 那正是意见 7 说的那件事`).not.toContain(zh);
  }
  // 英文议论文真正被打分的那两件事要在。
  for (const want of ["Thesis-body-conclusion", "Claim-counterargument-refutation"]) {
    expect(names, `英文议论文缺 ${want}`).toContain(want);
  }
  // 每一张卡都要带一句借来的示范 —— 光给定义，两个名字在她眼里是同一句话。
  for (const s of body.structures) {
    expect(s.example, `${s.name} 没有示范`).not.toBe("");
  }

  // 对照：同一条路上，中文那一篇拿到的还是语文课那四条。
  const zhCreated = await ctx.request.post(`${API}/api/v1/writings`, {
    data: { idea: "学校应不应该允许学生带手机", lang: "zh" },
  });
  const zhId = (await zhCreated.json()).id as string;
  await ctx.request.put(`${API}/api/v1/writings/${zhId}/setup`, {
    data: { lang: "zh", targetWords: 800, note: "" },
  });
  await ctx.request.put(`${API}/api/v1/writings/${zhId}/outline`, {
    data: {
      outline: [
        { text: "学校应该允许学生带手机", kind: "thesis", depth: 0 },
        { text: "放学能联系家长", kind: "point", depth: 1 },
      ],
    },
  });
  const zhBody = (await (await ctx.request.get(`${API}/api/v1/writings/${zhId}/flow/structures`)).json()) as {
    structures: { name: string }[];
  };
  expect(zhBody.structures.map((s) => s.name).sort()).toEqual(
    ["并列式", "层进式", "总分式", "对照式"].sort(),
  );
  console.log("中文议论文的结构照旧：", zhBody.structures.map((s) => s.name).join(" / "));

  await ctx.close();
});

test("意见 1 + 3 + 4 + 5：构思看得见整道题，标题改得动，行文的顺序拖得动", async ({ browser }) => {
  test.setTimeout(300_000);
  const ctx = await freshAccount(browser, "r5-room");

  // 从题库里挑一道**题面长**的：那正是两行小字装不下的那种。
  const found = await (
    await ctx.request.get(`${API}/api/v1/writing-prompts?lang=zh&pageSize=100`)
  ).json();
  const long = (found.items as { id: string; text: string }[])
    .slice()
    .sort((a, b) => b.text.length - a.text.length)[0]!;
  console.log(`挑的这道题 ${long.id}，题面 ${long.text.length} 字`);
  expect(long.text.length, "库里最长的题面还不到 100 字 —— 挑错了").toBeGreaterThan(100);

  const started = await ctx.request.post(
    `${API}/api/v1/writing-prompts/${encodeURIComponent(long.id)}/start`,
  );
  expect(started.ok(), `开写作失败：${started.status()}`).toBeTruthy();
  const id = (await started.json()).id as string;

  // 摆一张图，照同事截图里那个形状 —— 包括被摆错的那一条。
  await ctx.request.put(`${API}/api/v1/writings/${id}/outline`, {
    data: {
      outline: [
        { text: "我想写「成功」这个词，我对它的理解发生了变化", kind: "thesis", depth: 0 },
        { text: "成功的定义太窄，成功的人就太少", kind: "point", depth: 1 },
        { text: "人人有自己的贡献，平凡尽责也是成功", kind: "point", depth: 1 },
        { text: "黑心商家哪怕赚很多钱，也是失败", kind: "reference", depth: 2 },
      ],
    },
  });

  const page = await ctx.newPage();
  await page.goto(`/writings/${id}`);

  // 🚨 先过「开始之前」那个弹窗。
  //
  // 从题库开的一篇，`setupAt` 是 null，于是 WritingRoomHost 在构思之前
  // early-return 那个弹窗 —— 这是产品负责人 2026-09-21 要的样子（语言、
  // 目标字数、作业要求都已经定好，不留输入框，一下就进去）。
  // 第一版走查没点它，于是报「构思那一屏上没有题目那一栏」，读起来像
  // 那一栏没做出来。**是走查自己的眼睛少了一步**
  //（[[observation-tool-is-the-bug-2026-09-12]]，这一轮第三次）。
  const startBtn = page.getByRole("button", { name: "开始", exact: true });
  await expect(startBtn, "「开始之前」那个弹窗没出现").toBeVisible({ timeout: 90_000 });
  // 弹窗里那几样必须是**定好的**，不该再问她一遍。
  await expect(page.getByText("目标字数")).toBeVisible();
  await expect(page.locator('[role="dialog"] textarea'), "弹窗里还留着输入框").toHaveCount(0);
  await startBtn.click();

  // ── 意见 1：构思那一屏上，整道题在左边那一栏里 ──────────────────
  const rail = page.getByRole("complementary", { name: "题目" });
  await expect(rail, "构思那一屏上没有题目那一栏").toBeVisible({ timeout: 90_000 });
  // 🚨 钉**题面的末尾**：截断的时候先没的就是它。
  const tail = long.text.trim().slice(-14);
  await expect(rail, `题面被截断了，末尾「${tail}」不在那一栏里`).toContainText(tail);
  console.log(`题目那一栏里有整道题，末尾是「${tail}」`);

  // 对话和图同时在屏幕上 —— 题目没有挤掉其中一个。
  await expect(page.getByText("写作构思")).toBeVisible();
  await expect(page.getByText("你的思路")).toBeVisible();

  // 折得起来，而且折起来之后还说得出自己是什么。
  const wide = (await rail.boundingBox())!.width;
  await page.getByTitle("折起题目").click();
  await expect(page.getByTitle("展开题目")).toBeVisible();
  expect((await rail.boundingBox())!.width, "折起来之后那一栏没有窄下去").toBeLessThan(wide);
  await page.getByTitle("展开题目").click();
  await expect(page.getByTitle("折起题目")).toBeVisible();

  // ── 意见 3：那条被摆成论据的判断，她自己改得动 ──────────────────
  const label = page.getByRole("button", { name: "论据 · 你找来的材料", exact: true }).last();
  await expect(label, "卡片上那个小标题不是个按钮 —— 她改不动").toBeVisible();
  await label.click();
  const menu = page.getByRole("menu");
  await expect(menu).toBeVisible();
  await expect(menu.getByRole("menuitem", { name: /反方观点/ }), "菜单里没有「反方观点」").toBeVisible();
  await menu.getByRole("menuitem", { name: /反方观点/ }).click();
  await expect(page.getByRole("button", { name: "反方观点", exact: true })).toBeVisible();

  // 🚨 落库了吗 —— 屏幕上变了而服务端没变，等于她下次进来又是错的。
  const after = (await (await ctx.request.get(`${API}/api/v1/writings/${id}/outline`)).json())
    .outline as { text: string; kind: string; depth: number }[];
  const changed = after.find((n) => n.text.includes("黑心商家"))!;
  expect(changed.kind, "改完没落库").toBe("counter");
  expect(changed.depth, "反方观点该在深度 1，和分论点并列").toBe(1);
  console.log("「黑心商家…」现在是 counter，深度 1 —— 和分论点并列");

  // ── 意见 4 + 5：行文那一步 ──────────────────────────────────────
  await ctx.request.post(`${API}/api/v1/writings/${id}/stage`, { data: { stage: "flow" } });
  await page.goto(`/writings/${id}`);
  await expect(page.getByRole("heading", { name: "行文" })).toBeVisible({ timeout: 90_000 });

  // 意见 5：那个下拉整块没了。
  await expect(
    page.locator('button[aria-haspopup="listbox"]'),
    "「每一条打算怎么证明」那个下拉还在",
  ).toHaveCount(0);
  await expect(page.getByText("每一条打算怎么证明")).toHaveCount(0);

  // 意见 4：顺序真的改得动。
  const up = page.getByLabel("把「人人有自己的贡献，平凡尽责也是成功」往前挪");
  await expect(up, "顺序那一栏里没有能挪的按钮").toBeVisible();
  await up.click();

  // 🚨 等那一下真的落下去，再去读服务端。
  //
  // 点一下触发的是一次异步 PUT；点完立刻读接口，读到的是**挪之前**那一份。
  // 第一版走查就是这样报的「往前挪那一下没生效」—— 整条测试只花了 2.4 秒，
  // 而那次写入还在路上。
  //
  // 等的信号是界面自己说的那句话：它排到第一个之后，「往前挪」就该变灰
  //（FlowOrderList 里 `disabled={i === 0}`）。这比 sleep 一个固定秒数可靠。
  await expect(up, "挪完之后它该排在第一个，往前挪该变灰").toBeDisabled();

  const reordered = (await (await ctx.request.get(`${API}/api/v1/writings/${id}/outline`)).json())
    .outline as { text: string; position: number }[];
  const order = reordered.slice().sort((a, b) => a.position - b.position).map((n) => n.text);
  console.log("挪完的顺序：", order.map((t) => t.slice(0, 10)).join(" → "));
  expect(
    order.findIndex((t) => t.includes("人人有自己的贡献")),
    "往前挪那一下没生效",
  ).toBeLessThan(order.findIndex((t) => t.includes("成功的定义太窄")));

  await ctx.close();
});
