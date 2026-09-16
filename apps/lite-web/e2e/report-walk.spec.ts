import { expect, test, type Page } from "@playwright/test";

/**
 * report-walk.spec.ts — the last leg of the reports sub-project: a finished
 * reading's report walked end to end in a real browser, against the real
 * API and a real database, so the thirteen tasks that built it are proven
 * to work TOGETHER rather than only in isolation.
 *
 * Follows reading-walk.spec.ts's and coach-walk.spec.ts's own conventions:
 * a file-local `startReading` helper, strings taken from the sources (not
 * from the brief), and one continuous journey per test rather than a matrix
 * of tiny ones.
 *
 * What each leg is actually proving — see the file comments on
 * ReportPanel.tsx / SharePanel.tsx / PublicReportPage.tsx / exportPoster.ts
 * for the design intent each assertion below is standing in for:
 *
 *  - The report replaces the 「报告还在路上」 placeholder in the real render
 *    tree, not just in a unit test that renders ReportPanel directly.
 *  - Sharing mints a REAL link and a REAL QR image.
 *  - **The one assertion that matters most in this whole file**: the share
 *    URL is opened in `browser.newContext()` — a browser context with no
 *    cookies at all, not a second tab in the signed-in `page`. A new tab
 *    shares the session, so it would pass this walk even if the public page
 *    silently required a login; only a cookie-less context proves the page
 *    is actually public.
 *  - Revoking takes the link down immediately — asserted by reloading the
 *    SAME url in the SAME cookie-less context and getting the "gone" line.
 *  - The picture really exports: this is the only place in the whole test
 *    suite where a real PNG comes out of `exportPoster`'s `toPng` call — the
 *    unit test (Task 11) mocks the rasterizer, so it never proves this.
 */

const RUN = Date.now().toString(36);
const titled = (name: string) => `${name} ${RUN}`;

// Original prose, four paragraphs — same shape reading-walk.spec.ts uses,
// so this reading produces a real corpus for the report's own model call to
// draw 金句/gains from (report_facts.go's corpus is her own words, not the
// article's, but a real multi-paragraph article still exercises the real
// reading room rather than a one-line stub).
const ARTICLE_BODY = [
  "过去十年，全球太阳能装机容量增长了大约十倍。推动这件事的不是某一项突破性发明，而是制造规模、供应链和融资成本三件事同时变便宜。",
  "成本下降的幅度常被单独拎出来当作结论：组件价格在这十年里下降了八成以上。但价格只是发电成本的一部分，土地、并网、运维和资金成本在不同国家差别极大，同样的组件价格并不意味着同样的电价。",
  "真正的瓶颈已经从「发电贵不贵」转移到「电什么时候来」。太阳能的出力集中在正午前后，而用电高峰往往在傍晚，两者之间的错位要靠储能、需求响应或跨区输电来填。",
  "所以，如果储能和电网的问题不解决，继续增加装机带来的边际收益会递减：白天多出来的电卖不掉，甚至要被弃掉。十年的增长是真实的，但把它直接外推到下一个十年，是一种过于轻松的乐观。",
].join("\n\n");

// 🚨 逐字抄自 ReadingsLanding.tsx。这一份漂过一次（页面上是「PDF / DOCX / TXT」，
// 这里写的是「DOCX / PDF」），于是这条 walk 连房间都进不去，报的是 fill 超时。
const BODY_PLACEHOLDER = "贴一个链接，或者把整篇正文粘进来——也可以上传 PDF / DOCX / TXT";
const TITLE_PLACEHOLDER = "给这次阅读起个名字（可留空）";
const READING_URL = /\/readings\/[0-9a-f-]{36}$/;

/** Paste an article on the landing page and land in its room. */
async function startReading(page: Page, title: string): Promise<void> {
  await page.goto("/readings");
  await page.getByPlaceholder(TITLE_PLACEHOLDER).fill(title);
  await page.getByPlaceholder(BODY_PLACEHOLDER).fill(ARTICLE_BODY);
  await page.getByRole("button", { name: "开始阅读" }).click();
  await expect(page).toHaveURL(READING_URL, { timeout: 30_000 });
  await expect(page.locator(".mk-reading-room")).toBeVisible({ timeout: 30_000 });
}

/**
 * `完成这篇` → 一次确认，the same finishing sequence
 * reading-walk.spec.ts drives. Returns once the room shows the terminal
 * 已完成 surface — the report generator has not necessarily run yet at that
 * point, only the reading itself is finished.
 *
 * `ReadingRoomHost` only decides live-room-vs-已完成 in its mount effect
 * (`isFinishedReading(reading)`, checked once per `readingId`/`reloadKey`
 * change) — confirming the finalize dialog does not itself flip that
 * decision, it only marks the reading finished server-side. reading-walk.spec
 * proves the same thing by navigating away and back in through 我的阅读
 * before it ever asserts 已完成; this reloads the same URL instead, the
 * shorter of the two paths to the same cold-load check that spec's own last
 * assertion makes ("a cold load of the same URL is the same terminal
 * surface").
 */
async function finishReading(page: Page): Promise<void> {
  await page.getByRole("button", { name: "完成这篇" }).click();
  const finalize = page.getByRole("dialog", { name: "完成这篇" });
  // 完成之后不能再改，所以这一步要她确认一次——但只有确认，没有表格。
  await expect(finalize.getByRole("heading", { name: "完成这篇？" })).toBeVisible();
  await expect(
    finalize.getByText("完成之后这篇就不能再改了", { exact: false }),
  ).toBeVisible();
  await finalize.getByRole("button", { name: "完成，看报告" }).click();
  await expect(finalize).toBeHidden({ timeout: 30_000 });

  await page.reload();
  await expect(page.getByText("已完成", { exact: true })).toBeVisible({ timeout: 30_000 });
}

test("a finished reading's report: appears, is shared with a stranger, revoked, and exported as a picture", async ({
  page,
  browser,
  baseURL,
}) => {
  const title = titled("报告走查用的一篇");
  await startReading(page, title);
  await finishReading(page);

  // ── Step 2: the report appears where the old placeholder used to be ─────
  // The pre-redesign copy this sub-project replaced never appears again.
  await expect(page.getByText("这次阅读的报告还在路上。")).toHaveCount(0);

  // `ReportView` renders inside its own <article> — the only <article> on
  // this terminal surface once .mk-reading-room is gone, which lets every
  // assertion below scope INTO the report rather than risk matching the
  // duplicate copies FinishedReadingPanel keeps of the same strings (its
  // own 我的收获 card, its own <h1> title).
  //
  // Generous timeout: the FIRST open of a report is what generates it
  // server-side, a flagship-tier model call that can run tens of seconds.
  const report = page.locator("article");
  await expect(report).toBeVisible({ timeout: 150_000 });
  await expect(report.getByText("一次阅读的记录")).toBeVisible();
  await expect(report.getByRole("heading", { name: title })).toBeVisible();
  // 🚨 这条 walk 开一篇就直接完成，没有任何带读往来，所以报告里只有页眉和
  // 这次的数据——没有「我的收获」，也没有金句。以前有，是因为那张归纳表逼她
  // 手打一句；表删掉之后，这份报告诚实地薄。断言只压在必然在的那几样上。
  // 「这次的数据」是这一块的 aria-label，不是页面上的字——getByText 找不到它。
  await expect(report.getByRole("region", { name: "这次的数据" })).toBeVisible();
  // 统计 — assert the RULE, not one tile.
  //
  // This line used to read `expect(report.getByText("专注时长")).toBeVisible()`
  // and it has been stale since `bc99a23f` ("report drops zero stats"), which
  // landed 55 minutes after this file was written: `StatsRow` now filters out
  // every stat whose value is 0, because four coloured zeros read as a broken
  // page rather than as a record. This walk produces exactly that report —
  // it never talks to 印记 (和印记聊了 0 轮), never leaves a note (笔记 0 条),
  // never runs a 带读 step (读完 0 步), and finishes far inside the heartbeat's
  // 60-second cadence with an empty event trail behind the fallback estimate
  // (专注时长 0 分钟) — so the whole row is correctly absent.
  //
  // NOT a reading-room regression: the fork changed no stat, no heartbeat and
  // no report code, and every other assertion about this report still passes.
  //
  // What is worth pinning end to end is the rule itself, which until now had
  // unit coverage only: no zero reaches the page. Written against the value +
  // unit pair so it holds whether or not a slower run happens to earn its
  // first minute.
  await expect(report.getByText(/^0(分钟|轮|条|步)$/)).toHaveCount(0);

  // ── Step 3: turn sharing on — the link and the QR appear ────────────────
  //
  // 🚨 两步，不是一步。右上角那个「分享链接」只是把 SharePanel 展开；
  // 真正会把一个未成年人的作业发到公网上的那个按钮在面板里面
  // （ReportActions.tsx：「publishing a minor's schoolwork to a public URL is
  // never one click from arriving on a page」）。这条 walk 一直只点第一个，
  // 然后等一个还没出现的按钮——从加这道门起就红着。
  await page.getByRole("button", { name: "分享链接", exact: true }).click();
  await page.getByRole("button", { name: "生成分享链接" }).click();
  // 「分享链接」现在有两个：右上角那个展开面板的按钮，和面板里这个只读输入框。
  // 按角色区分，别让断言落在按钮上。
  const linkInput = page.getByRole("textbox", { name: "分享链接" });
  await expect(linkInput).toBeVisible({ timeout: 15_000 });
  const shareUrl = await linkInput.inputValue();
  expect(shareUrl).toMatch(/\/s\/[0-9a-f]+$/);
  await expect(page.getByAltText("分享二维码，扫码可以直接打开这份报告")).toBeVisible();

  // ── Step 4: open the share URL in a FRESH context with no session ───────
  // This is the assertion that matters most: a new TAB in `page`'s own
  // context would inherit its session cookie and pass even if the public
  // route silently required a login — proving nothing about public access.
  // `browser.newContext()` carries no cookies at all (it does not inherit
  // `use.storageState` from the config — that only applies to the `page`
  // fixture), so this is the only way to prove the link is really public.
  const strangerContext = await browser.newContext();
  const strangerPage = await strangerContext.newPage();
  await strangerPage.goto(shareUrl.startsWith("http") ? shareUrl : `${baseURL}${shareUrl}`);

  const strangerReport = strangerPage.locator("article");
  await expect(strangerReport).toBeVisible({ timeout: 30_000 });
  await expect(strangerReport.getByRole("heading", { name: title })).toBeVisible();
  // 陌生人看到的是同一份报告：页眉和标题都对得上。
  await expect(strangerReport.getByText("一次阅读的记录")).toBeVisible();
  // No sign-in screen: this page has no login control at all.
  await expect(strangerPage.getByPlaceholder(/邮箱|密码/)).toHaveCount(0);
  await expect(strangerPage.getByRole("button", { name: /登录|登陆/ })).toHaveCount(0);

  // ── Step 5: revoke, then reload the SAME public url in the SAME context ─
  await page.getByRole("button", { name: "停止分享" }).click();
  await expect(page.getByRole("button", { name: "生成分享链接" })).toBeVisible({ timeout: 15_000 });

  await strangerPage.reload();
  await expect(strangerPage.getByText("这份记录不存在，或者已经被收回了。")).toBeVisible({ timeout: 15_000 });
  await expect(strangerPage.locator("article")).toHaveCount(0);

  await strangerContext.close();

  // ── The picture actually exports ─────────────────────────────────────────
  // `exportPoster` builds a real <a download> anchor around a `data:` URL
  // and clicks it. Rather than fight a real browser "download" dialog for a
  // data: URI, this captures the href off the anchor at the moment it would
  // have been clicked — one of the two approaches the brief names, and the
  // more reliable one for a `data:` href specifically. Scoped to anchors
  // that carry `download` so ordinary link clicks elsewhere are untouched.
  await page.evaluate(() => {
    const proto = HTMLAnchorElement.prototype;
    const original = proto.click;
    (window as unknown as { __exportedDataUrl: string | null }).__exportedDataUrl = null;
    proto.click = function (this: HTMLAnchorElement) {
      if (this.download) {
        (window as unknown as { __exportedDataUrl: string | null }).__exportedDataUrl = this.href;
        return;
      }
      return original.call(this);
    };
  });

  const exportButton = page.getByRole("button", { name: "导出图片" });
  await expect(exportButton).toBeVisible();
  await exportButton.click();
  // Back to its resting label once the export (and the offscreen poster
  // mount/unmount around it) has finished.
  await expect(exportButton).toBeVisible({ timeout: 30_000 });

  const dataUrl = await page.evaluate(
    () => (window as unknown as { __exportedDataUrl: string | null }).__exportedDataUrl,
  );
  expect(dataUrl).not.toBeNull();
  expect(dataUrl!.startsWith("data:image/png")).toBe(true);
  expect(dataUrl!.length).toBeGreaterThan(1000);
});
