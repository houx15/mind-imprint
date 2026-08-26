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
 * 「追来源」 and the longer form appears nowhere in apps/web/src. Every literal
 * below has been grepped and rendered:
 *
 *   证据笔记      ReadingRoom.tsx           (gated on caps.evidenceMap)
 *   追来源        ReadingRoom.tsx           (gated on caps.explorationLeads)
 *   新的线索      FinalizeReadingPanel.tsx  (gated on caps.proposalImpact)
 *   对论点的影响  FinalizeReadingPanel.tsx  (gated on caps.proposalImpact)
 *
 * Only the last two are load-bearing HERE: the first two are ALSO gated on
 * callbacks `ReadingRoomHost` never passes, so they would stay absent even
 * under PRO_CAPABILITIES. The real guard for those lives at the unit level, in
 * apps/lite-web/test/readingRoomCapabilities.test.tsx, which supplies the
 * callbacks and asserts both surfaces PRESENT under PRO_CAPABILITIES and
 * absent under lite. These two lines are belt-and-braces on top of it.
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

  // The room's own surfaces: the coach column, the two view tabs, and 完成这篇.
  await expect(page.getByRole("heading", { name: "换一个视角，再读一遍" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "文章" })).toBeVisible();
  await expect(page.getByRole("tab", { name: /阅读成果/ })).toBeVisible();
  await expect(page.getByRole("button", { name: "完成这篇" })).toBeVisible();
  await expect(page.getByPlaceholder("说说你对哪一句有疑问…")).toBeVisible();

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

  // ── 完成这篇 → 我的收获 → 确认归纳 ─────────────────────────────────────────
  await page.getByRole("button", { name: "完成这篇" }).click();
  const finalize = page.getByRole("dialog", { name: "完成这篇" });
  await expect(finalize.getByRole("heading", { name: "把这篇的阅读成果归纳一下" })).toBeVisible();
  await expect(finalize.getByText("你的阅读记录 · 只读")).toBeVisible();
  // The synthesis half is HERS in lite: 我的收获, with no proposal behind it.
  await expect(finalize.getByText("我的收获")).toBeVisible();
  await expect(finalize.getByText("新的线索")).toHaveCount(0);
  await expect(finalize.getByText("对论点的影响")).toHaveCount(0);

  const takeaway = "增长是真的，但把它外推到下一个十年之前，得先问储能解决了没有。";
  const takeawayBox = finalize.locator("textarea");
  await expect(takeawayBox).toHaveCount(1);
  await takeawayBox.fill(takeaway);
  await finalize.getByRole("button", { name: "确认归纳" }).click();
  await expect(finalize.getByText("已归纳 ✓")).toBeVisible({ timeout: 30_000 });
  await expect(finalize).toBeHidden({ timeout: 15_000 });

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
  await page.getByRole("button", { name: "返回", exact: true }).click();
  await expect(page).toHaveURL(/\/readings$/);
  await expectGreeting(page);

  await page.getByRole("button", { name: /我的阅读/ }).click();
  await expect(page.getByText(/^已完成 · \d+$/)).toBeVisible();
  await expect(page.getByRole("button", { name: new RegExp(`${ARTICLE_TITLE}.*看报告`, "s") })).toBeVisible();
  await page.getByRole("button", { name: new RegExp(`${ARTICLE_TITLE}.*看报告`, "s") }).click();

  await expect(page).toHaveURL(new RegExp(`/readings/${id}$`));
  await expect(page.getByText("已完成", { exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: ARTICLE_TITLE })).toBeVisible();
  await expect(page.getByText("我的收获")).toBeVisible();
  await expect(page.getByText(takeaway)).toBeVisible();
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

  // The room opens with the loop's own greeting; a real reply is the SECOND
  // assistant turn.
  //
  // CAREFUL: the "正在阅读与判断" typing indicator is itself a
  // `.mk-msg--assistant` node, so counting that class alone would go to 2 the
  // instant the request left the browser and pass without any reply at all.
  // The wait is therefore on the indicator CLEARING, which only happens when
  // the turn resolves.
  const assistantTurns = page.locator(".mk-msg--assistant");
  const thinking = page.locator(".mk-msg__thinking");
  await expect(assistantTurns).toHaveCount(1);
  await expect(thinking).toHaveCount(0);

  await page.getByPlaceholder("说说你对哪一句有疑问…").fill("第四段说边际收益会递减，这个推论站得住吗？");
  await page.getByRole("button", { name: "发送" }).click();
  await expect(page.locator(".mk-msg--student")).toHaveCount(1);
  await expect(thinking).toBeVisible();

  // One live model call through the lite gateway. At least two assistant
  // turns, not exactly two: a turn that also proposes a lens adds a second
  // frame ("… 已就绪") after the reply, and that is correct behaviour, not a
  // failure — so the assertion is on the REPLY being real, not on the count.
  await expect(thinking).toHaveCount(0, { timeout: 180_000 });
  expect(await assistantTurns.count()).toBeGreaterThanOrEqual(2);
  const reply = (await assistantTurns.nth(1).innerText()).trim();
  expect(reply.length).toBeGreaterThan(20);
  expect(reply).not.toContain("文章已经准备好了");

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
  //
  // Whether the summon grounded an example decides two things: the proposed
  // button's wording, and whether the D1 probe below is applicable at all. A
  // graceful-degrade summon (the AI could not ground a sentence) renders
  // 「开始选句」 with no example mark in the article, so there is nothing to
  // click to trigger the rejection. Captured here rather than inferred later.
  const hadExample = (await card.getByRole("button", { name: "看懂示范，开始选句" }).count()) > 0;
  await card.getByRole("button", { name: /开始选句$/ }).click();
  await expect(card.getByText("在文章里点出你自己的证据句")).toBeVisible();

  // ── D1 (fixed in Task 18): clicking the AI's OWN underlined example is
  // still refused — she must find her own sentence — but it is no longer
  // refused in silence. This is the one path a student hits first, and until
  // Task 18 it was a dead click with no feedback whatsoever.
  //
  // Stable because: the example is the ONLY <mark> in this reading's article
  // (this test creates no annotations and has confirmed no outcomes yet), the
  // hint's 2.6s auto-clear is far longer than Playwright's polling interval,
  // and the whole probe is skipped — loudly — when the summon returned no
  // example to click.
  if (hadExample) {
    const exampleMark = page.locator(`p[data-block-id="${anchorBlock}"] mark`).first();
    await expect(exampleMark).toBeVisible();
    await exampleMark.click();
    await expect(card.getByText("这句是示范句——换一句你自己的证据句。")).toBeVisible();
    // Refused, not evaluated: the card stays 'active' and never spends a
    // model call on the example. And the hint is transient — it reverts to
    // the ordinary instruction instead of lingering as a stale error.
    await expect(card.getByRole("button", { name: "记下这条发现" })).toHaveCount(0);
    await expect(card.getByText("在文章里点出你自己的证据句")).toBeVisible({ timeout: 10_000 });
  } else {
    test.info().annotations.push({
      type: "skipped",
      description: "D1 probe not applicable: this summon grounded no example sentence to click.",
    });
  }
  // Her pick has to be a paragraph OTHER than the one carrying the AI's
  // example: `useReadingLoop.pickSentence` rejects a span overlapping the
  // example outright (she must choose for herself), and it rejects it
  // SILENTLY — clicking the example looks like a dead click.
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
