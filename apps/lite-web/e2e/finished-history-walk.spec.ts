import { execFileSync } from "node:child_process";
import { expect, test, type Page } from "@playwright/test";
import { freshAccount } from "./freshAccount";

/**
 * finished-history-walk.spec.ts —— 读完之后那一页，在真浏览器里走一遍。
 *
 * 这一条守的是 2026-09-16 那一版的三件事：
 *
 *  1. **回看。** 完成页有三格（报告 · 对话 · 原文），对话那一格里她的话和印记
 *     的话各自标明是谁说的，原文那一格能看到正文。产品负责人的原话：「they
 *     cannot go back to view their chat history.」
 *  2. **公开的范围由她定。** 没勾「公开我和印记的对话」时，那条公开链接上
 *     **一个字的对话都没有**；勾了才有。
 *  3. 开场那一句（「第 N 篇」）真的渲染出来了。
 *
 * ## 为什么自己注册一个账号
 *
 * `freshAccount`。共用的种子账号上「她读完过几篇」是累计的，而这一条要断
 * 「第 1 篇」—— 那是一个**一旦跑过就回不去**的前提，靠文件名的字母序守不住
 * （2026-09-05 的教训）。
 *
 * ## 为什么对话是 psql 塞进去的，不是聊出来的
 *
 * 聊出来要真的调模型：这套 walk 的栈（run-stack.sh）不带任何模型 key，而为了
 * 两句话去花一次调用，既慢又花钱（2026-09-12 的教训：先查是不是自己的走查工具
 * 在花钱）。这一条要验的是**那两句话在屏幕上长什么样**，不是模型会不会说话 ——
 * 后者有 coachwalk 专门管。
 *
 * 报告那一次模型调用同样会失败（没有 provider），于是报告以 `prosePending` 的
 * 样子渲染。那**也是**这条 walk 顺带证明的一件事：确定性的那一半自己站得住。
 */

const RUN = Date.now().toString(36);
const PG_CONTAINER = process.env.E2E_PG_CONTAINER ?? "mindimprint-lite-e2e-pg";

const ARTICLE_BODY = [
  "过去十年，全球太阳能装机容量增长了大约十倍。推动这件事的不是某一项突破性发明，而是制造规模、供应链和融资成本三件事同时变便宜。",
  "成本下降的幅度常被单独拎出来当作结论：组件价格在这十年里下降了八成以上。但价格只是发电成本的一部分，土地、并网、运维和资金成本在不同国家差别极大。",
  "真正的瓶颈已经从「发电贵不贵」转移到「电什么时候来」。太阳能的出力集中在正午前后，而用电高峰往往在傍晚。",
].join("\n\n");

const HER_LINE = `装机量和实际发电量不是一回事吧？${RUN}`;
const COACH_LINE = `对，这是两件事。你更想先弄清楚哪一个？${RUN}`;

// 🚨 逐字抄自 ReadingsLanding.tsx。report-walk.spec.ts 里那一份写的是
// 「DOCX / PDF」，而页面上是「PDF / DOCX / TXT」—— 那条 walk 现在也进不去房间。
const BODY_PLACEHOLDER = "贴一个链接，或者把整篇正文粘进来——也可以上传 PDF / DOCX / TXT";
const TITLE_PLACEHOLDER = "给这次阅读起个名字（可留空）";
const READING_URL = /\/readings\/([0-9a-f-]{36})$/;

function psql(sql: string): string {
  return execFileSync(
    "docker",
    ["exec", PG_CONTAINER, "psql", "-U", "postgres", "-d", "mindimprint", "-v", "ON_ERROR_STOP=1", "-tAc", sql],
    { encoding: "utf8" },
  ).trim();
}

/**
 * 直接往 atom_message 里塞一轮对话。
 *
 * seq 由 `max(seq)+1` 算出来，不写死：房间自己可能已经写过几条（导读、系统
 * 消息），写死会撞 atom_message_seq_idx 那个唯一索引。
 */
function seedTranscript(atomId: string): void {
  psql(
    `INSERT INTO atom_message (atom_id, seq, role, content)
     SELECT '${atomId}', COALESCE(MAX(seq), 0) + 1, 'student', '${HER_LINE}' FROM atom_message WHERE atom_id = '${atomId}'`,
  );
  psql(
    `INSERT INTO atom_message (atom_id, seq, role, content)
     SELECT '${atomId}', COALESCE(MAX(seq), 0) + 1, 'ai', '${COACH_LINE}' FROM atom_message WHERE atom_id = '${atomId}'`,
  );
}

async function startReading(page: Page, title: string): Promise<string> {
  await page.goto("/readings");
  await page.getByPlaceholder(TITLE_PLACEHOLDER).fill(title);
  await page.getByPlaceholder(BODY_PLACEHOLDER).fill(ARTICLE_BODY);
  await page.getByRole("button", { name: "开始阅读" }).click();
  await expect(page).toHaveURL(READING_URL, { timeout: 30_000 });
  const match = READING_URL.exec(page.url());
  if (!match?.[1]) throw new Error(`no reading id in ${page.url()}`);
  return match[1];
}

test("读完之后能回看对话和原文，公开的范围由她自己定", async ({ browser }, testInfo) => {
  const ctx = await freshAccount(browser, "finished-history-walk");
  const page = await ctx.newPage();

  const title = `太阳能的十年 ${RUN}`;
  const atomId = await startReading(page, title);
  seedTranscript(atomId);

  // 完成这一篇。对话框里的确认按钮写的是「完成，看报告」（report-walk 也是
  // 这么走的）。房间只在 mount effect 里判 live/已完成，所以完成之后要重进。
  await page.getByRole("button", { name: "完成这篇" }).click();
  const finalize = page.getByRole("dialog", { name: "完成这篇" });
  await finalize.getByRole("button", { name: "完成，看报告" }).click();
  await expect(finalize).toBeHidden({ timeout: 30_000 });
  await page.goto(`/readings/${atomId}`);
  await expect(page.getByRole("button", { name: "返回", exact: true })).toBeVisible({ timeout: 30_000 });

  /* ── 1 · 报告在，顶上只有返回，「查看阅读记录」在报告右上角 ──────────
     2026-09-18 产品负责人：「just a back button. and a right upper 查看阅读记录
     button, which near the download, link share button.」 */
  await expect(page.getByRole("tab")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "查看阅读记录" })).toBeVisible({ timeout: 60_000 });
  // 这是她的第一篇 —— fresh account 的意义就在这一条断言上。
  await expect(page.getByText(/这是我和印记一起读的第 1 篇文章/)).toBeVisible({ timeout: 60_000 });
  await page.screenshot({ path: testInfo.outputPath("1-report.png"), fullPage: true });

  /* ── 2 · 阅读记录：两句话，各自标明是谁说的 ─────────────────────── */
  await page.getByRole("button", { name: "查看阅读记录" }).click();
  await expect(page.getByText(HER_LINE)).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText(COACH_LINE)).toBeVisible();
  await expect(page.getByText("印记", { exact: true }).first()).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("2-transcript.png"), fullPage: true });

  /* ── 3 · 原文：报告下面「再看一遍这篇文章」，正文真的在 ─────────────── */
  await expect(page.getByRole("button", { name: "继续阅读" })).toBeVisible();
  await page.getByRole("button", { name: "返回报告" }).click();
  await page.getByRole("button", { name: "再看一遍这篇文章" }).click();
  // 🚨 要限定在原文那一块里。报告那一格是用 `hidden` 藏的（不卸载，否则每切
  // 一次页签就再买一次那通旗舰调用），而报告上的「我读的这篇」里也有这段摘录
  // —— 不限定的话 strict mode 会同时匹配到两个。
  await expect(
    page.locator("section.mk-rp-section").getByText(/过去十年，全球太阳能装机容量/),
  ).toBeVisible({ timeout: 15_000 });
  await page.screenshot({ path: testInfo.outputPath("3-source.png"), fullPage: true });

  /* ── 4 · 分享。默认**不**公开对话 ────────────────────────────────── */
  await page.getByRole("tab", { name: "报告" }).click();
  await page.getByRole("button", { name: /分享/ }).click();
  await page.getByRole("button", { name: "生成分享链接" }).click();
  // 🚨 `getByRole("textbox")`，不是 `getByLabel`：那颗打开面板的按钮的
  // aria-label 也是「分享链接」，按 label 找会先撞上它。
  const link = page.getByRole("textbox", { name: "分享链接" });
  await expect(link).toHaveValue(/\/s\/[0-9a-f]{32}$/, { timeout: 60_000 });
  const shareUrl = await link.inputValue();

  // 🚨 没有 cookie 的 context，不是第二个标签页。第二个标签页共用 session，
  // 就算公开页偷偷要求登录也照样能过。
  const guest = await browser.newContext();
  const guestPage = await guest.newPage();
  await guestPage.goto(shareUrl);
  // 标题在这一页上出现两次：手记的大标题，和「我读的这篇」那一块里的标题。
  // 断大标题那一个。
  await expect(guestPage.getByRole("heading", { name: title })).toBeVisible({ timeout: 30_000 });
  // 这一条是这个文件里最要紧的一条：她没勾，链接上就一个字的对话都没有。
  await expect(guestPage.getByText(HER_LINE)).toHaveCount(0);
  await expect(guestPage.getByText(COACH_LINE)).toHaveCount(0);
  await guestPage.screenshot({ path: testInfo.outputPath("4-public-no-transcript.png"), fullPage: true });

  /* ── 5 · 勾上之后，同一条链接上才有对话 ──────────────────────────── */
  await page.getByLabel(/公开我和印记的对话/).check();
  await expect.poll(async () => {
    await guestPage.reload();
    return guestPage.getByText(HER_LINE).count();
  }, { timeout: 30_000 }).toBeGreaterThan(0);
  await expect(guestPage.getByText(COACH_LINE)).toBeVisible();
  await guestPage.screenshot({ path: testInfo.outputPath("5-public-with-transcript.png"), fullPage: true });

  await guest.close();
  await ctx.close();
});
