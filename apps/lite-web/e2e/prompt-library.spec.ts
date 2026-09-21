import { test, expect } from "@playwright/test";
import { freshAccount } from "./freshAccount";

/**
 * 写作题库，线上走一遍 —— 705 道题、四维筛选、全文搜、页码条。
 *
 * 🚨 这一条守的是**单元测试守不到的那一半**：promptlib 的筛搜翻在 Go 里逐条
 * 验过了，但那份 JSON 要经过 go:embed、HTTP、前端的 normalizer 才到她眼前。
 * 中间任何一段漏了，本地全绿而线上是一页空白。
 *
 * 跑法：
 *   npx playwright test -c e2e/online.config.ts prompt-library
 */

const API = (process.env.E2E_API_BASE ?? "https://mind-api.uni-robot.cn").replace(/\/+$/, "");

test("题库：筛、搜、翻页，以及从一道题开一篇写作", async ({ browser }) => {
  test.setTimeout(240_000);
  const ctx = await freshAccount(browser, "prompt-lib");

  // ── 整库在不在 ────────────────────────────────────────────────────
  const all = await (await ctx.request.get(`${API}/api/v1/writing-prompts`)).json();
  console.log(`整库 ${all.total} 道，每页 ${all.pageSize}，共 ${all.pages} 页`);
  expect(all.total, "题库是空的 —— go:embed 那一步没把 JSON 带上线").toBeGreaterThan(700);
  expect(all.items.length).toBe(all.pageSize);
  expect(all.pages).toBeGreaterThan(1);

  // 四维筛选都得有值可选，否则那一行筛选器在她那边是空的。
  for (const [name, list] of [
    ["语言", all.facets.langs],
    ["考试", all.facets.categories],
    ["难度", all.facets.difficulties],
    ["话题", all.facets.topics],
  ] as const) {
    expect(list.length, `${name}那一维一个可选值都没有`).toBeGreaterThan(0);
    console.log(`  ${name}：${list.map((f: any) => `${f.label}(${f.count})`).join(" ")}`);
  }

  // ── 🚨 筛选项不许点进去为空 ──────────────────────────────────────
  // 这是这一族最容易错的地方：计数按整库算、结果按筛完算，两个数对不上，
  // 她点一个写着 100 的标签，进去是一页空白。
  const cat = all.facets.categories[0];
  const filtered = await (
    await ctx.request.get(`${API}/api/v1/writing-prompts?category=${encodeURIComponent(cat.value)}`)
  ).json();
  expect(filtered.total, `「${cat.value}」说有 ${cat.count} 道，点进去是 ${filtered.total} 道`).toBe(cat.count);
  for (const it of filtered.items) expect(it.category).toBe(cat.value);

  // 筛完之后每一维的计数也要跟着变。
  const zh = await (await ctx.request.get(`${API}/api/v1/writing-prompts?lang=zh`)).json();
  for (const f of zh.facets.topics.slice(0, 3)) {
    const sub = await (
      await ctx.request.get(`${API}/api/v1/writing-prompts?lang=zh&topic=${encodeURIComponent(f.value)}`)
    ).json();
    expect(sub.total, `中文 + 话题「${f.value}」：说 ${f.count}，实际 ${sub.total}`).toBe(f.count);
  }

  // ── 翻页：页与页之间不重不漏 ────────────────────────────────────
  const p1 = await (await ctx.request.get(`${API}/api/v1/writing-prompts?pageSize=10`)).json();
  const p2 = await (await ctx.request.get(`${API}/api/v1/writing-prompts?pageSize=10&page=2`)).json();
  const ids1 = p1.items.map((x: any) => x.id);
  const ids2 = p2.items.map((x: any) => x.id);
  expect(ids1.length).toBe(10);
  expect(ids2.length).toBe(10);
  expect(ids1.filter((id: string) => ids2.includes(id)), "第 1 页和第 2 页有重复").toEqual([]);

  // 翻过头给空页，不回卷 —— 回卷的样子是她按到底屏幕跳回开头。
  const far = await (await ctx.request.get(`${API}/api/v1/writing-prompts?page=99999`)).json();
  expect(far.items.length).toBe(0);
  expect(far.total).toBe(all.total);

  // ── 搜索 ────────────────────────────────────────────────────────
  const hit = await (await ctx.request.get(`${API}/api/v1/writing-prompts?q=${encodeURIComponent("环境")}`)).json();
  console.log(`搜「环境」：${hit.total} 道`);
  expect(hit.total).toBeGreaterThan(0);
  const miss = await (await ctx.request.get(`${API}/api/v1/writing-prompts?q=zzzq不存在的词`)).json();
  expect(miss.total).toBe(0);

  // ── 🚨 题面上不许有卷面脚手架 ────────────────────────────────────
  // 产品负责人 2026-09-21：「don't appear like 20分、第三节 书面表达」。
  // 清理在编译期做（promptlib.CleanPromptText），但**编译产物要真的上线** ——
  // 本地那份 JSON 干净、线上还是旧的，正是 go:embed 这条链子会出的错。
  const wide = await (
    await ctx.request.get(`${API}/api/v1/writing-prompts?pageSize=100`)
  ).json();
  for (const it of wide.items) {
    const first = (it.text as string).split("\n")[0];
    expect(first, `${it.id} 第一行还挂着题号：${first}`).not.toMatch(/^\s*\d{1,3}\s*[.．、]/);
    expect(first, `${it.id} 第一行还挂着章节：${first}`).not.toMatch(
      /第[一二三四五六七八九十\d]+\s*[部节]/,
    );
    expect(it.text, `${it.id} 还留着分值：${first}`).not.toMatch(/[（(]\s*\d{1,3}\s*分\s*[）)]/);
    expect((it.text as string).trim().length, `${it.id} 题面是空的`).toBeGreaterThan(5);
  }
  console.log(`题面干净：抽查 ${wide.items.length} 道，没有题号 / 章节 / 分值`);

  // ── 推荐 ────────────────────────────────────────────────────────
  expect(all.recommended.length, "落地页那一排推荐是空的").toBeGreaterThan(0);
  for (const r of all.recommended) {
    expect(r.prompt.id).toBeTruthy();
    expect(Array.isArray(r.prompt.topics), "topics 回了 null，前端 .map() 会崩").toBe(true);
  }

  // ── 从一道题开一篇写作 ──────────────────────────────────────────
  const pick = all.items[0];
  const started = await ctx.request.post(
    `${API}/api/v1/writing-prompts/${encodeURIComponent(pick.id)}/start`,
  );
  expect(started.ok(), `开写作失败：${started.status()} ${await started.text()}`).toBeTruthy();
  const wid = (await started.json()).id as string;

  const wr = await (await ctx.request.get(`${API}/api/v1/writings/${wid}`)).json();
  console.log("开出来的写作：", wr.title, "| lang =", wr.lang);
  // 🚨 题面要落在 assignedPrompt 上，不是落在她的第一句话里 ——
  // 她没说过那段字，存成 atom_message 整条链路下游都会当作「她说的」。
  expect(wr.assignedPrompt, "题面没进 assignedPrompt").toBeTruthy();
  expect(wr.assignedPrompt).toContain(pick.text.slice(0, 20));
  expect(wr.lang).toBe(pick.lang);
  // 🚨 题目自己写着「不少于800字」，就不该再在「开始之前」里问她一遍
  //（产品负责人 2026-09-21：这几样应该是定好的）。
  // 解得出来的必须已经写进去；解不出来的留空是对的 —— 宁可空着也不猜一个，
  // 她会被一个题目里根本不存在的要求追着跑。
  if (/\d{2,}/.test(pick.wordLimit ?? "")) {
    expect(
      wr.targetWords,
      `${pick.id} 的「${pick.wordLimit}」没有变成目标字数`,
    ).toBeGreaterThan(0);
  }
  console.log("目标字数 =", wr.targetWords, "| 题目写的是", pick.wordLimit);

  await ctx.close();
});

test("题库那一屏在浏览器里打得开", async ({ browser }) => {
  test.setTimeout(240_000);
  const ctx = await freshAccount(browser, "prompt-lib-ui");
  const page = await ctx.newPage();

  await page.goto("/writings/library");
  await expect(page.getByRole("heading", { name: "写作题库" })).toBeVisible({ timeout: 60_000 });

  // 卡片出来了。
  await expect(page.locator("article").first()).toBeVisible({ timeout: 60_000 });
  const cards = await page.locator("article").count();
  console.log(`第一屏 ${cards} 张卡`);
  expect(cards).toBeGreaterThan(0);

  // 筛选器那几行在。
  for (const label of ["语言", "考试", "难度", "话题"]) {
    await expect(page.getByText(label, { exact: true }).first()).toBeVisible();
  }

  // 🚨 页码条在。705 道题没有页码条，等于让她滚到天荒地老。
  await expect(page.getByRole("navigation", { name: "分页" })).toBeVisible();

  // 两档切换在，而且能走回「自己写」。
  await expect(page.getByRole("tab", { name: /写作题库/ })).toBeVisible();
  await page.getByRole("tab", { name: "自己写" }).click();
  await expect(page).toHaveURL(/\/writings$/, { timeout: 30_000 });

  await ctx.close();
});
