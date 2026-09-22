import { expect, test, type Page } from "@playwright/test";
// 🚨 阅读室聊天框的占位符有**四种**，看她当下站在哪儿（ReadingCoachPanel）：
//   透镜开着 → 「找不到合适的句子？跟印记说一声」
//   读完了   → 「读完了，还想聊点什么？」
//   卡片开着 → 「卡片以外的问题，请在这里输入」
//   其余     → 「请输入你的回答或问题」
// 原来钉的「读完这一步」那一句早就不在了，只认一种也会在另外三种情况下
// 去等一个根本不存在的框，然后报成「输入框锁住了」。

/**
 * 带读 is the room's front door now, and this file walks it.
 *
 * The two rulings it protects (see the Amendment section of
 * docs/superpowers/specs/2026-08-27-lite-reading-room-scaffold-design.md):
 *
 *  - **印记 leads; she does not manage stages.** So the assertions here are
 *    the negative ones as much as the positive: no 做完了, no 跳过, and nothing
 *    in the progress list is clickable. An edit that "helpfully" gives her
 *    back a checkbox fails here.
 *  - **The paragraph tools are instruments, not a permanent shelf.** The
 *    paragraph itself is the control — clicking one raises a bar of tools at
 *    her pointer, carrying the language-neutral 想一想 / 仿写 alongside the
 *    explainers for the ARTICLE's own language.
 *
 * It lives beside reading-walk.spec.ts rather than inside it because that file
 * is one continuous journey (land → read → 完成 → 已完成) and this is a
 * different entrance to the same room.
 */

const RUN = Date.now().toString(36);
const titled = (name: string) => `${name} ${RUN}`;

// Chinese, four paragraphs — so the panel must offer 成语修辞 and never 翻译.
// Blank lines matter: the server splits blocks on them.
const ARTICLE_BODY = [
  "过去十年，全球太阳能装机容量增长了大约十倍。推动这件事的不是某一项突破性发明，而是制造规模、供应链和融资成本三件事同时变便宜。",
  "成本下降的幅度常被单独拎出来当作结论：组件价格在这十年里下降了八成以上。但价格只是发电成本的一部分，土地、并网、运维和资金成本在不同国家差别极大。",
  "真正的瓶颈已经从「发电贵不贵」转移到「电什么时候来」。太阳能的出力集中在正午前后，而用电高峰往往在傍晚，两者之间的错位要靠储能、需求响应或跨区输电来填。",
  "所以，如果储能和电网的问题不解决，继续增加装机带来的边际收益会递减：白天多出来的电卖不掉，甚至要被弃掉。",
].join("\n\n");

// 🚨 2026-09-21 订正：占位符后来加了 TXT、顺序也换了（ReadingsLanding.tsx）。
// 钉整串等于把一句会改的文案当成契约，只钉不会变的那一截。
const BODY_PLACEHOLDER = "贴一个链接，或者把整篇正文粘进来";
const TITLE_PLACEHOLDER = "给这次阅读起个名字（可留空）";
const READING_URL = /\/readings\/[0-9a-f-]{36}$/;

async function startReading(page: Page, title: string): Promise<void> {
  await page.goto("/readings");
  await page.getByPlaceholder(TITLE_PLACEHOLDER).fill(title);
  await page.getByPlaceholder(BODY_PLACEHOLDER).fill(ARTICLE_BODY);
  await page.getByRole("button", { name: "开始阅读" }).click();
  await expect(page).toHaveURL(READING_URL, { timeout: 30_000 });
  await expect(page.locator(".mk-reading-room")).toBeVisible({ timeout: 30_000 });
}

/**
 * One live model call (the plan turn), which is why this asserts on the SHAPE
 * of what came back — a numbered route and a first instruction — and never on
 * particular sentences. The model is free to phrase it its own way; it is not
 * free to hand her a checklist to work.
 */
test("带读: 印记 plans the route and leads, and she administrates none of it", async ({ page }) => {
  await startReading(page, titled("带读走查"));

  // The invitation, before anything has been spent.
  // 🚨 这里原来钉的是那句邀请语（「让我来带你详细读一遍这篇文章。」）。
  // 2026-09-22 的文案改版把它换成了一个名词标题「阅读引导」——界面文案规则 1
  // （标签是名词，不是句子），改得对。
  // 钉不变的那一截：**那块邀请还在，而且它带着一颗「开始」**。
  await expect(page.getByText("阅读引导")).toBeVisible();
  const start = page.getByRole("button", { name: "开始", exact: true });
  await expect(start).toBeVisible();

  await start.click();

  // The plan turn is a real model call on the chaperone tier. The plan lands
  // on the floating dial, which is FOLDED — 3/5 in a ring in the corner — so
  // what proves the plan arrived is the disc appearing, not the list.
  const dial = page.locator(".mk-plandial__disc");
  await expect(dial).toBeVisible({ timeout: 120_000 });
  // Folded, it says where she is only to a screen reader.
  await expect(dial).toHaveAttribute("aria-label", /带读进度 · 第 \d+ 步 \/ 共 \d+ 步/);

  // Hover unfolds it. A route of more than a couple of steps, and a first
  // instruction to act on. Scoped to the panel, not a bare `ol` — an unscoped
  // list selector would stay green off any other list in the room if this one
  // disappeared, which is exactly the regression the next three assertions
  // are for.
  await dial.hover();
  const progress = page.locator(".mk-plandial__panel");
  await expect(progress.getByText("带读进度")).toBeVisible();
  const steps = progress.locator("ol > li");
  expect(await steps.count()).toBeGreaterThan(2);

  // Ruling 1: progress, not controls. She has no way to mark her own step
  // done, and no step is clickable — 印记 moves her.
  await expect(page.getByRole("button", { name: "做完了" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "跳过" })).toHaveCount(0);
  await expect(steps.locator("button")).toHaveCount(0);

  // Skipping did not disappear, it moved into language: the composer is the
  // only control, and it invites her to answer rather than to administrate.
  await expect(page.getByPlaceholder(/请输入你的回答或问题|跟印记说一声|还想聊点什么|请在这里输入/)).toBeVisible();
});

/**
 * Deliberately no model call: this asserts the instruments are REACHABLE and
 * correctly scoped, which is the part a refactor breaks. Whether the coach
 * reaches for one is a model decision, and belongs in the unit tests where it
 * can be made deterministic (reading_coach_test.go).
 */
test("clicking a paragraph raises its tools, carrying 想一想 and 仿写", async ({ page }) => {
  await startReading(page, titled("段落工具走查"));

  // Nothing is parked under the paragraphs — the old 详细带读 button was never
  // found, and a control repeated under all four read as clutter.
  await expect(page.getByRole("button", { name: "详细带读" })).toHaveCount(0);
  const bar = page.getByRole("toolbar", { name: "这一段可以怎么拆" });
  await expect(bar).toHaveCount(0);

  await page.locator("p[data-block-id]").nth(1).click();
  await expect(bar).toBeVisible();

  // Chinese article ⇒ the Chinese explainers, never the English ones. The
  // toolset is derived from the article's own characters server-side, so a
  // failure here means the language detection drifted.
  await expect(bar.getByRole("button", { name: "成语修辞" })).toBeVisible();
  await expect(bar.getByRole("button", { name: "翻译" })).toHaveCount(0);

  // Language-neutral, so present on every article whatever its language.
  await expect(bar.getByRole("button", { name: "想一想" })).toBeVisible();
  await expect(bar.getByRole("button", { name: "仿写" })).toBeVisible();

  // The paragraph is not reprinted anywhere: she is looking straight at it,
  // and a copy of it in a card underneath was the duplication this replaced.
  const paragraph = (await page.locator("p[data-block-id]").nth(1).innerText()).trim();
  expect(await page.getByText(paragraph, { exact: false }).count()).toBe(1);

  // Clicking the same paragraph again puts the bar away.
  await page.locator("p[data-block-id]").nth(1).click();
  await expect(bar).toHaveCount(0);
});

/**
 * Programmatic drag-select: Annotate's `onReferenceSelection` (fine-grained
 * quoting, apps/web/src/primitives/annotate/Annotate.tsx) reads a real,
 * non-collapsed `window.getSelection()` on `mouseup` — there is no button for
 * this, a student does it by dragging her cursor across a sentence. A
 * Playwright mouse drag over CJK text is unreliable (no word boundaries to
 * land on), so this builds the same end state directly: a `Range` over the
 * quote's own text node, installed as the live selection, followed by a real
 * `mouseup` DOM event (bubbles, so Annotate's handler on the wrapping `<div>`
 * still fires) — the exact shape `selectionToSpan` reads either way.
 */
async function selectQuoteInBlock(page: Page, blockId: string, quote: string): Promise<void> {
  const found = await page.evaluate(
    ({ blockId, quote }) => {
      const p = document.querySelector(`p[data-block-id="${blockId}"]`);
      if (!p) return false;
      const walker = document.createTreeWalker(p, NodeFilter.SHOW_TEXT);
      let range: Range | null = null;
      let node: Text | null;
      while ((node = walker.nextNode() as Text | null)) {
        const idx = node.data.indexOf(quote);
        if (idx !== -1) {
          range = document.createRange();
          range.setStart(node, idx);
          range.setEnd(node, idx + quote.length);
          break;
        }
      }
      if (!range) return false;
      const sel = window.getSelection();
      sel?.removeAllRanges();
      sel?.addRange(range);
      p.dispatchEvent(new MouseEvent("mouseup", { bubbles: true }));
      return true;
    },
    { blockId, quote },
  );
  if (!found) throw new Error(`could not select "${quote}" inside block ${blockId}`);
}

/**
 * The sub-project's centrepiece: 印记 doesn't just say which paragraph it
 * means, it hangs the lens THERE. The AI turn is mocked (page.route) for a
 * deterministic aim — the live version of this same hand-off (a real model
 * call grounding a real example) is already walked for real in
 * reading-walk.spec.ts's "a summoned lens hangs under a paragraph and
 * becomes a finding"; this test is about the paragraph-naming wiring, not
 * the model's judgement.
 */
test("带读 hands her a lens aimed at the paragraph it just named", async ({ page }) => {
  const readingIdFrom = (url: string) => new URL(url).pathname.match(/\/readings\/([^/]+)\//)?.[1] ?? "";

  // A real substring of paragraph 2 (b2) — so the mark it becomes renders
  // exactly where "aimed at the paragraph it just named" claims it will.
  const paragraph2 = ARTICLE_BODY.split("\n\n")[1]!;
  const quote = "组件价格在这十年里下降了八成以上";
  const start = paragraph2.indexOf(quote);
  expect(start).toBeGreaterThan(-1);

  // Anchor field names are the shared `Anchor` contract's own (snake_case) —
  // distinct from the camelCase wire shape of the card DTO around it, and
  // that mismatch is real (apps/api/internal/agent/anchors.go vs
  // apps/api/internal/api/reading_cards.go's cardDTO), not a typo here.
  const card = (readingId: string) => ({
    id: "mock-card-1",
    cardId: "craap",
    blockId: "b2",
    status: "proposed",
    origin: "router",
    anchors: [
      {
        id: "mock-anchor-1",
        material_id: readingId,
        block_id: "b2",
        start,
        end: start + quote.length,
        quote,
        dimension: "",
        author: "ai",
        question: "这句给了个具体百分比，但没说是哪国的数字——查一下来源。",
        answer: "",
      },
    ],
    fieldValues: {},
    eventTrace: [],
    framework: {},
    createdAt: new Date().toISOString(),
    submittedAt: null,
  });

  // Before the coach turn below fires there is no card yet — the room's
  // mount-time "resume an open card" check (useReadingLoop) hits this same
  // GET, so it must answer honestly with nothing until the turn has minted
  // one, or the card would appear before 开始 is even clicked.
  let minted = false;
  await page.route("**/api/v1/readings/*/coach", async (route) => {
    minted = true;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        reply: "第二段这个百分比没说是哪国的数据，值得先查一下来源。",
        tasks: [],
        currentTaskId: "",
        focusBlock: "b2",
        tool: "",
        finished: false,
        nudge: "这句缺出处，用 CRAAP 查一下来源再往下读。",
        card: card(readingIdFrom(route.request().url())),
      }),
    });
  });
  await page.route("**/api/v1/readings/*/cards", async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ cards: minted ? [card(readingIdFrom(route.request().url()))] : [] }),
    });
  });

  await startReading(page, titled("透镜命中走查"));
  await page.getByRole("button", { name: "开始", exact: true }).click();

  const hangingCard = page.locator(".lens-connector").locator("..");
  await expect(hangingCard).toBeVisible({ timeout: 30_000 });

  // PARAGRAPH-ANCHORED: the card renders immediately after b2's own <p> —
  // the same check reading-walk.spec.ts's lens walk makes, here on the
  // paragraph the COACH (not the student) named.
  const anchorBlock = await hangingCard.evaluate((el) => {
    const prev = el.previousElementSibling;
    return prev && prev.tagName === "P" ? prev.getAttribute("data-block-id") : null;
  });
  expect(anchorBlock).toBe("b2");

  // The 示范 sentence itself is rendered inside that same paragraph.
  const exampleMark = page.locator('p[data-block-id="b2"] mark', { hasText: quote });
  await expect(exampleMark).toBeVisible();
});

/**
 * A 找一找 (hunt) step settles on a POINT, not a typed description — the
 * plan is arranged directly (page.route on GET /plan and /messages) rather
 * than walked there through a live model call, since what this test is
 * proving is the wiring from a pending "hunt" step through to the
 * structured `picks` field, not the coach's own routing judgement.
 */
test("a hunt step is answered by clicking a paragraph", async ({ page }) => {
  const huntTask = {
    id: "task-hunt-1",
    position: 3,
    kind: "hunt",
    label: "找一找：这个百分比说的是哪国的数字？",
    detail: "在文章里点出能回答这个问题的那一句。",
    blockId: "",
    status: "pending",
    completedAt: null,
  };

  await page.route("**/api/v1/readings/*/plan", async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ routineKey: "zh-scan-focus-lens", routineName: "扫读定位透镜", tasks: [huntTask] }),
    });
  });
  // 带读 has to already be STARTED for the room to render its log —
  // ReadingCoachPanel shows the 开始 invitation until `messages` is non-empty.
  await page.route("**/api/v1/readings/*/messages", async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        messages: [
          { seq: 1, role: "ai", content: "先看看这句是不是点名了国家。", createdAt: new Date().toISOString() },
        ],
      }),
    });
  });

  // 🚨 装在一个对象里，不用裸的 let。
  //
  // 裸 let 只在 route 回调里赋值，TypeScript 的控制流分析看不到那次赋值发生在读
  // 之前，于是把它收窄回初始值 null，最后 `capturedBody?.picks` 报的是
  // 「Property 'picks' does not exist on type 'never'」——一句和真实问题毫无关系
  // 的错。放进对象属性里，`await` 之后的读取不会被收窄成初始值。
  const captured: { body: { text: string; picks: { blockId: string; quote: string }[] } | null } = {
    body: null,
  };
  await page.route("**/api/v1/readings/*/coach", async (route) => {
    captured.body = route.request().postDataJSON();
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        reply: "找到了，这句确实点名了国家。",
        tasks: [{ ...huntTask, status: "done", completedAt: new Date().toISOString() }],
        currentTaskId: "",
        focusBlock: "",
        tool: "",
        finished: false,
        card: null,
        nudge: "",
      }),
    });
  });

  await startReading(page, titled("找一找走查"));

  // R4 (4): the panel no longer carries a pointing instruction of its own —
  // that line belongs to the pick_in_article card, so it is collected when the
  // card is answered instead of lingering above the composer. What must be on
  // screen here is the coach's own turn; the gesture is proved by the chip.
  await expect(page.getByText("先看看这句是不是点名了国家。")).toBeVisible();
  await expect(page.getByText("在文章里点出那一句")).toHaveCount(0);

  const quote = "组件价格在这十年里下降了八成以上";
  await selectQuoteInBlock(page, "b2", quote);

  // 🚨 划一下**不再自己做事**（2026-09-22）。同事：「一划线句子就被收到右下角，
  // 还得一个个删除」—— 她划一句常常只是为了读顺一点。划选现在只标出对哪几个字，
  // 做什么全在工具条上（摘抄 / 放入对话框 / 查词 / 语法）。
  //
  // 所以这一条要多按一下那颗按钮。这不是把判据放宽：要证明的仍然是
  // 「一个 POINT 成立了」，只是那个动作现在由她自己发起。
  await page.getByRole("button", { name: "放入对话框" }).click();

  // The chip is the proof a POINT was made, distinct from her typed words.
  await expect(page.getByText(`“${quote}”`)).toBeVisible();

  await page.getByPlaceholder(/请输入你的回答或问题|跟印记说一声|还想聊点什么|请在这里输入/).fill("这句提到具体国家了吗？");
  // 🚨 exact：划选工具条那一轮之后，页面上还有一颗「发送 1 处引文 →」，
  // 裸名字会 strict mode 撞车。
  await page.getByRole("button", { name: "发送", exact: true }).click();

  await expect.poll(() => captured.body).not.toBeNull();
  // The structured field, not just the inlined blockquote in `text`.
  expect(captured.body?.picks?.[0]).toEqual({ blockId: "b2", quote });
});
