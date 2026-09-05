import { expect, test, type Page, type Locator } from "@playwright/test";

/**
 * The lite edition's writing room, walked end to end against a real API and a
 * real database: land → type an idea → the 设定 dialog → 印记 speaks first →
 * plan out loud while a mind map grows from her answers → go write → two
 * paragraphs → compose → feedback → 完成这篇 → 已完成.
 *
 * Reuses the reading walk's harness verbatim (globalSetup.ts flips the seeded
 * school to `edition='lite'` and signs Phoebe in once; run-stack.sh boots the
 * throwaway Postgres + API + lite dev server; playwright.config.ts points
 * baseURL at :5174) — see reading-walk.spec.ts for why each piece exists.
 *
 * Strings come from the sources, not from a brief:
 * apps/lite-web/src/writings/{WritingsLanding,WritingRoomHost,StageMap,
 * WritingSetupModal,PlanningView,SnippetsStage,ComposeStage,GuideBox,
 * CommentPanel,ProseSurface}.tsx.
 *
 * REWRITTEN 2026-08-27 for the scaffold redesign. What changed and why it
 * matters to this walk:
 *   - 构思 is gone; the map is three steps (结构/段落/成稿).
 *   - A 设定 dialog gates the room on first open, and is where length is set
 *     (it used to be a dead box on the 构思 page).
 *   - 印记 speaks FIRST, so the room is not silent on arrival.
 *   - 「帮我拟一份候选」 is DELETED, and so is the skeleton picker that briefly
 *     replaced it. 结构 is a planning conversation whose output IS the outline.
 *   - The 工具卡 are gone from this room entirely.
 *
 * REWRITTEN AGAIN 2026-08-28, this time to catch up with eleven merged
 * commits that rebuilt 段落 and 成稿 underneath this file while it sat
 * unrun (it had gone known-stale). What changed this round:
 *   - 从段落拼出初稿 is gone. composeWritingDraft now fires ONCE,
 *     automatically, on arrival in 成稿 — the draft is simply there, not
 *     behind a button. The button survives, demoted to 从段落重新拼一次, for
 *     re-pulling after she edits 段落 again.
 *   - 请印记看看 now answers a STRUCTURED Comment (summary + points), not a
 *     prose blob under a "印记的反馈" heading — that heading no longer
 *     exists. `CommentPanel` renders it, and every point is a
 *     `[data-comment-point]` button that traces to a `<mark>` inside
 *     ProseSurface's `[data-prose-layer]`.
 *   - 段落's guide box is now PRESENT ON ARRIVAL: a single batch call guides
 *     the whole outline by itself, so the happy path never needs to click
 *     「卡住了？」.
 *
 * WHAT EACH LEG PROTECTS
 *
 *  - **The room is real, not a mock.** The idea typed into the landing box
 *    really becomes the first message in the transcript (createWriting's
 *    atom+writing+first-message transaction, writings.go); the opening line,
 *    the coach turn, the planning turns and the review are all real model
 *    calls through the lite gateway.
 *  - **铁律 proof #1 — the map is built from HER words.** Every node a planning
 *    turn adds carries text, and a SECOND turn must leave every earlier node
 *    byte-identical: writing_plan.go holds no update or delete call, so 印记
 *    can add to her map but never rewrite or remove it.
 *  - **铁律 proof #2 — guidance is questions, never sentences.** The guiding
 *    box's every line must end in a question mark, and asking for it must
 *    leave her own field byte-identical.
 *  - **铁律 proof #3 — compose is concatenation, not invention.**
 *    composeSnippetsIntoDraft (writing_compose.go) is a pure string join, so
 *    the composed body is asserted EXACTLY equal to the two paragraphs she
 *    typed — not "contains", not "roughly matches".
 *  - **Guidance is visible without being clicked.** 段落's guide box is
 *    painted from a batch call that fires on arrival, never gated behind
 *    「卡住了？」 — asserted with no click anywhere in the happy path. If this
 *    ever needs a click to pass, that is the exact regression it exists to
 *    catch.
 *  - **A comment traces to a real sentence.** Clicking a
 *    `[data-comment-point]` must produce a `<mark>` inside ProseSurface's
 *    `[data-prose-layer]`, over the literal quote the server validated.
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

test("writing walk: 设定 → 印记 opens → planning grows a mind map → 去写 → two paragraphs → compose → feedback → 完成这篇", async ({
  page,
}) => {
  // Five separate live model calls happen here — opening, two planning
  // turns, the batch paragraph guide (fired once on arrival in 段落), and
  // the review — each capped server-side at 150s. The shared 300s default
  // was sized for reading's two-call walk, so this test takes its own
  // generous ceiling rather than racing it.
  test.setTimeout(1_200_000);

  const idea =
    "我想写一篇论证文，说说学校该不该允许学生在课间用手机——我自己观察到很多同学课间刷手机后上课更难集中注意力，但也有人说课间是唯一能自由社交、放松一下的时间。";
  // 完成这篇时她给这篇起的名字。带上这一轮的后缀，重复跑不会在「我的写作」
  // 里撞名字。
  const PIECE_NAME = titled("课间手机该不该禁");
  const id = await startWriting(page, idea);

  // ── the 设定 dialog: language, length, and her own words ────────────────
  await completeSetup(page, { lang: "中文", words: 500, note: "这是老师布置的作业，我自己更倾向不要一刀切禁止。" });
  await expect(page.getByPlaceholder("说说你的想法")).toBeVisible({ timeout: 30_000 });

  // ── the idea really made the round trip: it is BOTH the title and the
  // first message in the transcript (createWriting's ONE transaction) ──────
  await expect(page.getByRole("heading", { level: 1 })).toContainText(idea.slice(0, 20));
  await expect(page.locator('[data-role="student"]', { hasText: idea })).toBeVisible();

  // ── planning takes the WHOLE screen. No stage bar and no length counter
  // compete with the one thing this screen is for; both come back the moment
  // she leaves for 段落, and the length is asserted there instead. ─────────
  await expect(page.getByRole("navigation", { name: STAGE_NAV })).toHaveCount(0);
  await expect(page.getByText(/已写/)).toHaveCount(0);

  // ── 印记 SPEAKS FIRST. The old room was silent until she typed; this is
  // the assertion that keeps it from going quiet again. One reply, unprompted,
  // and no error banner (a failed opening is surfaced, never faked). ───────
  await expect(assistantReplies(page)).toHaveCount(1, { timeout: 180_000 });
  const opening = (await assistantReplies(page).first().innerText()).trim();
  expect(opening.length).toBeGreaterThan(10);
  await expect(page.getByRole("alert")).toHaveCount(0);

  // ── 结构: a PLANNING CONVERSATION, never a picker ───────────────────────
  //
  // Two dead affordances asserted gone. 「帮我拟一份候选」 sent her whole
  // transcript to the model and got a finished outline back. 「帮我挑一副」
  // replaced it with a shelf of fixed skeletons whose blocks read
  // 「你承认它哪一部分是对的」 — still a form, and unreadable for a
  // middle-schooler. Both are deleted; this keeps them deleted.
  await expect(page.getByRole("button", { name: "帮我拟一份候选" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "帮我挑一副" })).toHaveCount(0);
  await expect(page.getByText("全部结构")).toHaveCount(0);

  // The map panel does not exist until the map has something on it — an empty
  // panel from the first second is a promise the screen has not kept.
  await expect(page.getByText("你的思路")).toHaveCount(0);

  // One planning turn: she states the claim, and it grows onto the canvas.
  const claim = "我最想让读的人相信：课间可以用手机，但要限制时长。";
  const [planResp] = await Promise.all([
    page.waitForResponse(
      (r) => r.url().includes("/plan/turn") && r.request().method() === "POST",
      { timeout: 180_000 },
    ),
    (async () => {
      await page.getByPlaceholder("说说你的想法").fill(claim);
      await page.getByRole("button", { name: "发送", exact: true }).click();
    })(),
  ]);
  const planned = (await planResp.json()) as {
    reply: string;
    outline: { id: string; text: string; role: string; depth: number }[];
    addedIds: string[];
  };
  await expect(page.getByRole("alert")).toHaveCount(0);
  expect(planned.reply.trim().length).toBeGreaterThan(5);

  // ── 铁律 PROOF #1: the map is built from HER words, never invented ──────
  //
  // Every node the turn added must carry text, and 印记 may only ever ADD —
  // writing_plan.go holds no update or delete call. The strongest browser-
  // level check of that is the SECOND turn below: everything from this one
  // must survive it byte-identical.
  expect(planned.outline.length).toBeGreaterThanOrEqual(1);
  for (const node of planned.outline) {
    expect(node.text.trim().length).toBeGreaterThan(0);
  }
  await expect(page.getByText("你的思路")).toBeVisible({ timeout: 15_000 });
  for (const node of planned.outline) {
    await expect(page.getByText(node.text, { exact: true }).first()).toBeVisible();
  }

  // ── 铁律 PROOF #2: a planning turn cannot rewrite or delete her nodes ───
  const [secondResp] = await Promise.all([
    page.waitForResponse(
      (r) => r.url().includes("/plan/turn") && r.request().method() === "POST",
      { timeout: 180_000 },
    ),
    (async () => {
      await page
        .getByPlaceholder("说说你的想法")
        .fill("第一个理由是课间是唯一能和同学聊两句的时间；第二个理由是完全禁止之后大家改成偷偷玩，反而更难管。");
      await page.getByRole("button", { name: "发送", exact: true }).click();
    })(),
  ]);
  const afterSecond = (await secondResp.json()) as {
    outline: { id: string; text: string; role: string; depth: number }[];
  };
  for (const before of planned.outline) {
    const still = afterSecond.outline.find((n) => n.id === before.id);
    expect(still, `a planning turn deleted her node "${before.text}"`).toBeTruthy();
    expect(still!.text, "a planning turn rewrote her node").toBe(before.text);
  }
  // It did grow, though — that is the whole point of the screen.
  expect(afterSecond.outline.length).toBeGreaterThan(planned.outline.length);
  await expect(page.getByRole("alert")).toHaveCount(0);

  // The map is a TREE, not a flat list: at least one node hangs under another.
  expect(afterSecond.outline.some((n) => n.depth > 0)).toBe(true);

  // ── planning is never a gate: 去写 leaves whenever she wants ────────────
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/stage") && r.request().method() === "POST"),
    page.getByRole("button", { name: /去写/ }).click(),
  ]);
  await expect(page.getByRole("navigation", { name: STAGE_NAV })).toBeVisible({ timeout: 30_000 });

  // ── THE HANDOVER: the map she planned IS the outline she now writes into ─
  //
  // This is the claim the whole redesign rests on — no conversion step, so
  // nothing can be lost between planning and writing. Every node she produced
  // is a paragraph slot, carrying her own sentence as its heading.
  await expect(page.getByRole("heading", { name: "段落" })).toBeVisible();
  for (const node of afterSecond.outline) {
    await expect(page.getByText(node.text, { exact: true }).first()).toBeVisible();
  }
  const plannedCount = afterSecond.outline.length;

  // ── the length is VISIBLE here. The old room saved it and never showed it
  // again, which is why it read as broken; the header counter is both the
  // confirmation and the feature. ─────────────────────────────────────────
  await expect(page.getByRole("button", { name: /500/ })).toBeVisible();

  // ── 段落: write exactly two paragraphs, nothing else ────────────────────
  const paragraph1 =
    "我自己就有过这种经历：课间刷十分钟手机之后，上课铃响了脑子还没转回来，常常要老师提醒才翻到正确的那一页。";
  const paragraph2 =
    "但如果直接完全禁止课间用手机，也会让平时靠线上聊天维持友谊的同学少了唯一能自由社交的窗口——这是「完全禁止」绕不开的代价。";

  const paragraphBoxes = page.getByPlaceholder("写这一段……");
  await expect(paragraphBoxes).toHaveCount(plannedCount, { timeout: 15_000 });

  // ── guidance is PRESENT ON ARRIVAL — no 「卡住了？」 click needed on the
  // happy path. A single batch call (POST /writings/{id}/guide) guides the
  // whole outline by itself the first time 段落 has nothing guided yet; if
  // this box only ever showed up after a click, that IS the regression it
  // exists to catch. Waited on with a generous timeout because the batch
  // call is a real (if fast) round trip that fires on mount — not something
  // already true the instant the heading appeared. ────────────────────────
  const firstGuideBox = page.getByText("写作引导").first();
  await expect(firstGuideBox).toBeVisible({ timeout: 180_000 });
  await expect(page.getByText("这一段要做的事").first()).toBeVisible();
  await expect(page.getByText("想一想").first()).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);

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
  // An absence is only worth asserting once the thing it contradicts has had
  // its chance to render. `waitForResponse` returns on the NETWORK response —
  // one tick before React has committed anything — so checking the banner
  // there would pass whether or not it later appears. SnippetsStage clears
  // 「保存中…」 in the same `finally` that follows the `catch` which sets
  // 「保存这一段失败」, so the spinner going away IS the save's settle signal:
  // once it is gone, a failed save has already painted its banner.
  await expect(page.getByText("保存中…")).toHaveCount(0);
  await expect(page.getByText("保存这一段失败，请重试。")).toHaveCount(0);

  // ── there are NO 工具卡 in this room. Not a shelf, not a deck, not an
  // offer inside the guiding box. Safe as absences: nothing in flight could
  // introduce one, and the room is provably fully painted by here (the guide
  // box arrived above, and the save settle just landed). ─────────────────
  await expect(page.getByRole("button", { name: "工具卡" })).toHaveCount(0);
  await expect(page.getByText("让步段 · 以退为进")).toHaveCount(0);

  // ── 成稿: the draft ASSEMBLES ITSELF on arrival, then the 铁律 mechanical
  // proof. There is no button to click here first — 从段落拼出初稿 is gone;
  // composeWritingDraft (writing_compose.go's pure string join) now fires
  // once, automatically, the moment she reaches a stage with nothing
  // composed yet and paragraphs to compose from. ─────────────────────────
  await jumpStage(page, "成稿");
  await expect(page.getByRole("heading", { name: "成稿", level: 2 })).toBeVisible();

  // 🚨 和 ComposeStage.tsx 里那一行一模一样。2026-09-03 那次「不写文学腔」的
  // 清扫把「剩下的会跟着它长出来」改掉了（「长出来」只留给树），单元测试同步了，
  // 这条 e2e 漏了，于是它在这里报「找不到元素」。
  const draftBox = page.getByPlaceholder("请先写下你最想说的那句话，再围绕它展开。");

  // ── 铁律 PROOF #3. composeSnippetsIntoDraft only trims and joins non-empty
  // snippet text with "\n\n" — no model call, nothing invented. Only two
  // slots were ever filled, so the body must be EXACTLY those two paragraphs
  // joined — not merely "contains them". A generous timeout here: the
  // arrival assembly is a real round trip fired on mount, not something
  // already true the instant the heading appeared.
  await expect(draftBox).toHaveValue(`${paragraph1}\n\n${paragraph2}`, { timeout: 30_000 });

  // ── the button survives, demoted to a manual re-pull for after she edits
  // 段落 again (从段落重新拼一次). Pressing it here — body === assembledBody,
  // nothing at stake — exercises /compose the same way a click always did,
  // without tripping the overwrite-confirm dialog this button now guards.
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/compose") && r.request().method() === "POST"),
    page.getByRole("button", { name: "从段落重新拼一次", exact: true }).click(),
  ]);
  await expect(draftBox).toHaveValue(`${paragraph1}\n\n${paragraph2}`);

  // ── ask for feedback: a real model call that must never touch the draft.
  // POST /review now answers a STRUCTURED Comment (summary + points), not
  // the old prose blob under a "印记的反馈" heading — that heading is gone.
  // CommentPanel renders the summary as a line of text and each point as a
  // `[data-comment-point]` button. ────────────────────────────────────────
  const beforeReview = await draftBox.inputValue();
  const [reviewResp] = await Promise.all([
    page.waitForResponse((r) => r.url().includes("/review") && r.request().method() === "POST", { timeout: 180_000 }),
    page.getByRole("button", { name: "请印记看看", exact: true }).click(),
  ]);
  const reviewed = (await reviewResp.json()) as {
    comment: { summary: string; points: { text: string; quote: string }[] };
  };
  expect(reviewed.comment.summary.trim().length).toBeGreaterThan(10);

  const summaryLine = page.getByText(reviewed.comment.summary, { exact: true });
  await expect(summaryLine).toBeVisible({ timeout: 180_000 });
  // AFTER the summary is on screen, never before. `reviewResp.json()` resolves
  // a tick after the network response and well before CommentPanel has
  // re-rendered, so checking this absence there would pass regardless of
  // whether the error state later appears — vacuous, and exactly the trap
  // this file has been caught by twice. The summary being visible is proof
  // the review round trip has been applied; only then does the error banner's
  // absence mean anything.
  await expect(page.getByText("这次体检没成功，请重试。")).toHaveCount(0);

  // reviewWritingDraft returns commentary only — never writes to
  // writing_draft.body. The box must read exactly as it did before.
  await expect(draftBox).toHaveValue(beforeReview);

  // ── a comment traces back to a real sentence: click a
  // `[data-comment-point]`, and a <mark> appears in ProseSurface's mirrored
  // highlight layer over the sentence it quoted. Both waited on as
  // ARRIVALS (visible, generous timeout) — never as an absence that could
  // resolve true in the instant before the click's re-render lands.
  const firstPoint = page.locator("[data-comment-point]").first();
  await expect(firstPoint).toBeVisible({ timeout: 15_000 });
  await firstPoint.click();
  const mark = page.locator("[data-prose-layer] mark");
  await expect(mark).toBeVisible({ timeout: 15_000 });
  await expect(mark).toHaveText(reviewed.comment.points[0].quote);

  // ── 完成这篇 → 起名字 → 已完成 ────────────────────────────────────────
  //
  // 🚨 完成这篇先问名字（NamePieceModal，2026-08-30 上线）：这一栏里放的还是她
  // 最开始写的那句「我想写…」，而报告、导出的图和发出去的链接上印的都是它。
  // 这条 walk 一直没跟上——点完按钮就干等 /finish，等 15 秒然后失败。从那天起
  // 它就没绿过，而它失败的方式看起来像"完成坏了"，其实是走到了一扇没人认识的门。
  await page.getByRole("button", { name: "完成这篇", exact: true }).click();
  const nameBox = page.getByPlaceholder("写一个你想让别人看到的名字");
  await expect(nameBox).toBeVisible({ timeout: 60_000 });
  await nameBox.fill(PIECE_NAME);
  await Promise.all([
    page.waitForResponse(
      (r) => r.url().includes("/finish") && r.request().method() === "POST",
      { timeout: 60_000 },
    ),
    page.getByRole("button", { name: "确认并完成" }).click(),
  ]);
  await expect(page.getByText("已完成", { exact: true })).toBeVisible({ timeout: 15_000 });
  // 起过名字之后，标题就是这个名字，不再是她最初那句「我想写…」。
  await expect(page.getByRole("heading", { name: PIECE_NAME })).toBeVisible();
  // 🚨 完成之后先出现的是「印记正在把这次写的东西整理成一份报告，稍等一下。」，
  // 那是一次真的模型调用。她写的那两段要等报告落下来才显示，所以这里必须等它
  // 走完——直接断言段落，等到的是那句"稍等一下"，看起来像"完成把她的字弄丢了"。
  await expect(
    page.getByText("印记正在把这次写的东西整理成一份报告", { exact: false }),
  ).toHaveCount(0, { timeout: 300_000 });
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

  // Skipping the dialog lands her in planning, not in a wall.
  await expect(page.getByPlaceholder("说说你的想法")).toBeVisible({ timeout: 30_000 });

  // 去写 is available from the FIRST render — a student who already knows what
  // she wants to say must not have to talk her way past a planning screen.
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/stage") && r.request().method() === "POST"),
    page.getByRole("button", { name: /去写/ }).click(),
  ]);
  await expect(page.getByRole("navigation", { name: STAGE_NAV })).toBeVisible({ timeout: 30_000 });

  // No target set, so the counter offers to set one rather than showing a
  // number nobody chose.
  await expect(page.getByRole("button", { name: /定个目标/ })).toBeVisible();

  // And she can walk straight to 成稿 without ever planning a thing —
  // stages are a map, not a gate.
  await jumpStage(page, "成稿");
  await expect(page.getByRole("heading", { name: "成稿", level: 2 })).toBeVisible();

  // Reload: the dialog is done with, permanently.
  await page.reload();
  await expect(page.getByRole("dialog", { name: "开始之前" })).toHaveCount(0, { timeout: 30_000 });
});

