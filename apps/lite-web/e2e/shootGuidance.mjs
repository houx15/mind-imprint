// 走一遍新的带读，把每一步拍下来。
//
// 这不是断言测试（那是 library-walk.spec.ts）。这是**拿眼睛看**的那一半：
// 导读卡长什么样、板能不能拖、拖完印记接不接得住。2026-08-30 的教训是
// 344 个测试全绿而导出的 PNG 是全白的 —— 布局和交互只有真浏览器分得出来。
//
// 用法：
//   cd apps/lite-web && node e2e/shootGuidance.mjs <out-dir> [slug] [tier]
import { chromium } from "@playwright/test";
import path from "node:path";

const API = process.env.E2E_API_BASE ?? "https://mind-api.uni-robot.cn";
const BASE = process.env.E2E_BASE_URL ?? "https://mind-lite.uni-robot.cn";
const JOIN = process.env.E2E_JOIN_CODE ?? "G624-UXFE";
const OUT = process.argv[2] ?? ".";
const SLUG = process.argv[3] ?? "oh-mideast-crisis-aid-groups";
const TIER = process.argv[4] ?? "3";

const tag = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`;
const email = `guide-${tag}@demo.mindimprint.local`;
const password = `guide-${tag}-pass`;

const browser = await chromium.launch();
const ctx = await browser.newContext({ baseURL: BASE, viewport: { width: 1440, height: 1000 } });

const up = await ctx.request.post(`${API}/api/v1/auth/signup`, {
  data: { email, password, display_name: "走查", join_code: JOIN },
});
if (!up.ok()) throw new Error(`signup ${up.status()} ${await up.text()}`);
await ctx.request.post(`${API}/api/v1/auth/signin`, { data: { email, password } });

const started = await ctx.request.post(`${API}/api/v1/library/${SLUG}/levels/${TIER}`);
if (!started.ok()) throw new Error(`start ${started.status()} ${await started.text()}`);
const { id } = await started.json();
console.log("reading", id);

const page = await ctx.newPage();
page.on("console", (m) => {
  if (m.type() === "error") console.log("  console.error:", m.text().slice(0, 160));
});
await page.goto(`/readings/${id}`);
await page.locator(".mk-reading-room__article").first().waitFor({ timeout: 30_000 });

async function shot(name) {
  await page.screenshot({ path: path.join(OUT, `${name}.png`), fullPage: false });
  console.log("  shot", name);
}

await shot("00-room");

// 开始 → 排读法 → 导读卡应该出现在正文顶上。
await page.getByRole("button", { name: "开始" }).click();
console.log("clicked 开始, waiting for the plan…");
await page.locator(".mk-reading-outline").waitFor({ timeout: 120_000 });
const outline = (await page.locator(".mk-reading-outline").innerText()).replace(/\n/g, " | ");
console.log("OUTLINE:", outline);
console.log("core paragraphs marked in the article:", await page.locator("p[data-core]").count());
await shot("01-outline");

// 印记 的第一轮：它不该再把文章讲一遍。
await page.locator('[data-chat-row="ai"]').first().waitFor({ timeout: 120_000 });
const first = await page.locator('[data-chat-row="ai"]').first().innerText();
console.log("FIRST TURN:", JSON.stringify(first));
console.log("first turn length:", [...first].length);
await shot("02-first-turn");

/** 印记 打完这一轮字了没有。
 *
 * 🚨 不等它打完就去数卡片，会把「它还在打字」记成「它这一轮没给卡片」——
 * 走查第一遍就是这么误报的（截图里那三个点还在跳）。 */
async function settle() {
  await page.waitForFunction(
    () => !document.querySelector('[aria-label="印记正在打字"]'),
    null,
    { timeout: 120_000 },
  );
  await page.waitForTimeout(400);
}

// 走几轮，看能不能碰到一块板。
for (let turn = 0; turn < 10; turn += 1) {
  await settle();
  const board = page.locator(".mk-board").last();
  if (await board.count()) {
    console.log(`turn ${turn}: A BOARD APPEARED`);
    await board.scrollIntoViewIfNeeded();
    await shot(`1${turn}-board`);
    // 点选那条路径：点一张卡，再点一个格子。逐张摆完。
    for (;;) {
      const chips = board.locator(".mk-board__loose .mk-board__chip");
      if (!(await chips.count())) break;
      await chips.first().click();
      await board.locator(".mk-board__bin").first().click();
    }
    await shot(`1${turn}-board-filled`);
    await board.locator(".mk-board__submit").click();
    console.log(`turn ${turn}: board submitted, waiting for 印记 to pick it up…`);
    await settle();
    const back = await page.locator('[data-chat-row="ai"]').last().innerText();
    console.log(`turn ${turn}: 印记 answered the board: ${JSON.stringify(back.slice(0, 140))}`);
    await shot(`1${turn}-board-answered`);
    continue;
  }

  const card = page.locator('[data-coach-card="open"]').last();
  if (!(await card.count())) {
    // 没有卡片不是走不下去 —— 说一句话，让它继续带。
    const said = await page.locator('[data-chat-row="ai"]').last().innerText();
    console.log(`turn ${turn}: no card. 印记 said: ${JSON.stringify(said.slice(0, 90))}`);
    await shot(`1${turn}-notcard`);
    await page.locator("textarea, input[type=text]").last().fill("好，我读完了这一段。");
    await page.keyboard.press("Enter");
    await page.waitForTimeout(1500);
    continue;
  }
  const kind = (await card.locator("textarea").count())
    ? "short_text"
    : (await card.locator("ul li button").count())
      ? "choose_span"
      : "pick_in_article";
  console.log(`turn ${turn}: card = ${kind} — ${(await card.locator("p").first().innerText()).slice(0, 60)}`);
  if (kind === "choose_span") {
    await card.locator("ul li button").first().click();
  } else if (kind === "short_text") {
    await card.locator("textarea").fill("我觉得作者主要在讲援助进不去这件事。");
    await card.getByRole("button", { name: "说说看" }).click();
  } else {
    console.log("  pick_in_article — 需要在正文里划选，这条自动化跳过");
    break;
  }
  await page.waitForTimeout(1200);
}

await shot("20-final");
console.log("done");
await browser.close();
