import { expect, test, type Page, type Locator } from "@playwright/test";

/**
 * The lite edition's writing room, walked end to end against a real API and a
 * real database: land → type an idea → the 设定 dialog → 印记 speaks first →
 * pick a structure from the fixed library → fill blocks with her own points →
 * ask the guiding box for help → write two paragraphs → compose → feedback →
 * 完成这篇 → 已完成.
 *
 * Reuses the reading walk's harness verbatim (globalSetup.ts flips the seeded
 * school to `edition='lite'` and signs Phoebe in once; run-stack.sh boots the
 * throwaway Postgres + API + lite dev server; playwright.config.ts points
 * baseURL at :5174) — see reading-walk.spec.ts for why each piece exists.
 *
 * Strings come from the sources, not from a brief:
 * apps/lite-web/src/writings/{WritingsLanding,WritingRoomHost,StageMap,
 * WritingSetupModal,StructureStage,SnippetsStage,ComposeStage,GuideBox}.tsx.
 *
 * REWRITTEN 2026-08-27 for the scaffold redesign. What changed and why it
 * matters to this walk:
 *   - 构思 is gone; the map is three steps (结构/段落/成稿).
 *   - A 设定 dialog gates the room on first open, and is where length is set
 *     (it used to be a dead box on the 构思 page).
 *   - 印记 speaks FIRST, so the room is not silent on arrival.
 *   - 「帮我拟一份候选」 is DELETED. The AI may not author an outline; it may
 *     only pick one skeleton out of a fixed generic library.
 *   - The 工具卡 are gone from this room entirely.
 *
 * WHAT EACH LEG PROTECTS
 *
 *  - **The room is real, not a mock.** The idea typed into the landing box
 *    really becomes the first message in the transcript (createWriting's
 *    atom+writing+first-message transaction, writings.go); the opening line,
 *    the coach turn, the structure recommendation and the review are all real
 *    model calls through the lite gateway.
 *  - **铁律 proof #1 — the AI never authors her outline.** After a structure
 *    is applied, EVERY block's input is asserted EMPTY. The skeleton
 *    contributes labels ("反方最强的说法") and not one word of content. This
 *    is the browser-level version of applyWritingStructure writing `text=""`.
 *  - **铁律 proof #2 — guidance is questions, never sentences.** The guiding
 *    box's every line must end in a question mark, and asking for it must
 *    leave her own field byte-identical.
 *  - **铁律 proof #3 — compose is concatenation, not invention.**
 *    composeSnippetsIntoDraft (writing_compose.go) is a pure string join, so
 *    the composed body is asserted EXACTLY equal to the two paragraphs she
 *    typed — not "contains", not "roughly matches".
 *  - **铁律 proof #4 — the English exemplar never reaches her draft**, and no
 *    control anywhere could put it there.
 *  - **Stages are a map, not a gate.** Every step is always clickable.
 */

// ── the run's own naming, so repeat runs never collide in 我的写作 ─────────
const RUN = Date.now().toString(36);
const titled = (name: string) => `${name} ${RUN}`;

const BOX_PLACEHOLDER = "说说你想写点什么，直接开始";
const WRITING_URL = /\/writings\/[0-9a-f-]{36}$/;
const STAGE_NAV = "写作三步";

type StageLabel = "结构" | "段落" | "成稿";

/**
 * The greeting is split across elements (写 is its own <span> so the ink ring
 * can be drawn behind it), so — same convention as reading-walk's
 * expectGreeting — it is matched on the heading's textContent.
 */
async function expectGreeting(page: Page): Promise<void> {
  const heading = page.getByRole("heading", { level: 1 });
  await expect(heading).toBeVisible();
  expect(await heading.textContent()).toBe("Hi，今天想写点什么");
}

/**
 * Complete the entry 设定 dialog. It gates the room on first open only
 * (`setup_at` is stamped once), so every fresh writing passes through here.
 *
 * `words: null` exercises the 跳过 path — length is never a precondition
 * (铁律②), and the dialog must let her out either way.
 */
async function completeSetup(
  page: Page,
  opts: { lang?: "中文" | "English"; words?: number | null; note?: string } = {},
): Promise<void> {
  const dialog = page.getByRole("dialog", { name: "开始之前" });
  await expect(dialog).toBeVisible({ timeout: 30_000 });

  // There is deliberately NO 文体 selector: a lite student may not know the
  // word, so the third field is an open box instead and the model infers
  // genre from her own sentences. Asserted so a future edit cannot quietly
  // reintroduce a vocabulary question.
  await expect(dialog.getByText("文体")).toHaveCount(0);

  if (opts.lang) await dialog.getByRole("button", { name: opts.lang }).click();
  if (opts.words != null) await dialog.getByLabel("目标字数").fill(String(opts.words));
  if (opts.note) await dialog.getByLabel("还想说点什么").fill(opts.note);

  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/setup") && r.request().method() === "PUT"),
    dialog.getByRole("button", { name: opts.words == null && !opts.note ? "跳过" : "开始", exact: true }).click(),
  ]);
  await expect(dialog).toHaveCount(0, { timeout: 15_000 });
}

/** Type an idea into the landing box and land in its room. Returns the id. */
async function startWriting(page: Page, idea: string): Promise<string> {
  await page.goto("/writings");
  await expectGreeting(page);
  await page.getByPlaceholder(BOX_PLACEHOLDER).fill(idea);
  await page.getByRole("button", { name: "开始写作", exact: true }).click();
  await expect(page).toHaveURL(WRITING_URL, { timeout: 30_000 });
  return new URL(page.url()).pathname.split("/").pop()!;
}

/**
 * Jump to a stage via the always-clickable StageMap — never a gate
 * (writing_stage.go). Waits for the stage's own POST so the caller's next
 * action never races the state update.
 *
 * NOT `exact: true`: each button's accessible name is its step number or
 * checkmark plus the label ("1结构"), so a substring match is correct here —
 * and 结构/段落/成稿 never collide with each other as substrings.
 */
async function jumpStage(page: Page, label: StageLabel): Promise<void> {
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/stage") && r.request().method() === "POST"),
    page.getByRole("navigation", { name: STAGE_NAV }).getByRole("button", { name: label }).click(),
  ]);
}

/** The coach's live reply bubbles. ThinkingRow carries BOTH `data-role` and
 *  `aria-label="印记正在打字"`; a real reply carries `data-role` alone, so the
 *  `:not([aria-label])` is what separates "a bubble appeared" from "the reply
 *  actually landed". */
function assistantReplies(page: Page): Locator {
  return page.locator('[data-role="assistant"]:not([aria-label])');
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

test("writing walk: 设定 → 印记 opens → structure → her own points → guiding box → two paragraphs → compose → feedback → 完成这篇", async ({
  page,
}) => {
  // Four separate live model calls happen here (opening, coach turn,
  // structure recommendation, block guide) plus the review, each capped
  // server-side at 150s. The shared 300s default was sized for reading's
  // two-call walk, so this test takes its own generous ceiling rather than
  // racing it.
  test.setTimeout(900_000);

  const idea =
    "我想写一篇论证文，说说学校该不该允许学生在课间用手机——我自己观察到很多同学课间刷手机后上课更难集中注意力，但也有人说课间是唯一能自由社交、放松一下的时间。";
  const id = await startWriting(page, idea);

  // ── the 设定 dialog: language, length, and her own words ────────────────
  await completeSetup(page, { lang: "中文", words: 500, note: "这是老师布置的作业，我自己更倾向不要一刀切禁止。" });
  await expect(page.getByRole("navigation", { name: STAGE_NAV })).toBeVisible({ timeout: 30_000 });

  // ── the idea really made the round trip: it is BOTH the title and the
  // first message in the transcript (createWriting's ONE transaction) ──────
  await expect(page.getByRole("heading", { level: 1 })).toContainText(idea.slice(0, 20));
  await expect(page.locator('[data-role="student"]', { hasText: idea })).toBeVisible();
  await expect(page.getByRole("heading", { name: "结构" })).toBeVisible();

  // ── the length is VISIBLE. The old room saved it and never showed it
  // again, which is why it read as broken; the header counter is both the
  // confirmation and the feature. ─────────────────────────────────────────
  await expect(page.getByRole("button", { name: /500/ })).toBeVisible();

  // ── 印记 SPEAKS FIRST. The old room was silent until she typed; this is
  // the assertion that keeps it from going quiet again. One reply, unprompted,
  // and no error banner (a failed opening is surfaced, never faked). ───────
  const thinking = page.locator('[aria-label="印记正在打字"]');
  await expect(assistantReplies(page)).toHaveCount(1, { timeout: 180_000 });
  const opening = (await assistantReplies(page).first().innerText()).trim();
  expect(opening.length).toBeGreaterThan(10);
  await expect(page.getByRole("alert")).toHaveCount(0);

  // ── a real coach turn on top of that opening ────────────────────────────
  const question = "如果只是弱化课间手机的使用时间，而不是完全禁止，这个角度站得住吗？";
  await page.getByPlaceholder("想到什么，跟印记说说").fill(question);
  await page.getByRole("button", { name: "发送", exact: true }).click();
  await expect(page.locator('[data-role="student"]', { hasText: question })).toBeVisible();
  await expect(thinking).toBeVisible();
  await expect(thinking).toHaveCount(0, { timeout: 180_000 });
  await expect(assistantReplies(page)).toHaveCount(2, { timeout: 5_000 });
  await expect(page.getByRole("alert")).toHaveCount(0);

  // ── 结构: the AI may only SELECT from the fixed library ─────────────────
  //
  // The button that used to live here — 「帮我拟一份候选」 — sent her whole
  // transcript to the model and got a finished outline back. It is deleted,
  // and this asserts it stays deleted.
  await expect(page.getByRole("button", { name: "帮我拟一份候选" })).toHaveCount(0);

  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/structure/recommend"), { timeout: 180_000 }),
    page.getByRole("button", { name: "帮我挑一副", exact: true }).click(),
  ]);
  await expect(page.getByText("印记的建议", { exact: true })).toBeVisible({ timeout: 180_000 });
  await expect(page.getByText("这次没能给出建议，你可以直接自己挑一个。")).toHaveCount(0);

  // Recommending PERSISTS NOTHING — she accepts by clicking. Until she does,
  // no block exists.
  await expect(page.getByPlaceholder("用一句话写下你在这一块想说什么")).toHaveCount(0);

  const [applyResp] = await Promise.all([
    page.waitForResponse((r) => r.url().endsWith("/structure") && r.request().method() === "POST"),
    page.getByRole("button", { name: "就用这一副", exact: true }).click(),
  ]);
  const applied = (await applyResp.json()) as { structureKey: string; outline: { text: string; role: string }[] };
  expect(applied.outline.length).toBeGreaterThanOrEqual(2);

  // ── 铁律 PROOF #1: every block arrives LABELLED and EMPTY ───────────────
  //
  // The skeleton contributes the role labels and not one word of content. If
  // any of these ever came back pre-filled, the product would be writing her
  // essay for her, and this is the browser-level assertion of that.
  const blockInputs = page.getByPlaceholder("用一句话写下你在这一块想说什么");
  await expect(blockInputs).toHaveCount(applied.outline.length, { timeout: 15_000 });
  for (let i = 0; i < applied.outline.length; i++) {
    await expect(blockInputs.nth(i)).toHaveValue("");
    expect(applied.outline[i]!.text).toBe("");
    expect(applied.outline[i]!.role.length).toBeGreaterThan(0);
  }

  // ── 铁律 PROOF #2: the guiding box asks, it does not tell ───────────────
  const firstBlockValueBefore = await blockInputs.nth(0).inputValue();
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/guide") && r.request().method() === "POST", { timeout: 180_000 }),
    page.getByRole("button", { name: "想不出来？", exact: true }).first().click(),
  ]);
  await expect(page.getByText("想一想", { exact: true })).toBeVisible({ timeout: 180_000 });
  await expect(page.getByText("这次没问出问题来，再试一次。")).toHaveCount(0);

  // Every line the box shows must be a QUESTION. A declarative sentence here
  // would be a sentence she could paste into the essay, which is exactly what
  // the server-side filter exists to make impossible — asserted in the
  // browser rather than trusted.
  const guideBox = page.getByText("想一想", { exact: true }).locator("..").locator("..");
  const guideLines = await guideBox.locator("li").allInnerTexts();
  expect(guideLines.length).toBeGreaterThanOrEqual(1);
  for (const line of guideLines) {
    expect(line.trim()).toMatch(/[？?]$/);
  }
  // And asking for help changed nothing she had written.
  await expect(blockInputs.nth(0)).toHaveValue(firstBlockValueBefore);

  // ── her own points, in her own words ────────────────────────────────────
  await blockInputs.nth(0).fill("我反对完全禁止课间用手机，但支持限制时长。");
  await Promise.all([
    page.waitForResponse((r) => r.url().endsWith("/outline") && r.request().method() === "PUT"),
    blockInputs.nth(0).blur(),
  ]);
  await expect(page.getByText("保存失败，请重试。")).toHaveCount(0);

  // The role labels survive a save of her text — the PUT rewrites both
  // columns, so a client that dropped `role` would wipe every label here.
  await expect(blockInputs.nth(0)).toHaveValue("我反对完全禁止课间用手机，但支持限制时长。");
  for (const block of applied.outline) {
    await expect(page.getByText(block.role, { exact: true }).first()).toBeVisible();
  }

  // ── 段落: write exactly two paragraphs, nothing else ────────────────────
  await jumpStage(page, "段落");
  await expect(page.getByRole("heading", { name: "段落" })).toBeVisible();

  const paragraph1 =
    "我自己就有过这种经历：课间刷十分钟手机之后，上课铃响了脑子还没转回来，常常要老师提醒才翻到正确的那一页。";
  const paragraph2 =
    "但如果直接完全禁止课间用手机，也会让平时靠线上聊天维持友谊的同学少了唯一能自由社交的窗口——这是「完全禁止」绕不开的代价。";

  const paragraphBoxes = page.getByPlaceholder("写这一段……");
  await expect(paragraphBoxes).toHaveCount(applied.outline.length, { timeout: 15_000 });

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

  // ── there are NO 工具卡 in this room. Not a shelf, not a deck, not an
  // offer inside the guiding box. ────────────────────────────────────────
  await expect(page.getByRole("button", { name: "工具卡" })).toHaveCount(0);
  await expect(page.getByText("让步段 · 以退为进")).toHaveCount(0);

  // ── 成稿: compose, then the 铁律 mechanical proof ───────────────────────
  await jumpStage(page, "成稿");
  await expect(page.getByRole("heading", { name: "成稿", level: 2 })).toBeVisible();

  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/compose") && r.request().method() === "POST"),
    page.getByRole("button", { name: "从段落拼出初稿", exact: true }).click(),
  ]);
  const draftBox = page.getByPlaceholder("拼出来的初稿会出现在这里——你也可以直接在这儿写、改。");

  // ── 铁律 PROOF #3. composeSnippetsIntoDraft only trims and joins non-empty
  // snippet text with "\n\n" — no model call, nothing invented. Only two
  // slots were ever filled, so the body must be EXACTLY those two paragraphs
  // joined — not merely "contains them".
  await expect(draftBox).toHaveValue(`${paragraph1}\n\n${paragraph2}`);

  // ── ask for feedback: a real model call that must never touch the draft ─
  const beforeReview = await draftBox.inputValue();
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/review") && r.request().method() === "POST", { timeout: 180_000 }),
    page.getByRole("button", { name: "请印记看看", exact: true }).click(),
  ]);
  await expect(page.getByRole("heading", { name: "印记的反馈" })).toBeVisible({ timeout: 180_000 });
  const feedbackPanel = page.getByRole("heading", { name: "印记的反馈" }).locator("..");
  const feedbackText = (await feedbackPanel.locator("p").innerText()).trim();
  expect(feedbackText.length).toBeGreaterThan(10);
  await expect(page.getByText("这次体检没成功，请重试。")).toHaveCount(0);
  // reviewWritingDraft returns commentary only — never writes to
  // writing_draft.body. The box must read exactly as it did before.
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
  await expect(page.getByRole("navigation", { name: STAGE_NAV })).toHaveCount(0);
  await expect(page.getByPlaceholder("想到什么，跟印记说说")).toHaveCount(0);

  // A cold reload of the same URL is the same terminal surface — and the
  // 设定 dialog does NOT reappear (setup_at is stamped once).
  await page.reload();
  await expect(page.getByText("已完成", { exact: true })).toBeVisible({ timeout: 30_000 });
  await expect(page.getByRole("dialog", { name: "开始之前" })).toHaveCount(0);
  await expect(page.getByRole("navigation", { name: STAGE_NAV })).toHaveCount(0);
  expect(new URL(page.url()).pathname).toBe(`/writings/${id}`);
});

test("the 设定 dialog can be skipped entirely — length is never a precondition", async ({ page }) => {
  test.setTimeout(400_000);

  await startWriting(page, titled("跳过设定的写作"));
  // 跳过 with nothing filled: language defaults, length stays unset.
  await completeSetup(page);

  await expect(page.getByRole("navigation", { name: STAGE_NAV })).toBeVisible({ timeout: 30_000 });
  // No target set, so the counter offers to set one rather than showing a
  // number nobody chose.
  await expect(page.getByRole("button", { name: /定个目标/ })).toBeVisible();

  // And she can walk straight to 成稿 without ever choosing a structure —
  // stages are a map, not a gate.
  await jumpStage(page, "成稿");
  await expect(page.getByRole("heading", { name: "成稿", level: 2 })).toBeVisible();

  // Reload: the dialog is done with, permanently.
  await page.reload();
  await expect(page.getByRole("dialog", { name: "开始之前" })).toHaveCount(0, { timeout: 30_000 });
});

test("铁律 — the English exemplar exists on the page, never enters the draft, and no control can insert it", async ({
  page,
}) => {
  // One live model call (exemplar generation), capped server-side at 150s.
  test.setTimeout(400_000);

  // The 英文写作 topic tile creates the writing with `lang: "en"`, and the
  // 设定 dialog opens with English already selected — so 开始 keeps it. (A
  // Chinese writing cannot exercise this leg at all: exemplar generation 400s
  // `exemplar_not_available` for anything but `lang === "en"` before any model
  // call is made.)
  await page.goto("/writings");
  await expectGreeting(page);
  await page.getByText("A Moment That Changed How I See Something", { exact: true }).click();
  await expect(page).toHaveURL(WRITING_URL, { timeout: 30_000 });
  await completeSetup(page, { lang: "English", words: 400 });
  await expect(page.getByRole("navigation", { name: STAGE_NAV })).toBeVisible({ timeout: 30_000 });

  // Stages are a map: jump straight to 段落 with no structure at all, then
  // add one free paragraph (SnippetsStage supports free-form paragraphs when
  // there is no structure to follow).
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

  // 示范段落 is offered at all only because lang === "en".
  const exemplarButton = page.getByRole("button", { name: "示范段落", exact: true });
  await expect(exemplarButton).toBeVisible();
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/exemplar") && r.request().method() === "POST", { timeout: 180_000 }),
    exemplarButton.click(),
  ]);

  // ── ASSERTION 1: the exemplar EXISTS on the page ────────────────────────
  const badge = page.getByText("示范", { exact: true });
  await expect(badge).toBeVisible({ timeout: 180_000 });
  await expect(
    page.getByText("读一读别人会怎么写这一段，再回去写你自己的版本——不是给你抄的", { exact: true }),
  ).toBeVisible();
  const exemplarBox: Locator = badge.locator("..").locator("..");
  const exemplarText = (await exemplarBox.locator("p").first().innerText()).trim();
  expect(exemplarText.length).toBeGreaterThan(40);
  expect(exemplarText).not.toBe(herOwnParagraph);
  expect(/[A-Za-z]/.test(exemplarText)).toBe(true);

  // ── ASSERTION 2: absent from the draft — both the paragraph box she typed
  // into and, downstream, the composed draft. ─────────────────────────────
  await expect(paragraphBox).toHaveValue(herOwnParagraph);

  await jumpStage(page, "成稿");
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/compose") && r.request().method() === "POST"),
    page.getByRole("button", { name: "从段落拼出初稿", exact: true }).click(),
  ]);
  const draftBox = page.getByPlaceholder("拼出来的初稿会出现在这里——你也可以直接在这儿写、改。");
  await expect(draftBox).toHaveValue(herOwnParagraph);
  expect(await draftBox.inputValue()).not.toContain(exemplarText);

  // ── ASSERTION 3: no control anywhere puts it there. ─────────────────────
  // Structural: nothing clickable or editable lives inside the exemplar's own
  // box — no copy button, no "用这段", no drag handle. Asserted by counting
  // interactive descendants rather than by trusting the source comment.
  await jumpStage(page, "段落");
  const badgeAgain = page.getByText("示范", { exact: true });
  if (await badgeAgain.count()) {
    const boxAgain = badgeAgain.locator("..").locator("..");
    await expect(boxAgain.locator('button, a, [role="button"], input, textarea, select')).toHaveCount(0);
  }
  await expect(page.getByRole("alert")).toHaveCount(0);

  // ── never persisted: reload and the exemplar is gone, while her own
  // paragraph — which WAS saved — survives. ──────────────────────────────
  await page.reload();
  await jumpStage(page, "段落");
  await expect(page.getByPlaceholder("写这一段……")).toHaveValue(herOwnParagraph, { timeout: 15_000 });
  await expect(page.getByText("示范", { exact: true })).toHaveCount(0);
});
