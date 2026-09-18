// One look at the writing room after the 2026-09-18 feedback round:
// the planning turns from the owner's screenshot (does every point land on the
// map, and does 印记 ask for a wider example?), the card layout of 段落, the
// 成稿 review with the rail scrolled away (the 「没有返回结果」 bug), and the
// naming dialog (keywords on demand, never a title).
//
// Usage (against a local stack — see e2e/run-stack.sh for the pieces):
//   cd apps/lite-web && E2E_BASE_URL=http://localhost:5184 node e2e/shootWriting.mjs <out-dir>
//
// Signs in as the seeded Phoebe, whose school must already be lite
// (globalSetup does that). Prints what it saw; the screenshots are for a person.
import { chromium } from "@playwright/test";
import path from "node:path";
import fs from "node:fs";

const BASE = process.env.E2E_BASE_URL ?? "http://localhost:5174";
// Online the API is its own host (relative /api hits the static site → 405).
// Locally the dev server proxies /api, so it stays empty.
const API = process.env.E2E_API_BASE ?? "";
// Online: sign up a fresh student into this class (the 写作走查班), never a real one.
const JOIN = process.env.E2E_JOIN_CODE ?? "";
const OUT = process.argv[2] ?? ".";
fs.mkdirSync(OUT, { recursive: true });
const shot = (page, name) => page.screenshot({ path: path.join(OUT, `${name}.png`) });

const browser = await chromium.launch();
const ctx = await browser.newContext({ baseURL: BASE, viewport: { width: 1440, height: 900 } });
const api = ctx.request;
const must = async (res, what) => {
  if (!res.ok()) throw new Error(`${what}: ${res.status()} ${await res.text()}`);
  return res.json();
};

if (JOIN) {
  const tag = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
  const email = `writecards-${tag}@demo.mindimprint.local`;
  const password = `wc-${tag}-pass`;
  await must(await api.post(`${API}/api/v1/auth/signup`, { data: { email, password, display_name: "卡片走查", join_code: JOIN } }), "signup");
  await must(await api.post(`${API}/api/v1/auth/signin`, { data: { email, password } }), "signin");
  console.log(`account ${email}`);
} else {
  await must(await api.post(`${API}/api/v1/auth/signin`, { data: { email: "phoebe@demo.mindimprint.local", password: "phoebe-dev-pass" } }), "signin");
}

// ── 1. planning: the owner's screenshot, turn by turn ──────────────────────
const { id } = await must(await api.post(`${API}/api/v1/writings`, { data: { idea: "人如何面对脆弱", lang: "zh" } }), "create");
await must(await api.put(`${API}/api/v1/writings/${id}/setup`, { data: { lang: "zh", targetWords: 800, note: "" } }), "setup");
const turns = [
  "脆弱不可怕，人是可以脆弱的。没有一个人可以不经历脆弱就能成长。",
  "我经历了爸爸进监狱，妹妹抑郁症，这些没有打垮我，反而让我成为了更沉着更懂得珍惜的人",
  "脆弱让人区别于机器",
  "还有 脆弱的感受往往也带来很多关于自己渴望的信息",
  // A historical example — the kind the new line asks for.
  "司马迁受了宫刑，最屈辱最脆弱的时候没有放弃，忍辱写完了《史记》",
];
for (const text of turns) {
  const t0 = Date.now();
  const r = await must(await api.post(`${API}/api/v1/writings/${id}/plan/turn`, { data: { text }, timeout: 180_000 }), "plan turn");
  console.log(`\n【她】${text}\n【印记 ${((Date.now() - t0) / 1000).toFixed(0)}s · ready=${r.ready}】${r.reply}`);
  for (const o of r.outline) console.log(`   ${"  ".repeat(o.depth)}- ${o.text}（${o.role}）`);
}

// ── 2. 段落: the card layout ────────────────────────────────────────────────
await must(await api.post(`${API}/api/v1/writings/${id}/stage`, { data: { stage: "snippets" } }), "stage snippets");
const page = await ctx.newPage();
await page.goto(`/writings/${id}`);
await page.locator("[data-write-card]").first().waitFor({ timeout: 60_000 });
const titles = await page.locator("[data-write-card]").evaluateAll((els) => els.map((e) => e.getAttribute("data-write-card")));
console.log(`\n卡片：${titles.join(" / ")}`);
await page.getByText("写作引导").first().waitFor({ timeout: 180_000 }).catch(() => console.log("（引导没等到）"));
await shot(page, "1-cards");

const paper = page.getByPlaceholder("写这一段……");
await page.locator("[data-write-card]").first().click();
await paper.fill("很多人把脆弱当成需要藏起来的东西。可我越来越觉得，脆弱不可怕，人是可以脆弱的。");
await page.locator("[data-write-card]").nth(1).click(); // leave mid-debounce: must still save
await page.waitForTimeout(1500);
const snips = await must(await api.get(`${API}/api/v1/writings/${id}/snippets`), "snippets");
console.log(`换卡之后服务端的片段：${JSON.stringify((snips.snippets ?? []).map((s) => s.text.slice(0, 12)))}`);
await shot(page, "2-second-card");
await page.getByRole("button", { name: "收起引导" }).click();
await shot(page, "3-guidance-folded");
await page.getByRole("button", { name: "展开引导" }).click();
const coachW = await page.locator(".student-coach-panel").evaluate((el) => el.getBoundingClientRect().width);
console.log(`印记那一栏宽 ${Math.round(coachW)}px`);

// 深入一层: markdown bold must render (no raw **), and 印记 talks TO her (你, not 她).
const deepen = page.getByRole("button", { name: "深入一层" }).first();
if (await deepen.count()) {
  await deepen.click();
  const drawer = page.getByRole("dialog");
  await drawer.waitFor();
  const composer = drawer.getByRole("textbox");
  await composer.fill("我这一段该用什么方法？请把方法名加粗告诉我。");
  await composer.press("Enter");
  await drawer.locator("strong").first().waitFor({ timeout: 120_000 }).catch(() => {});
  await page.waitForTimeout(500);
  const text = (await drawer.textContent()) ?? "";
  const bold = await drawer.locator("strong").count();
  console.log(`深入一层：加粗 ${bold} 处；原样星号：${text.includes("**")}；回复里「她那段/她的」：${/她那段|她的论点|她那/.test(text)}`);
  await shot(page, "2b-deepen");
  await page.keyboard.press("Escape");
  await drawer.getByRole("button").first().click().catch(() => {});
} else {
  console.log("（这张卡没有深入一层按钮）");
}

// ── 3. 成稿: 请印记看看 with the rail scrolled away ─────────────────────────
const essay = [
  "作者说“苦乐全在主观的心，不在客观的事”。同一份工作，有人觉得苦，有人觉得乐。",
  "刚开始练跳绳时，我只是为了完成体育老师布置的任务。每天拿起跳绳，我就想着赶紧跳完，边跳边数还剩多少个。绳子一绊住脚，我便更加烦躁，觉得这项练习既累又无聊。",
  "后来，我开始记录一分钟能跳多少个，还和朋友交流怎样保持节奏、减少中断。我的目标逐渐变成了超过上一次的自己。",
  "当我发现自己从频繁绊绳变得能够连续跳很久时，心里十分高兴。跳绳仍然让我出汗、喘气，但那些疲惫不再占据我全部的注意力。",
  "这段经历让我明白，乐趣可以在认真投入的过程中逐渐产生。当努力有了方向，辛苦有了意义，我们便更有可能带着期待继续前行。",
].join("\n\n");
await must(await api.put(`${API}/api/v1/writings/${id}/draft`, { data: { body: essay } }), "draft");
await must(await api.post(`${API}/api/v1/writings/${id}/stage`, { data: { stage: "draft" } }), "stage draft");
await page.goto(`/writings/${id}`);
await page.getByRole("button", { name: "请印记看看", exact: true }).waitFor();
const rail = page.locator("aside").last();
await rail.evaluate((el) => el.scrollTo(0, el.scrollHeight));
const t0 = Date.now();
await page.getByRole("button", { name: "请印记看看", exact: true }).click();
await page.getByText("印记正在通读全文").waitFor({ timeout: 10_000 });
await shot(page, "4-review-running");
await page.locator("[data-comment-point]").first().waitFor({ timeout: 240_000 });
await page.waitForTimeout(800); // the smooth scroll
const inView = await page.locator("[data-comment-point]").first().evaluate((el) => {
  const r = el.getBoundingClientRect();
  return r.top >= 0 && r.bottom <= window.innerHeight;
});
console.log(`\n意见 ${((Date.now() - t0) / 1000).toFixed(0)}s 后回来；第一条意见在屏幕里：${inView}`);
await shot(page, "5-review-landed");

// ── 4. 完成这篇 → the name is hers ─────────────────────────────────────────
await page.getByRole("button", { name: "完成这篇", exact: true }).click();
const box = page.getByLabel("这篇文章叫");
await box.waitFor({ timeout: 60_000 });
console.log(`起名框一开始是：「${await box.inputValue()}」`);
await shot(page, "6-name-empty");
await page.getByRole("button", { name: "需要提示" }).click();
await page.getByText("关键词（摘自你的正文）").waitFor({ timeout: 120_000 }).catch(() => {});
const chips = await page.locator('[role="dialog"] span.rounded-mk-full').allTextContents();
console.log(`关键词：${chips.join(" / ")}；都在正文里：${chips.every((k) => essay.includes(k))}`);
await shot(page, "7-name-keywords");

await browser.close();
