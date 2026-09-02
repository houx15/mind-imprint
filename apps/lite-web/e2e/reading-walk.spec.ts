import { expect, test, type Page } from "@playwright/test";

/**
 * The lite edition's whole P1 journey, walked end to end against a real API
 * and a real database: land → paste an article → read it in the REAL room →
 * write 我的收获 → 完成 → see it as 已完成.
 *
 * What each leg is actually protecting:
 *
 *  - **The room is the pro room.** 透镜库 with the real deck, a tool card that
 *    hangs under a paragraph, and annotation highlights rendered off stored
 *    anchors. This is the architectural claim of the whole lite edition — that
 *    it re-uses `apps/web`'s `ReadingRoom` rather than a thin copy of it — so
 *    a walk that only proved "a page loaded" would prove nothing.
 *  - **Project-only surfaces are gone.** 证据笔记 / 追来源 / 新的线索 /
 *    对论点的影响 are asserted ABSENT. That is `LITE_READING_CAPABILITIES`
 *    doing its one job; if a future edit drops the prop or flips a flag, this
 *    is the test that notices.
 *  - **Finished is terminal.** Reopening a finished reading must show the
 *    已完成 panel, never a live room she could summon fresh lenses in.
 *  - **Real Back/Forward.** Driven with page.goBack()/goForward(), not the
 *    shell's own synthetic popstate — jsdom has never been able to catch the
 *    deep-link/Back bugs this repo keeps rediscovering.
 *
 * Strings here are taken from the sources, not from the (pre-redesign) brief:
 * the landing page from apps/lite-web/src/readings/ReadingsLanding.tsx, the
 * drawer from ReadingHistoryPanel.tsx, the 已完成 panel from
 * ReadingRoomHost.tsx, and the room's own chrome from
 * apps/web/src/studio/reading/{ReadingRoom,LensLibrary,HangingCard,FinalizeReadingPanel}.tsx.
 */

// ── the article she brings in ────────────────────────────────────────────────
// Original prose, four paragraphs, deliberately shaped to give the room
// something real to work on: a mechanism, a number, and a concession that
// complicates it. Blank lines matter — the server splits blocks on them
// (apps/api/internal/api/reading_blocks.go).
// Titles are suffixed per run. The walk creates real rows in a real database
// and nothing here deletes them, so a second run against the same stack must
// not find two readings answering to the same name in 我的阅读.
const RUN = Date.now().toString(36);
const titled = (name: string) => `${name} ${RUN}`;

const ARTICLE_TITLE = titled("太阳能的十年");
const ARTICLE_BODY = [
  "过去十年，全球太阳能装机容量增长了大约十倍。推动这件事的不是某一项突破性发明，而是制造规模、供应链和融资成本三件事同时变便宜。",
  "成本下降的幅度常被单独拎出来当作结论：组件价格在这十年里下降了八成以上。但价格只是发电成本的一部分，土地、并网、运维和资金成本在不同国家差别极大，同样的组件价格并不意味着同样的电价。",
  "真正的瓶颈已经从「发电贵不贵」转移到「电什么时候来」。太阳能的出力集中在正午前后，而用电高峰往往在傍晚，两者之间的错位要靠储能、需求响应或跨区输电来填。",
  "所以，如果储能和电网的问题不解决，继续增加装机带来的边际收益会递减：白天多出来的电卖不掉，甚至要被弃掉。十年的增长是真实的，但把它直接外推到下一个十年，是一种过于轻松的乐观。",
].join("\n\n");

const BODY_PLACEHOLDER = "贴一个链接，或者把整篇正文粘进来——也可以上传 DOCX / PDF";
const TITLE_PLACEHOLDER = "给这次阅读起个名字（可留空）";
const READING_URL = /\/readings\/[0-9a-f-]{36}$/;

// The greeting is split across elements — 读 is its own <span> so the ink ring
// can be drawn behind it — so it is matched on the heading's textContent, not
// with a text selector over the whole phrase.
async function expectGreeting(page: Page): Promise<void> {
  const heading = page.getByRole("heading", { level: 1 });
  await expect(heading).toBeVisible();
  expect(await heading.textContent()).toBe("Hi，今天要读点什么");
}

/** Paste an article on the landing page and land in its room. Returns the id. */
async function startReading(page: Page, title: string, body: string): Promise<string> {
  await page.goto("/readings");
  await expectGreeting(page);
  await page.getByPlaceholder(TITLE_PLACEHOLDER).fill(title);
  await page.getByPlaceholder(BODY_PLACEHOLDER).fill(body);
  await page.getByRole("button", { name: "开始阅读" }).click();
  await expect(page).toHaveURL(READING_URL, { timeout: 30_000 });
  await expect(page.locator(".mk-reading-room")).toBeVisible({ timeout: 30_000 });
  return new URL(page.url()).pathname.split("/").pop()!;
}

/**
 * Every surface that belongs to a research project and must never appear in a
 * lite room. Asserted on DOM counts (not visibility) so a hidden-but-rendered
 * surface still fails.
 *
 * RULE FOR ANYONE ADDING TO THIS LIST: an absence assertion can never fail on
 * its own, so a literal that does not exist in the product is indistinguishable
 * from one that is correctly absent — it reads as coverage while guarding
 * nothing. This list carried 「追踪来源」 for one commit; the real string is
 * 「追来源」 and the longer form appears nowhere in apps/web/src.
 *
 * Since the 2026-08-29 fork none of the four is behind a `caps.*` flag on the
 * lite path — lite's own `readings/ReadingRoom.tsx` does not contain the
 * first two at all, and passes `proposalImpact={false}` / `credibility={false}`
 * as literals to the (still shared) `FinalizeReadingPanel`. Where each one
 * still lives, and therefore what this list is watching for regressing back
 * into lite:
 *
 *   证据笔记      apps/web  ReadingRoom.tsx           (pro's room only)
 *   追来源        apps/web  ReadingRoom.tsx           (pro's room only)
 *   新的线索      apps/web  FinalizeReadingPanel.tsx  (prop proposalImpact)
 *   对论点的影响  apps/web  FinalizeReadingPanel.tsx  (prop proposalImpact)
 *
 * Only the last two are reachable from lite at all, since that panel is shared
 * — the first two would need someone to import pro's room back into lite. The
 * unit-level guard is apps/lite-web/test/readingRoomCapabilities.test.tsx,
 * which renders LITE's room, opens 完成这篇, and asserts on the rendered modal.
 * These four lines are belt-and-braces on top of it.
 */
async function expectNoProjectSurfaces(page: Page): Promise<void> {
  await expect(page.getByText("证据笔记")).toHaveCount(0);
  await expect(page.getByText("追来源")).toHaveCount(0);
  await expect(page.getByText("新的线索")).toHaveCount(0);
  await expect(page.getByText("对论点的影响")).toHaveCount(0);
}

test("the landing page is the front door: greeting, shelf, and one way in", async ({ page }) => {
  await page.goto("/readings");
  await expectGreeting(page);

  // The three ways in share one box.
  await expect(page.getByPlaceholder(TITLE_PLACEHOLDER)).toBeVisible();
  await expect(page.getByPlaceholder(BODY_PLACEHOLDER)).toBeVisible();
  await expect(page.getByRole("button", { name: "开始阅读" })).toBeVisible();
  await expect(page.getByText("上传 DOCX / PDF")).toBeVisible();

  // The shelf, for a student who does not know what to read. 铁律②: a fixed
  // four, and nothing that invites her to keep scrolling.
  await expect(page.getByText("不知道读什么？")).toBeVisible();
  for (const title of [
    "城市为什么比郊区热？",
    "一份外卖的配送费，到底付给了谁？",
    "记忆不是一盘录像带",
    "The Gettysburg Address",
  ]) {
    await expect(page.getByText(title, { exact: true })).toBeVisible();
  }
  for (const bait of ["加载更多", "更多推荐", "继续读"]) {
    await expect(page.getByText(bait)).toHaveCount(0);
  }

  // 我的阅读 is a drawer, not a feed on the page.
  await page.getByRole("button", { name: /我的阅读/ }).click();
  await expect(page.getByRole("heading", { name: "我的阅读" })).toBeVisible();
  await page.getByRole("button", { name: "关闭" }).click();
  await expect(page.getByRole("heading", { name: "我的阅读" })).toBeHidden();
});

test("lite reading walk: paste → the real room → 收获 → 完成 → 已完成", async ({ page }) => {
  const id = await startReading(page, ARTICLE_TITLE, ARTICLE_BODY);

  // ── the article really made the round trip ────────────────────────────────
  await expect(page.getByRole("heading", { name: ARTICLE_TITLE, level: 2 })).toBeVisible();
  await expect(page.getByText("成本下降的幅度常被单独拎出来当作结论")).toBeVisible();
  await expect(page.locator("p[data-block-id]")).toHaveCount(4);

  // ── it is the REAL room ───────────────────────────────────────────────────
  // 透镜库 · N carries the real reading deck's size, so an empty or stubbed
  // deck fails here rather than silently rendering a button.
  const deckButton = page.getByRole("button", { name: /^透镜库 · \d+$/ });
  await expect(deckButton).toBeVisible();
  const deckLabel = (await deckButton.textContent()) ?? "";
  expect(Number(deckLabel.replace(/\D/g, ""))).toBeGreaterThanOrEqual(5);

  await deckButton.click();
  const library = page.getByRole("dialog", { name: "透镜库" });
  await expect(library.getByRole("heading", { name: "挑一副透镜，换个角度读这篇文章" })).toBeVisible();
  expect(await library.locator(".mk-lens-library__row").count()).toBeGreaterThanOrEqual(5);
  await library.getByRole("button", { name: "关闭透镜库" }).click();
  await expect(library).toBeHidden();

  // The room's own surfaces: the two view tabs and 完成这篇.
  await expect(page.getByRole("tab", { name: "文章" })).toBeVisible();
  await expect(page.getByRole("tab", { name: /阅读成果/ })).toBeVisible();
  await expect(page.getByRole("button", { name: "完成这篇" })).toBeVisible();

  // ONE 印记. The coach column carries the 带读 invitation, and the room's own
  // chat log, composer and starter row are NOT also on the page — two AI chat
  // boxes side by side is exactly what this replaced.
  await expect(page.getByText("让我来带你详细读一遍这篇文章吧。")).toBeVisible();
  await expect(page.getByPlaceholder("说说你对哪一句有疑问…")).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "换一个视角，再读一遍" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "这条来源可信吗？" })).toHaveCount(0);
  // 这篇用在哪个阶段 names PROJECT phases; a lite reading is not in one.
  await expect(page.getByLabel("这篇材料用在哪个阶段")).toHaveCount(0);

  // ── project-only surfaces are absent (LITE_READING_CAPABILITIES) ──────────
  await expectNoProjectSurfaces(page);

  // ── annotation: a stored margin note comes back as a highlight ────────────
  // Written through the API rather than the UI because P1's room has no
  // note-authoring control yet (annotations are read-only there); what is
  // being proved is the projection — stored anchor → <mark> in the article →
  // click reveals the note.
  const source = await page.request
    .get(`/api/v1/readings/${id}/source`)
    .then((r) => r.json() as Promise<{ blocks: { id: string; text: string }[] }>);
  const block = source.blocks[1];
  const quote = "组件价格在这十年里下降了八成以上";
  const start = block.text.indexOf(quote);
  expect(start).toBeGreaterThan(-1);
  const created = await page.request.post(`/api/v1/readings/${id}/annotations`, {
    data: {
      blockId: block.id,
      span: { start, end: start + quote.length },
      quote,
      note: "这里只说了组件价格，没说并网和运维。",
    },
  });
  expect(created.ok()).toBeTruthy();

  // Reload — which also proves the deep link into /readings/:id survives a
  // real page load, not just an in-app navigation.
  await page.reload();
  await expect(page.locator(".mk-reading-room")).toBeVisible({ timeout: 30_000 });
  const mark = page.locator("p[data-block-id] mark", { hasText: quote });
  await expect(mark).toBeVisible();
  await mark.click();
  await expect(page.getByText("这里只说了组件价格，没说并网和运维。")).toBeVisible();
  await expect(page.getByText("批注", { exact: true })).toBeVisible();

  // ── 完成这篇 → 一次确认 → 报告 ────────────────────────────────────────────
  //
  // 🚨 这里原来是一张归纳表（我的收获 + 只读的阅读记录 + 确认归纳）。它被删掉了，
  // ReadingRoom.tsx 里留着产品负责人的原话：「we have give abundant steps for the
  // reading. so we don't need to ask student to enter the form again.」
  // 这条 walk 一直没跟上，于是从那次简化起就红着。
  await page.getByRole("button", { name: "完成这篇" }).click();
  const finalize = page.getByRole("dialog", { name: "完成这篇" });
  // 完成之后不能再改，所以这一步要她确认一次——但只有确认，没有表格。
  await expect(finalize.getByRole("heading", { name: "完成这篇？" })).toBeVisible();
  await expect(
    finalize.getByText("完成之后这篇就不能再改了", { exact: false }),
  ).toBeVisible();
  await finalize.getByRole("button", { name: "完成，看报告" }).click();
  await expect(finalize).toBeHidden({ timeout: 30_000 });

  // ── it is 已完成 afterwards, and it does NOT reopen as a live room ────────
  // D2 fix (Task 18): the room's back button used to read 「返回工作区」 in the
  // lite room too — pro vocabulary a lite student has never seen (she has no
  // 工作区, only 我的阅读). ReadingRoom now varies the label by
  // `capabilities.mode`; asserted verbatim here so a regression back to the
  // pro string is caught by this walk.
  //
  // `exact: true` IS THE ASSERTION. Playwright matches accessible names by
  // SUBSTRING by default, so a bare { name: "返回" } also matches
  // 「返回工作区」 — it would pass on exactly the regression it exists to
  // catch. Same trap apps/web/e2e/helpers.ts already documents for 登录 vs
  // 退出登录. Do not drop the flag.
  //
  // 标签后来从「返回」变成了「回到阅读」——说的正是这条断言一直在守的那件事：
  // lite 的学生没有「工作区」，只有「我的阅读」。walk 没跟上，于是它红着，
  // 而它红的理由恰恰是文案变对了。
  await expect(page.getByRole("button", { name: "返回工作区" })).toHaveCount(0);
  await page.getByRole("button", { name: "回到阅读", exact: true }).click();
  await expect(page).toHaveURL(/\/readings$/);
  await expectGreeting(page);

  await page.getByRole("button", { name: /我的阅读/ }).click();
  // The groups are chips now, not section labels — one time-ordered list,
  // filtered by a control she can actually hit.
  await expect(page.getByRole("button", { name: /^已完成 \d+$/ })).toBeVisible();
  await expect(page.getByRole("button", { name: new RegExp(`${ARTICLE_TITLE}.*看报告`, "s") })).toBeVisible();
  await page.getByRole("button", { name: new RegExp(`${ARTICLE_TITLE}.*看报告`, "s") }).click();

  await expect(page).toHaveURL(new RegExp(`/readings/${id}$`));
  await expect(page.getByText("已完成", { exact: true })).toBeVisible();
  // 🚨 第一次打开这份报告就是在生成它——一次旗舰模型调用，几十秒。标题在报告
  // 里面，所以要先等报告落下来，否则断言等到的是一块还在生成的空位。
  const report = page.locator("article");
  await expect(report).toBeVisible({ timeout: 150_000 });
  await expect(report.getByRole("heading", { name: ARTICLE_TITLE })).toBeVisible();
  // 🚨 她不再手打一句「我的收获」——那张归纳表被删掉了（见上面）。报告里有
  // 什么由印记从她这次真读过的东西里写，内容不可预测，所以只压在必然在的
  // 那一块上。
  await expect(report.getByRole("region", { name: "这次的数据" })).toBeVisible();
  await expect(page.getByRole("button", { name: "回到阅读" })).toBeVisible();
  // Terminal means terminal: no room, so nothing that could summon a lens.
  await expect(page.locator(".mk-reading-room")).toHaveCount(0);
  await expect(page.getByRole("button", { name: /^透镜库/ })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "完成这篇" })).toHaveCount(0);

  // A cold load of the same URL is the same terminal surface — the finished
  // gate lives in the host's loader, not in in-app state.
  await page.reload();
  await expect(page.getByText("已完成", { exact: true })).toBeVisible({ timeout: 30_000 });
  await expect(page.locator(".mk-reading-room")).toHaveCount(0);
});

test("the browser's own Back/Forward move between the landing page and the room", async ({ page }) => {
  const backTitle = titled("回退走查用的一篇");
  const id = await startReading(page, backTitle, ARTICLE_BODY);

  // Real browser Back — not the shell's synthetic popstate, which is all the
  // unit tests have ever exercised.
  await page.goBack();
  await expect(page).toHaveURL(/\/readings$/);
  await expectGreeting(page);
  await expect(page.locator(".mk-reading-room")).toHaveCount(0);

  await page.goForward();
  await expect(page).toHaveURL(new RegExp(`/readings/${id}$`));
  await expect(page.locator(".mk-reading-room")).toBeVisible({ timeout: 30_000 });
  await expect(page.getByRole("heading", { name: backTitle, level: 2 })).toBeVisible();

  // And Back again from the restored room, so a second pop is proven too —
  // one-shot ref guards that never reset are exactly how this repo's previous
  // deep-link/Back bugs presented.
  await page.goBack();
  await expect(page).toHaveURL(/\/readings$/);
  await expectGreeting(page);

  // The drawer is the other way back into the same room, and it must land on
  // the same URL.
  await page.getByRole("button", { name: /我的阅读/ }).click();
  await page.getByRole("button", { name: new RegExp(`${backTitle}.*继续`, "s") }).click();
  await expect(page).toHaveURL(new RegExp(`/readings/${id}$`));
  await expect(page.locator(".mk-reading-room")).toBeVisible({ timeout: 30_000 });
});

test("the coach answers for real, and a failure would be said out loud", async ({ page }) => {
  await startReading(page, titled("陪练走查用的一篇"), ARTICLE_BODY);

  // There is ONE conversation in the room now, and it is 带读 — so this
  // drives the same chat a student drives, from its own front door.
  //
  // CAREFUL: the typing bubble is itself an assistant-role node, so counting
  // `[data-role="assistant"]` alone would tick up the instant the request left
  // the browser and pass without any reply at all. Every wait below is on the
  // typing bubble CLEARING, which only happens when the turn resolves.
  const assistantTurns = page.locator('[data-role="assistant"]:not([aria-label])');
  const thinking = page.locator('[aria-label="印记正在打字"]');
  await expect(assistantTurns).toHaveCount(0);

  // 开始 is one live model call: it plans the route AND leads her into step one.
  //
  // The wait is on the assistant TURN arriving, never on the typing bubble
  // clearing — `expect(thinking).toHaveCount(0)` is already true in the
  // millisecond before the bubble mounts, so it would resolve instantly and
  // pass the whole turn by.
  await page.getByRole("button", { name: "开始", exact: true }).click();
  await expect(assistantTurns).toHaveCount(1, { timeout: 180_000 });
  // 🚨 打字气泡要等这一轮真的结束才收——而回话是边流边渲的，行出现的时候这
  // 一轮往往还在跑。旗舰模型一轮 54 秒到 1 分 26 秒都见过（2026-09-02 的日志），
  // 默认那 15 秒根本不够，这条断言于是在"模型慢"和"这一轮卡住了"之间分不清。
  await expect(thinking).toHaveCount(0, { timeout: 180_000 });

  await page.getByPlaceholder(/读完这一步|还想聊点什么/).fill("第四段说边际收益会递减，这个推论站得住吗？");
  await page.getByRole("button", { name: "发送" }).click();
  await expect(page.locator('[data-role="student"]')).toHaveCount(1);

  // A second live model call. The assertion is on the REPLY being real rather
  // than on an exact count, so a turn that also opens a paragraph tool passes.
  await expect(assistantTurns).toHaveCount(2, { timeout: 180_000 });
  await expect(thinking).toHaveCount(0, { timeout: 180_000 });
  const reply = (await assistantTurns.nth(1).innerText()).trim();
  expect(reply.length).toBeGreaterThan(10);
  // She is being LED, not answered: 印记 must not hand back the article's own
  // conclusion, and must not talk in the internal block ids.
  expect(reply).not.toMatch(/\bb\d+\b/);

  // Standing rule: an AI failure is SURFACED, never masked as a coach
  // sentence. If the turn had failed, the host's banner would be here.
  await expect(page.getByRole("alert")).toHaveCount(0);
});

test("the room's AI is live: a summoned lens hangs under a paragraph and becomes a finding", async ({ page }) => {
  await startReading(page, titled("透镜走查用的一篇"), ARTICLE_BODY);
  await expect(page.getByRole("tab", { name: "阅读成果 0" })).toBeVisible();

  await page.getByRole("button", { name: /^透镜库 · \d+$/ }).click();
  const library = page.getByRole("dialog", { name: "透镜库" });
  const firstLens = library.locator(".mk-lens-library__row").first();
  const lensName = (await firstLens.locator(".mk-lens-library__row-name").textContent())?.trim() ?? "";
  expect(lensName.length).toBeGreaterThan(0);
  await firstLens.click();

  // One live model call. The card is the product's signature move, so this
  // waits for the real thing rather than settling for "the request was sent".
  const card = page.locator(".lens-connector").locator("..");
  await expect(card).toBeVisible({ timeout: 180_000 });
  await expect(card.getByText(lensName)).toBeVisible();

  // PARAGRAPH-ANCHORED: the card is rendered immediately after one of the
  // article's own blocks (renderAfterBlock), which is what makes it read as a
  // margin note on that paragraph rather than a floating modal.
  const anchorBlock = await card.evaluate((el) => {
    const prev = el.previousElementSibling;
    return prev && prev.tagName === "P" ? prev.getAttribute("data-block-id") : null;
  });
  expect(anchorBlock).toMatch(/^b\d+$/);

  // The room stays lite even with a card open.
  await expectNoProjectSurfaces(page);
  // Deadlock guard: an in-flight lens is always dismissable.
  await expect(page.getByRole("button", { name: "跳过这副透镜" })).toBeVisible();

  // ── the rest of the card's own loop: pick a sentence, get reviewed, keep it
  // (D1 — the refusal when she clicks the AI's OWN example — has its own test
  // below, because asserting it needs a `test.skip()` that would otherwise
  // abort this whole walk.)
  await card.getByRole("button", { name: /开始选句$/ }).click();
  await expect(card.getByText("在文章里点出你自己的证据句")).toBeVisible();

  // Her pick has to be a paragraph OTHER than the one carrying the AI's
  // example: `useReadingLoop.pickSentence` rejects a span overlapping the
  // example outright — she must choose for herself. Since Task 18 that
  // refusal is spoken rather than silent (asserted in the D1 test below);
  // this walk still picks elsewhere because it wants the accepted path.
  const ownBlock = anchorBlock === "b1" ? "b3" : "b1";
  await page.locator(`p[data-block-id="${ownBlock}"]`).click();

  // A second live call — the review of HER sentence, which is what makes the
  // card a thinking tool rather than a form.
  await expect(card.getByRole("button", { name: "记下这条发现" })).toBeVisible({ timeout: 180_000 });
  await expect(card.getByRole("button", { name: "重新选一句" })).toBeVisible();
  await card.getByRole("button", { name: "记下这条发现" }).click();

  // 过程即数据: the confirmed finding accumulates in 阅读成果 and the picked
  // sentence stays highlighted in the article.
  await expect(page.getByRole("tab", { name: "阅读成果 1" })).toBeVisible({ timeout: 30_000 });
  await expect(page.locator(`p[data-block-id="${ownBlock}"] mark`).first()).toBeVisible();
  await page.getByRole("tab", { name: "阅读成果 1" }).click();
  await expect(page.getByRole("heading", { name: "我的阅读成果" })).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);
});

/**
 * D1 (fixed in Task 18) — its own test, on purpose, rather than a branch
 * inside the lens walk above.
 *
 * WHY IT CAN SKIP. The refusal only exists to be triggered when the summon
 * actually grounded an example sentence to click. A graceful-degrade summon
 * renders 「开始选句」 with no example mark in the article, and then nothing
 * on screen can trigger D1 at all. That is a legitimate product path —
 * HangingCard carries its own `hasExample: false` copy for it — so the model
 * being terse must not turn the suite red; a red that means "the model was
 * brief today" only teaches people to ignore red.
 *
 * WHY IT MUST NOT SKIP QUIETLY. If it does skip, D1 went untested, and a
 * maintainer reading CI has to be told. `test.skip()` is what actually tells
 * them: the reporter counts it and prints it in the run summary. The two
 * tempting alternatives do not:
 *   - `test.info().annotations.push(...)` is INERT metadata. Playwright only
 *     special-cases annotations the real test.skip()/test.fixme() APIs create,
 *     and formatTestTitle — which composes every console line for the `list`
 *     and `line` reporters — never prints annotations at all. A skipped probe
 *     would have looked like an ordinary green pass.
 *   - a bare `console.log` does print, but into a stream that (under
 *     run-stack.sh) also carries the API's per-request JSON log. One line in
 *     several hundred is not "visible".
 *
 * WHY IT IS A SEPARATE TEST. `test.skip()` inside a test body aborts that
 * test. Called from inside the lens walk it would take the entire card cycle
 * — pick, evaluate, 记下这条发现, 阅读成果 — down with it, trading the
 * coverage we have for visibility of the coverage we lack.
 */
test("D1: clicking the AI's own example sentence is refused — and says so", async ({ page }) => {
  await startReading(page, titled("D1 走查用的一篇"), ARTICLE_BODY);

  await page.getByRole("button", { name: /^透镜库 · \d+$/ }).click();
  await page.getByRole("dialog", { name: "透镜库" }).locator(".mk-lens-library__row").first().click();

  const card = page.locator(".lens-connector").locator("..");
  await expect(card).toBeVisible({ timeout: 180_000 });

  // 「看懂示范，开始选句」 ⇒ an example was grounded. 「开始选句」 alone ⇒
  // the summon degraded and there is no example mark to click.
  const hasExample = (await card.getByRole("button", { name: "看懂示范，开始选句" }).count()) > 0;
  test.skip(
    !hasExample,
    "This summon grounded no example sentence, so nothing on screen can trigger the D1 refusal — D1 went UNTESTED this run.",
  );

  const anchorBlock = await card.evaluate((el) => {
    const prev = el.previousElementSibling;
    return prev && prev.tagName === "P" ? prev.getAttribute("data-block-id") : null;
  });
  expect(anchorBlock).toMatch(/^b\d+$/);

  await card.getByRole("button", { name: "看懂示范，开始选句" }).click();
  await expect(card.getByText("在文章里点出你自己的证据句")).toBeVisible();

  // Until Task 18 this click did NOTHING — no message, no shake — which made
  // the most natural click on the screen look broken. It is still refused (she
  // has to find her own sentence); it just says so now.
  //
  // The example is the only <mark> in this article: the reading is fresh, so
  // there are no stored annotations and no confirmed outcomes adding marks.
  const exampleMark = page.locator(`p[data-block-id="${anchorBlock}"] mark`).first();
  await expect(exampleMark).toBeVisible();
  await exampleMark.click();
  await expect(card.getByText("这句是示范句——换一句你自己的证据句。")).toBeVisible();

  // Still refused. This does NOT independently prove "no model call was
  // spent": Playwright aborts at the first failing expect, so this line is
  // only reached when the assertion above already passed — and only the
  // refusal path sets the hint, so an accepted click would have failed there
  // first. Kept because it costs nothing and narrows the window in which both
  // could be wrong at once.
  await expect(card.getByRole("button", { name: "记下这条发现" })).toHaveCount(0);

  // Transient, not a lingering error banner: it reverts on its own (2.6s).
  await expect(card.getByText("在文章里点出你自己的证据句")).toBeVisible({ timeout: 10_000 });
});
