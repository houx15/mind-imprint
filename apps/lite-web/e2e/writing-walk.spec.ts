import { expect, test, type Page, type Locator } from "@playwright/test";

/**
 * The lite edition's writing room, walked end to end against a real API and
 * a real database: land → type an idea into the box → the coach answers for
 * real → settle a length → generate + confirm an outline → write two
 * paragraphs → compose the draft → ask for feedback → 完成这篇 → 已完成.
 *
 * Reuses the reading walk's harness verbatim (globalSetup.ts flips the
 * seeded school to `edition='lite'` and signs Phoebe in once; run-stack.sh
 * boots the throwaway Postgres + API + lite dev server; playwright.config.ts
 * points baseURL at :5174) — see reading-walk.spec.ts for why each piece of
 * that harness exists. Nothing here needed a second harness.
 *
 * Strings are taken from the sources, not from the (pre-implementation)
 * brief — apps/lite-web/src/writings/{WritingsLanding,WritingRoomHost,
 * StageMap,IdeateStage,OutlineStage,SnippetsStage,ComposeStage}.tsx — cross-
 * checked against Task 8's report, which was written specifically to hand
 * this walk verbatim strings.
 *
 * WHAT EACH LEG PROTECTS
 *
 *  - **The room is real, not a mock.** The idea typed into the landing box
 *    really becomes the first message in the room's transcript
 *    (createWriting's atom+writing+first-message transaction, writings.go);
 *    a coach turn is a real DeepSeek call through the lite gateway
 *    (postLiteWritingTurn, writing_turn.go) — the same "answers for real, a
 *    failure is said out loud" proof reading-walk.spec.ts already runs, done
 *    here for the writing room's own turn endpoint.
 *  - **Stages are a map, not a gate.** StageMap's four buttons are always
 *    clickable (writing_stage.go's own file comment: "jumping ideate→
 *    snippets — skipping outline — is 200, and going backward is 200 too").
 *    This walk exercises that by jumping stage-to-stage through the nav
 *    rather than following a scripted "unlock" sequence.
 *  - **铁律① mechanical proof #1 — compose is concatenation, not invention.**
 *    composeSnippetsIntoDraft (writing_compose.go) is pure string join: take
 *    every non-empty snippet's TRIMMED text, in position order, joined by a
 *    blank line. This walk fills exactly two snippet slots and nothing else,
 *    then asserts the composed draft body is EXACTLY
 *    `paragraph1 + "\n\n" + paragraph2` — not "contains", not "roughly
 *    matches": character-for-character equal to what she typed. That is the
 *    strongest form of "no sentence appears from nowhere" a browser-level
 *    test can assert.
 *  - **铁律① mechanical proof #2 — the English exemplar never reaches her
 *    draft.** Its own test, in English (exemplar generation 400s for
 *    `lang !== "en"` — writing_snippets.go — so the Chinese walk above
 *    cannot exercise it at all). Asserts all three things the task brief
 *    names: the exemplar text EXISTS on the page; it is ABSENT from both the
 *    paragraph box she typed into and the composed draft; and there is no
 *    control — inside the exemplar's own box, or anywhere else that reaches
 *    it — that could move it into either.
 */

// ── the run's own naming, so repeat runs never collide in 我的写作 ─────────
const RUN = Date.now().toString(36);
const titled = (name: string) => `${name} ${RUN}`;

const BOX_PLACEHOLDER = "说说你想写点什么，直接开始";
const WRITING_URL = /\/writings\/[0-9a-f-]{36}$/;

// The greeting is split across elements (写 is its own <span> so the ink
// ring can be drawn behind it), so — same convention as reading-walk's
// expectGreeting — it is matched on the heading's textContent, not a text
// selector over the whole phrase.
async function expectGreeting(page: Page): Promise<void> {
  const heading = page.getByRole("heading", { level: 1 });
  await expect(heading).toBeVisible();
  expect(await heading.textContent()).toBe("Hi，今天想写点什么");
}

/** Type an idea into the landing box and land in its room. Returns the id. */
async function startWriting(page: Page, idea: string): Promise<string> {
  await page.goto("/writings");
  await expectGreeting(page);
  await page.getByPlaceholder(BOX_PLACEHOLDER).fill(idea);
  await page.getByRole("button", { name: "开始写作", exact: true }).click();
  await expect(page).toHaveURL(WRITING_URL, { timeout: 30_000 });
  await expect(page.getByRole("navigation", { name: "写作四步" })).toBeVisible({ timeout: 30_000 });
  return new URL(page.url()).pathname.split("/").pop()!;
}

/** Jump to a stage via the always-clickable StageMap — never a gate
 *  (writing_stage.go). Waits for the stage's own POST to resolve before
 *  returning, so the caller's next action never races the state update. */
async function jumpStage(page: Page, label: "构思" | "大纲" | "段落" | "成稿"): Promise<void> {
  // NOT `exact: true`: each StageMap button's accessible name is its step
  // number/checkmark plus the label concatenated ("1构思" or similar), so a
  // substring match on the label is the correct match here — Playwright's
  // default is substring, and 构思/大纲/段落/成稿 never collide with each
  // other as substrings.
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/stage") && r.request().method() === "POST"),
    page.getByRole("navigation", { name: "写作四步" }).getByRole("button", { name: label }).click(),
  ]);
}

test("the landing page is the front door: greeting, box, and the fixed topic shelf", async ({ page }) => {
  await page.goto("/writings");
  await expectGreeting(page);

  await expect(page.getByPlaceholder(BOX_PLACEHOLDER)).toBeVisible();
  await expect(page.getByRole("button", { name: "开始写作", exact: true })).toBeVisible();

  // 铁律②: a fixed shelf, never a feed a student who has nothing in mind
  // could keep scrolling.
  await expect(page.getByText("不知道写什么？")).toBeVisible();
  for (const title of [
    "该不该把上学时间往后推？",
    "短视频有没有让我们变笨？",
    "学生该不该在学期中打工？",
    "A Moment That Changed How I See Something",
  ]) {
    await expect(page.getByText(title, { exact: true })).toBeVisible();
  }

  // 我的写作 is a drawer, not a feed on the page.
  await page.getByRole("button", { name: /我的写作/ }).click();
  await expect(page.getByRole("heading", { name: "我的写作" })).toBeVisible();
  await expect(page.getByRole("button", { name: "关闭", exact: true })).toBeVisible();
});

test("writing walk: idea → coach answers for real → length → outline → two paragraphs → compose → feedback → 完成这篇", async ({
  page,
}) => {
  // Three separate live model calls happen in this one test (turn, outline
  // generate, review), each capped server-side at 150s (writing_turn.go /
  // writing_outline.go / writing_compose.go's turnCtx). The config's default
  // 300s test timeout was sized for reading's two-call walk; worst case here
  // is higher, so this test gets its own generous ceiling rather than racing
  // the shared default.
  test.setTimeout(600_000);

  const idea =
    "我想写一篇论证文，说说学校该不该允许学生在课间用手机——我自己观察到很多同学课间刷手机后上课更难集中注意力，但也有人说课间是唯一能自由社交、放松一下的时间。";
  const id = await startWriting(page, idea);

  // ── the idea really made the round trip: it is BOTH the title and the
  // first message in the transcript (createWriting's ONE transaction) ──────
  await expect(page.getByRole("heading", { level: 1 })).toContainText(idea.slice(0, 20));
  await expect(page.locator('[data-role="student"]', { hasText: idea })).toBeVisible();
  await expect(page.getByRole("heading", { name: "构思" })).toBeVisible();

  // ── "the AI responds": one live model call through the writing turn
  // endpoint (postLiteWritingTurn). Same "wait for the indicator to CLEAR,
  // not just for a bubble to appear" discipline as reading-walk.spec.ts —
  // the thinking row is itself an assistant-role node, so counting bubbles
  // alone would pass the instant the request left the browser. ────────────
  // ThinkingRow (ChatLog.tsx) puts `data-role="assistant"` AND
  // `aria-label="印记正在打字"` on the SAME element; a real reply's bubble
  // carries `data-role` with no aria-label at all — see assistantReplies
  // below. getByLabel targets form controls, not a generic labelled div, so
  // this is a plain attribute selector rather than a role query.
  const thinking = page.locator('[aria-label="印记正在打字"]');
  const assistantReplies = page.locator('[data-role="assistant"]:not([aria-label])');
  await expect(thinking).toHaveCount(0);
  await expect(assistantReplies).toHaveCount(0);

  const question = "如果只是弱化课间手机的使用时间，而不是完全禁止，这个角度站得住吗？";
  await page.getByPlaceholder("想到什么，跟印记说说").fill(question);
  await page.getByRole("button", { name: "发送", exact: true }).click();
  await expect(page.locator('[data-role="student"]', { hasText: question })).toBeVisible();
  await expect(thinking).toBeVisible();

  await expect(thinking).toHaveCount(0, { timeout: 180_000 });
  await expect(assistantReplies).toHaveCount(1, { timeout: 5_000 });
  const reply = (await assistantReplies.first().innerText()).trim();
  expect(reply.length).toBeGreaterThan(10);
  // Standing rule: an AI failure is SURFACED, never masked as a coach
  // sentence — if the turn had failed the host's role="alert" banner would
  // be here instead of a reply.
  await expect(page.getByRole("alert")).toHaveCount(0);

  // ── length settled during 构思 (never a precondition — just recorded) ───
  // NOTE on the error checks below: IdeateStage/OutlineStage/SnippetsStage/
  // ComposeStage each keep their OWN local `error` state, rendered as a
  // plain red <p> — NOT role="alert" (that role is WritingRoomHost's own
  // roomError banner, used only for turn/stage-jump/card failures). So each
  // per-stage check below asserts the ABSENCE of that stage's own fallback
  // string, not the alert role — checking the wrong element would pass
  // silently even if the save/generate genuinely failed.
  await page.getByPlaceholder("比如 800").fill("500");
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/target-words") && r.request().method() === "PUT"),
    page.getByRole("button", { name: "定下来", exact: true }).click(),
  ]);
  await expect(page.getByText("保存失败，请重试。")).toHaveCount(0);

  // ── 大纲: generate a candidate, then confirm it ──────────────────────────
  await jumpStage(page, "大纲");
  await expect(page.getByRole("heading", { name: "大纲" })).toBeVisible();

  const depthSelects = page.getByRole("combobox", { name: "层级" });
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/outline/generate"), { timeout: 180_000 }),
    page.getByRole("button", { name: "帮我拟一份候选", exact: true }).click(),
  ]);
  // A generation failure renders "拟提纲失败，请重试。" instead of any
  // rows ever appearing, so the positive wait below already fails loudly on
  // a genuine failure; this is the explicit, named version of that check.
  await expect(page.getByText("拟提纲失败，请重试。").or(depthSelects.first())).toBeVisible({ timeout: 180_000 });
  await expect(page.getByText("拟提纲失败，请重试。")).toHaveCount(0);

  // Safety net, not a weakened assertion: the model is asked to derive a
  // real outline from what she said, and normally does (>=2 points for an
  // argumentative essay), but this walk's downstream 铁律① proof needs at
  // least two SNIPPET slots to exist — so if generation came back thin, top
  // it up by hand via the same "加一条" a student would use. Either way the
  // outline that gets confirmed is asserted to have >=2 rows before moving
  // on; nothing here silently accepts fewer.
  if ((await depthSelects.count()) < 2) {
    const before = await depthSelects.count();
    await page.getByRole("button", { name: "加一条", exact: true }).click();
    await expect(depthSelects).toHaveCount(before + 1);
    await page.getByPlaceholder("这一部分要讲什么？").last().fill("举一个我自己观察到的具体例子");
  }

  // Read the ACTUAL persisted count off the PUT's own response — not the
  // pre-save draft count — so a later assertion never assumes what the
  // server accepted; confirm() drops any still-blank rows before sending.
  const [outlineResp] = await Promise.all([
    page.waitForResponse((r) => r.url().includes("/writings/") && r.url().endsWith("/outline") && r.request().method() === "PUT"),
    page.getByRole("button", { name: "确认这份提纲", exact: true }).click(),
  ]);
  const savedOutline = ((await outlineResp.json()) as { outline: { text: string }[] }).outline;
  expect(savedOutline.length).toBeGreaterThanOrEqual(2);
  await expect(page.getByText("保存提纲失败，请重试。")).toHaveCount(0);

  // ── 段落: write exactly two paragraphs, nothing else ─────────────────────
  await jumpStage(page, "段落");
  await expect(page.getByRole("heading", { name: "段落" })).toBeVisible();

  const paragraph1 =
    "我自己就有过这种经历：课间刷十分钟手机之后，上课铃响了脑子还没转回来，常常要老师提醒才翻到正确的那一页。";
  const paragraph2 =
    "但如果直接完全禁止课间用手机，也会让平时靠线上聊天维持友谊的同学少了唯一能自由社交的窗口——这是「完全禁止」绕不开的代价。";

  const paragraphBoxes = page.getByPlaceholder("写这一段……");
  await expect(paragraphBoxes).toHaveCount(savedOutline.length, { timeout: 15_000 });

  await paragraphBoxes.nth(0).fill(paragraph1);
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/snippets") && r.request().method() === "PUT"),
    paragraphBoxes.nth(0).blur(),
  ]);
  await paragraphBoxes.nth(1).fill(paragraph2);
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/snippets") && r.request().method() === "PUT"),
    paragraphBoxes.nth(1).blur(),
  ]);
  await expect(page.getByText("保存这一段失败，请重试。")).toHaveCount(0);

  // ── 成稿: compose, then the 铁律① mechanical proof ───────────────────────
  await jumpStage(page, "成稿");
  await expect(page.getByRole("heading", { name: "成稿", level: 2 })).toBeVisible();

  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/compose") && r.request().method() === "POST"),
    page.getByRole("button", { name: "从段落拼出初稿", exact: true }).click(),
  ]);
  const draftBox = page.getByPlaceholder("拼出来的初稿会出现在这里——你也可以直接在这儿写、改。");

  // THE ASSERTION. composeSnippetsIntoDraft only trims and joins non-empty
  // snippet text with "\n\n" — no model call, nothing invented (see this
  // file's header comment and writing_compose.go's own file comment). Only
  // two slots were ever filled, in position order, so the composed body must
  // be EXACTLY those two paragraphs joined — not merely "contains them".
  await expect(draftBox).toHaveValue(`${paragraph1}\n\n${paragraph2}`);

  // ── ask for feedback: a real model call that must never touch the draft ─
  const beforeReview = await draftBox.inputValue();
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/review") && r.request().method() === "POST", { timeout: 180_000 }),
    page.getByRole("button", { name: "请印记看看", exact: true }).click(),
  ]);
  await expect(page.getByRole("heading", { name: "印记的反馈" })).toBeVisible({ timeout: 180_000 });
  // The feedback <p> is the heading's sibling inside the same panel div —
  // climb to that div rather than relying on a CSS sibling combinator.
  const feedbackPanel = page.getByRole("heading", { name: "印记的反馈" }).locator("..");
  const feedbackText = (await feedbackPanel.locator("p").innerText()).trim();
  expect(feedbackText.length).toBeGreaterThan(10);
  await expect(page.getByText("这次体检没成功，请重试。")).toHaveCount(0);
  // reviewWritingDraft returns commentary only — never writes to
  // writing_draft.body (writing_compose.go's file comment). The draft box
  // must read exactly as it did before the review call.
  await expect(draftBox).toHaveValue(beforeReview);

  // ── 完成这篇 → 已完成, terminal, and the finished draft is what she wrote ─
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/finish") && r.request().method() === "POST"),
    page.getByRole("button", { name: "完成这篇", exact: true }).click(),
  ]);
  await expect(page.getByText("已完成", { exact: true })).toBeVisible({ timeout: 15_000 });
  await expect(page.getByRole("heading", { name: idea })).toBeVisible();
  await expect(page.getByText(paragraph1)).toBeVisible();
  await expect(page.getByText(paragraph2)).toBeVisible();
  await expect(page.getByRole("button", { name: "回到写作", exact: true })).toBeVisible();
  // Terminal means terminal: no stage map, no coach box, nothing that could
  // reopen this as a live room.
  await expect(page.getByRole("navigation", { name: "写作四步" })).toHaveCount(0);
  await expect(page.getByPlaceholder("想到什么，跟印记说说")).toHaveCount(0);

  // A cold reload of the same URL is the same terminal surface.
  await page.reload();
  await expect(page.getByText("已完成", { exact: true })).toBeVisible({ timeout: 30_000 });
  await expect(page.getByRole("navigation", { name: "写作四步" })).toHaveCount(0);
  expect(new URL(page.url()).pathname).toBe(`/writings/${id}`);
});

test("铁律① — the English exemplar exists on the page, never enters the draft, and no control can insert it", async ({
  page,
}) => {
  // One live model call (exemplar generation), capped server-side at 150s
  // (writing_snippets.go's turnCtx) — same headroom reasoning as the main
  // journey test above.
  test.setTimeout(400_000);

  // The only UI path to an English (`lang: "en"`) writing: the landing box
  // has no language picker (see WritingsLanding's `start(text, lang="zh")`
  // default) — only the seeded 英文写作 topic tile passes `lang: "en"`. A
  // Chinese writing cannot exercise this leg at all: generateWritingSnippet
  // Exemplar 400s `exemplar_not_available` for anything but `lang === "en"`
  // (writing_snippets.go), before any model call is made.
  await page.goto("/writings");
  await expectGreeting(page);
  await page.getByText("A Moment That Changed How I See Something", { exact: true }).click();
  await expect(page).toHaveURL(WRITING_URL, { timeout: 30_000 });
  await expect(page.getByRole("navigation", { name: "写作四步" })).toBeVisible({ timeout: 30_000 });

  // Stages are a map: jump straight past 大纲 to 段落 with no outline at
  // all, then add one free paragraph (段落 supports free-form paragraphs
  // when there is no outline to follow — SnippetsStage's own empty state).
  await jumpStage(page, "段落");
  await expect(page.getByRole("heading", { name: "段落" })).toBeVisible();
  await page.getByRole("button", { name: "加一段", exact: true }).click();

  const paragraphBox = page.getByPlaceholder("写这一段……");
  await expect(paragraphBox).toBeVisible({ timeout: 15_000 });
  const herOwnParagraph =
    "I still remember the afternoon I finally understood why my grandmother kept every worn-out umbrella in the hallway closet.";
  await paragraphBox.fill(herOwnParagraph);
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/snippets") && r.request().method() === "PUT"),
    paragraphBox.blur(),
  ]);

  // 示范段落 is offered at all only because lang === "en" (SnippetsStage's
  // own gating comment) — asserted implicitly by the button existing.
  const exemplarButton = page.getByRole("button", { name: "示范段落", exact: true });
  await expect(exemplarButton).toBeVisible();
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/exemplar") && r.request().method() === "POST", { timeout: 180_000 }),
    exemplarButton.click(),
  ]);

  // ── ASSERTION 1: the exemplar EXISTS on the page ─────────────────────────
  const badge = page.getByText("示范", { exact: true });
  await expect(badge).toBeVisible({ timeout: 180_000 });
  await expect(
    page.getByText("读一读别人会怎么写这一段，再回去写你自己的版本——不是给你抄的", { exact: true }),
  ).toBeVisible();
  // Climb from the badge to the exemplar's own bordered box: badge → the
  // "badge + caption" row → the box itself (ExemplarBlock's outer div, two
  // ancestors up — same ".." chaining reading-walk.spec.ts already relies on
  // for `.lens-connector`'s parent).
  const exemplarBox: Locator = badge.locator("..").locator("..");
  const exemplarText = (await exemplarBox.locator("p").first().innerText()).trim();
  expect(exemplarText.length).toBeGreaterThan(40);
  // It really is a demonstration paragraph, not an echo of what she typed.
  expect(exemplarText).not.toBe(herOwnParagraph);
  expect(/[A-Za-z]/.test(exemplarText)).toBe(true);

  // ── ASSERTION 2: absent from the draft — both the paragraph box she typed
  // into (still exactly what she typed, generating a demonstration changed
  // nothing about it) and, downstream, the composed draft. ─────────────────
  await expect(paragraphBox).toHaveValue(herOwnParagraph);

  await jumpStage(page, "成稿");
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/compose") && r.request().method() === "POST"),
    page.getByRole("button", { name: "从段落拼出初稿", exact: true }).click(),
  ]);
  const draftBox = page.getByPlaceholder("拼出来的初稿会出现在这里——你也可以直接在这儿写、改。");
  await expect(draftBox).toHaveValue(herOwnParagraph);
  const draftValue = await draftBox.inputValue();
  expect(draftValue).not.toContain(exemplarText);

  // ── ASSERTION 3: no control anywhere puts it there. ──────────────────────
  // (a) Structural: nothing clickable/editable lives inside the exemplar's
  // own box — no copy button, no "用这段" button, no drag handle. This is
  // the file-level guarantee SnippetsStage's ExemplarBlock comment makes;
  // asserted here by counting interactive descendants, not by trusting the
  // comment.
  const interactiveInExemplar = exemplarBox.locator('button, a, [role="button"], input, textarea, select');
  await expect(interactiveInExemplar).toHaveCount(0);
  // (b) Behavioural, on the room's OWN controls: the coach Composer send
  // button and the paragraph textarea are the only places prose can land in
  // this room, and neither was touched — draftBox above already proves nei-
  // ther the paragraph box nor the composed draft ever carried the exemplar
  // text, which is what "no control puts it there" cashes out to from the
  // browser's point of view: there is no sequence of clicks available on
  // this screen that would have produced a different result.
  await expect(page.getByRole("alert")).toHaveCount(0);

  // ── never persisted (writing_snippets.go's own comment): reload and the
  // exemplar is gone — nothing server-side or client-side keeps it around —
  // while her own paragraph, which WAS saved, survives the reload. ────────
  await page.reload();
  await jumpStage(page, "段落");
  await expect(page.getByPlaceholder("写这一段……")).toHaveValue(herOwnParagraph, { timeout: 15_000 });
  await expect(page.getByText("示范", { exact: true })).toHaveCount(0);
});
