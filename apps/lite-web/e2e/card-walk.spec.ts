import { expect, test, type Locator, type Page } from "@playwright/test";

/**
 * 带读的卡片, walked end to end against a real API, a real database and a real
 * model — the one thing eleven tasks of unit coverage could not do.
 *
 * The claim under test is not "a card renders". It is the guarantee the whole
 * feature rests on: **she cannot answer without having read the article.** A
 * `choose_span` card's options are sentences taken verbatim out of the
 * paragraphs in front of her (`validateCoachCard` in
 * apps/api/internal/api/reading_coach_card.go drops the whole card otherwise),
 * so the walk reads the options off the SCREEN and looks for each of them in
 * the article's own rendered paragraphs. A mocked turn cannot prove that: the
 * mock would be the thing supplying the sentences.
 *
 * The other four legs, in the order she meets them:
 *
 *  - **A tap is the whole turn.** She types nothing. The POST is intercepted
 *    with `route.continue()` (observed, never faked) so the walk can assert
 *    the request carried `text: ""` and her `cardAnswer` — the design claim
 *    that answering costs her no sentence-composing at all.
 *  - **Refresh keeps both halves.** The card and her answer are rebuilt out of
 *    `atom_message.payload` (migration 0106), not held in React state.
 *  - **Nothing reads as right/wrong** (铁律②). No ✓, no ✗, no score, no answer
 *    key — anywhere on the page, at any point in the journey. The server does
 *    not even send an answer key, because there is no correct answer.
 *  - **A lens still summons with chat cards on screen.** Chat cards live on
 *    `atom_message.payload` precisely so they never take a slot under
 *    `atom_card_one_open_idx` (0096); if someone ever "tidies" them into
 *    `atom_card`, every lens summon deadlocks and this is the leg that says so.
 *
 * Conventions follow reading-walk.spec.ts and coach-walk.spec.ts: file-local
 * helpers, UI strings copied from the sources rather than invented, and ONE
 * continuous journey rather than a matrix of tiny tests — a card can only be
 * answered by a student who got one, and she can only get one by starting 带读.
 *
 * Where the strings come from: the landing page from
 * apps/lite-web/src/readings/ReadingsLanding.tsx, the card from
 * readings/CoachCard.tsx, the step dial from readings/ReadingPlanDial.tsx, the
 * room's chrome from readings/ReadingRoom.tsx.
 *
 * 🚨 SLOW ON PURPOSE. Every leg below is a real flagship call (40–70s each on
 * DeepSeek), and the walk may need a few turns to be handed the card shape it
 * can tap (see findChooseSpanCard). The per-test budget is raised accordingly;
 * a run that looks stuck is almost always a model call in flight.
 */

// ── the article she brings in ────────────────────────────────────────────────
// Original prose, four paragraphs. Deliberately shaped for this walk: every
// sentence closes on a punctuation mark, because `coachCardQuoteIsClause`
// requires an option to sit on clause boundaries — an article of hard-wrapped
// fragments would make the model's cards die in validation and look, wrongly,
// like a frontend bug. Blank lines matter: the server splits blocks on them
// (apps/api/internal/api/reading_blocks.go).
//
// The topic is the spec's own worked example (城市热岛), including the
// 「把灰色的屋顶改成绿色的」 sentence its clause-boundary rule was written
// against.
const RUN = Date.now().toString(36);
const titled = (name: string) => `${name} ${RUN}`;

const ARTICLE_TITLE = titled("城市为什么在夜里也热");
const ARTICLE_BODY = [
  "夏天的傍晚，市中心的气温常常比郊区高出四五度。这种差别不是错觉，气象学上把它叫作城市热岛。",
  "热岛的成因并不神秘。柏油马路和混凝土在白天大量吸热，夜里再慢慢放出来；高楼把风挡住，热量散不出去；空调、汽车和工厂又源源不断地往外排热。",
  "所以近年来很多城市在做的事情，是把灰色的屋顶改成绿色的。屋顶种上草，或者干脆刷成白色，都能把一部分阳光反射回天上。",
  "但把希望全押在屋顶上，未免太轻松了。一栋楼的屋顶降下来的温度，抵不过一整条街的车流；真正管用的办法，往往是把树种回街道两边，而这件事比刷屋顶慢得多，也贵得多。",
].join("\n\n");

const BODY_PLACEHOLDER = "贴一个链接，或者把整篇正文粘进来——也可以上传 DOCX / PDF";
const TITLE_PLACEHOLDER = "给这次阅读起个名字（可留空）";
const READING_URL = /\/readings\/[0-9a-f-]{36}$/;

/** 「点一句就行，怎么想都算你的。」 is rendered ONLY under an OPEN, unanswered
 *  `choose_span` card (CoachCard.tsx) — it is how the one tappable shape is
 *  told apart on screen from `pick_in_article` (which sends her to the
 *  article) and `short_text` (which gives her a textarea). It disappears the
 *  moment she answers, so it is a finder, never a handle: see the prompt-based
 *  locator in the walk. */
const TAPPABLE = "点一句就行，怎么想都算你的。";

/**
 * Everything that would turn this into an exam. Scanned over the WHOLE page's
 * text at four points in the journey, because 铁律② is not a property of one
 * component — a verdict added anywhere on screen breaks it.
 *
 * RULE FOR ANYONE ADDING TO THIS LIST — the same one reading-walk.spec.ts
 * states for its absent-surfaces list: an absence assertion can never fail on
 * its own, so a literal nobody would ever ship reads as coverage while
 * guarding nothing. Every entry below is a mark or a phrase a well-meaning
 * edit really could add to a card that has a "choice" and a "next reply":
 * tick/cross glyphs, a score, a streak, an answer key.
 *
 * NOT in the list, on purpose:
 *  - the ✕ on a quote chip (U+2715, ReadingCoachPanel) — that is 取消引用, a
 *    control, not a verdict;
 *  - the lucide `Check` icon on a SETTLED step in ReadingPlanDial — that is
 *    progress through a plan, and it is an SVG with no text to scan anyway;
 *  - the word 正确 alone, which 印记 may legitimately use about the ARTICLE's
 *    claims (「作者这么说不一定正确」). 正确答案 is the exam word, and it is
 *    the one listed.
 */
const VERDICT_MARKS = ["✓", "✔", "✗", "✘", "√", "❌", "⭕"];
const VERDICT_WORDS = ["正确答案", "标准答案", "答对", "答错", "回答正确", "回答错误", "得分", "满分", "连胜"];

async function expectNothingReadsAsRightOrWrong(page: Page, where: string): Promise<void> {
  const text = await page.locator("body").innerText();
  for (const mark of [...VERDICT_MARKS, ...VERDICT_WORDS]) {
    expect(text, `${where}: 「${mark}」 appeared — a card is a ladder, not an exam (铁律②)`).not.toContain(mark);
  }
}

/** Paste the article on the landing page and land in its room. */
async function startReading(page: Page, title: string): Promise<void> {
  await page.goto("/readings");
  await page.getByPlaceholder(TITLE_PLACEHOLDER).fill(title);
  await page.getByPlaceholder(BODY_PLACEHOLDER).fill(ARTICLE_BODY);
  await page.getByRole("button", { name: "开始阅读" }).click();
  await expect(page).toHaveURL(READING_URL, { timeout: 30_000 });
  await expect(page.locator(".mk-reading-room")).toBeVisible({ timeout: 30_000 });
}

/** The assistant's real turns. The typing bubble is an assistant node too and
 *  carries an aria-label, so it is excluded — counting it would tick up the
 *  instant the request left the browser. Same guard reading-walk.spec.ts uses. */
function assistantTurns(page: Page): Locator {
  return page.locator('[data-role="assistant"]:not([aria-label])');
}

function cardsInLog(page: Page): Locator {
  return page.locator("[data-coach-log] .mk-coachcard");
}

/**
 * Get 印记 to a `choose_span` card, the one shape she can answer by tapping.
 *
 * WHY THIS IS A LOOP AND NOT AN ASSERTION. Which of the three shapes 印记
 * reaches for is its own judgement — 裁定一 of the spec is that the CONTENT of
 * a card is written on the spot, and the shape is part of that content. Two
 * runs of this very walk against the same article opened with two different
 * shapes: once `choose_span`, once `pick_in_article` (「在文中点出你读完最想
 * 画下来的句子」) — both entirely correct cards for a 通读 step. Demanding one
 * particular shape on one particular turn would be a test asserting a coin
 * landed heads, and its red would teach nobody anything.
 *
 * So the walk asserts what the spec actually rules — the first reply carries a
 * CARD (its acceptance item 5) — and then, if that card is not the tappable
 * shape, keeps reading like a student would until one arrives. Her nudge is a
 * plain sentence in the composer; the still-open card collapses beside it
 * (`stale`), which is the product's own 一次只问一个 behaviour, not a
 * workaround.
 *
 * Bounded at four extra turns — five cards seen in all. If none of them is
 * tappable, that is a finding about the coach rather than a flake, and the
 * failure message says so.
 */
async function findChooseSpanCard(page: Page): Promise<Locator> {
  const tappable = cardsInLog(page).filter({ hasText: TAPPABLE });
  for (let nudge = 0; nudge < 4 && (await tappable.count()) === 0; nudge++) {
    // 印记 may have reached for a LENS this turn instead, which locks the
    // composer until the article-side card is dealt with (一次只问一个). An
    // in-flight lens is always dismissable — the room's own deadlock guard —
    // and skipping is a real thing a student does. The walk summons a lens
    // deliberately at the end; here one is just in the way.
    if ((await page.getByRole("button", { name: "跳过这副透镜" }).count()) > 0) {
      await page.getByRole("button", { name: "跳过这副透镜" }).click();
    }
    const before = await assistantTurns(page).count();
    await page.getByPlaceholder(/读完这一步|还想聊点什么/).fill("我读完了，接着来吧。");
    await page.getByRole("button", { name: "发送" }).click();
    await expect(assistantTurns(page)).toHaveCount(before + 1, { timeout: 180_000 });
    // 🚨 等这一轮真的说完再推下一句。
    //
    // 回话是边流边渲的：行一出现计数就 +1，可印记还在打字，发送键这时是
    // disabled。上一版没等，于是下一圈的点击撞在一个按不动的按钮上，15 秒后
    // 超时——看起来像"第三轮没来"，其实是我们没让它把话说完。
    // 今天日志里一轮 41 秒到 1 分 22 秒都有，所以预算给到和这一轮同一档。
    await expect(page.locator('[aria-label="印记正在打字"]')).toHaveCount(0, {
      timeout: 180_000,
    });
  }
  expect(
    await tappable.count(),
    "印记 handed no tappable (choose_span) card in five turns — nothing on screen can be answered by a tap alone",
  ).toBeGreaterThan(0);
  return tappable.first();
}

test("带读 hands her a card, she answers it with one tap, and it survives a refresh", async ({ page }) => {
  // Six or more live flagship calls, at 40–70s apiece.
  test.setTimeout(900_000);

  // Watch her side of every turn. `route.continue()` — the request really is
  // served by the API and really does reach the model; this only reads it on
  // the way past, so "she typed nothing" can be asserted on the wire rather
  // than inferred from an empty box.
  const turns: { text: string; picks?: unknown[]; cardAnswer?: { choice?: string } | null }[] = [];
  await page.route("**/api/v1/readings/*/coach", async (route) => {
    if (route.request().method() === "POST") turns.push(route.request().postDataJSON());
    await route.continue();
  });

  await startReading(page, ARTICLE_TITLE);
  await expect(page.locator("p[data-block-id]")).toHaveCount(4);

  // ── 「你读这篇是为了」 is gone, and nothing stands in that slot ────────────
  // Pro fills the top of the coach column with a brief bar; lite does not have
  // one at all (it was write-only here). Where she is in the plan lives on the
  // floating dial instead — and before 印记 has planned there is no position
  // to report, so the dial is not drawn either: 「第 0 步 / 共 0 步」, or an
  // empty ring, would be worse than the quiet.
  await expect(page.getByText("你读这篇是为了")).toHaveCount(0);
  const dial = page.locator(".mk-plandial__disc");
  await expect(dial).toHaveCount(0);

  // ── 开始: one live model call plans the route AND leads her into step one ──
  await page.getByRole("button", { name: "开始", exact: true }).click();
  await expect(assistantTurns(page)).toHaveCount(1, { timeout: 180_000 });
  await expect(page.locator('[aria-label="印记正在打字"]')).toHaveCount(0, {
    timeout: 180_000,
  });

  // Now she can see where she is — one step surface, present at every width
  // (the rail this replaced hung in an `lg:`-gated aside, i.e. it was simply
  // absent on a phone).
  await expect(dial).toBeVisible();
  await expect(dial).toHaveAttribute("aria-label", /带读进度 · (第 \d+ 步 \/ 共 \d+ 步|带读走完了)/);
  await expect(page.getByText("你读这篇是为了")).toHaveCount(0);

  // ── the first reply carries a CARD, not three lines of prose ──────────────
  // Acceptance 5 of the spec, and the whole reason the sub-project exists:
  // 「每一步都走同一条通道：一段散文，用打字回答」 was the complaint. This
  // assertion is what caught the prompt still only PERMITTING a card
  // (「有时候…更管用」): against a real model the opening turn came back as
  // 「先通读一遍…读完告诉我一声」, twice. See the report.
  //
  // Asserted INSIDE `[data-coach-log]`: the position is the design claim
  // (Task 8) — the card belongs in the conversation, right under the words
  // 印记 asked it with, not in a rail beside it.
  await expect(cardsInLog(page)).toHaveCount(1);
  await expectNothingReadsAsRightOrWrong(page, "first card");

  // ── EVERY option is verbatim in the article ───────────────────────────────
  // The guarantee the feature rests on. Read off the rendered options and
  // looked for in the rendered paragraphs — neither side is this file's own
  // constant, so a server that stopped validating would fail here even if the
  // article and the assertions agreed with each other.
  const found = await findChooseSpanCard(page);
  const prompt = (await found.locator("p").first().innerText()).trim();
  expect(prompt.length).toBeGreaterThan(0);
  // The stable handle. `found` is filtered on 「点一句就行」, which is exactly
  // the line that disappears when she answers — using it after the tap would
  // resolve to nothing. The QUESTION stays on the card for good, so it is what
  // the card is held by from here on.
  const card = cardsInLog(page).filter({ hasText: prompt });

  const options = card.locator("ul > li > button");
  const optionCount = await options.count();
  expect(optionCount, "a choose_span card carries 2–4 options").toBeGreaterThanOrEqual(2);
  expect(optionCount).toBeLessThanOrEqual(4);

  const paragraphs = await page.locator("p[data-block-id]").allInnerTexts();
  const optionTexts: string[] = [];
  for (let i = 0; i < optionCount; i++) {
    const quote = (await options.nth(i).innerText()).trim();
    expect(quote.length).toBeGreaterThanOrEqual(4);
    expect(
      paragraphs.some((p) => p.includes(quote)),
      `option ${i + 1} 「${quote}」 is not verbatim in any paragraph — she could answer this card without reading`,
    ).toBeTruthy();
    optionTexts.push(quote);
  }
  // Two options reading the same are not a choice; validateCoachCard dedupes,
  // including the nesting case (one option a substring of another).
  expect(new Set(optionTexts).size).toBe(optionCount);
  for (const a of optionTexts) {
    for (const b of optionTexts) {
      if (a !== b) expect(b.includes(a), `「${a}」 is nested inside 「${b}」`).toBeFalsy();
    }
  }

  await expectNothingReadsAsRightOrWrong(page, "card open");

  // ── she answers with a tap. Nothing typed. ────────────────────────────────
  const composer = page.getByPlaceholder(/读完这一步|还想聊点什么/);
  await expect(composer).toHaveValue("");
  const bubblesBefore = await page.locator('[data-role="student"]').count();
  const turnsBefore = turns.length;
  const chosen = optionTexts[0]!;
  await options.first().click();

  // The card grows in place: the options collapse into the one sentence she
  // chose, labelled as HERS. The others are deliberately not kept beside it —
  // a row of unchosen options with one marked out reads as an answer key.
  await expect(card.getByText("你选的")).toBeVisible();
  await expect(card.getByText(`“${chosen}”`)).toBeVisible();
  for (const other of optionTexts.slice(1)) {
    await expect(card.getByText(other, { exact: true })).toHaveCount(0);
  }

  // …and her tap really was the whole turn, on the wire.
  await expect.poll(() => turns.length, { timeout: 30_000 }).toBe(turnsBefore + 1);
  const tap = turns[turns.length - 1]!;
  expect(tap.text, "she typed nothing — the tap alone had to carry the turn").toBe("");
  expect(tap.picks ?? []).toHaveLength(0);
  expect(tap.cardAnswer?.choice).toBe(chosen);

  // ── 印记 answers HER CHOICE ───────────────────────────────────────────────
  // A live model's wording cannot be asserted, so what is asserted is the
  // mechanism that makes the next reply be about her choice: the sentence she
  // tapped is stored as her turn, `> `-prefixed line by line, and that stored
  // transcript is what the next prompt is built from (readingCoachTurnsWindow).
  // Any reply is then necessarily downstream of it.
  const replies = await assistantTurns(page).count();
  await expect(assistantTurns(page)).toHaveCount(replies + 1, { timeout: 180_000 });
  await expect(page.locator('[aria-label="印记正在打字"]')).toHaveCount(0, {
    timeout: 180_000,
  });
  const reply = (await assistantTurns(page).last().innerText()).trim();
  expect(reply.length).toBeGreaterThan(10);
  // Internal block ids are for the model's eyes; she has never seen a "b2".
  expect(reply).not.toMatch(/\bb\d+\b/);

  const readingId = new URL(page.url()).pathname.split("/").pop()!;
  const stored = await page.request
    .get(`/api/v1/readings/${readingId}/messages`)
    .then((r) => r.json() as Promise<{ messages: { role: string; content: string }[] }>);
  const tapped = stored.messages.filter((m) => m.role === "student" && m.content.includes(chosen));
  expect(tapped, "her tap did not become a stored turn").toHaveLength(1);
  // 🚨 Spec item 11, pinned because this mine has gone off once already: EVERY
  // line of the quoted half carries `> `. The lines that are not hers must all
  // be prefixed, or report_facts.go's stripQuotedLines lets the article's own
  // words through into 金句 printed under her name.
  const lines = tapped[0]!.content.split("\n").filter((l) => l.trim() !== "");
  expect(lines.length).toBeGreaterThan(0);
  for (const line of lines) {
    expect(line.startsWith(">"), `her stored turn has an unquoted line: ${line}`).toBeTruthy();
  }

  // Her tap adds NO bubble of its own: the words are on the card above, and
  // saying them twice (once in the raw `> ` form) is what `ownWords` exists to
  // prevent.
  await expect(page.locator('[data-role="student"]')).toHaveCount(bubblesBefore);

  await expectNothingReadsAsRightOrWrong(page, "card answered");
  await expect(page.getByRole("alert")).toHaveCount(0);

  // ── refresh: the card and her answer are both still there ─────────────────
  // Neither is React state — both are rebuilt out of `atom_message.payload`.
  await page.reload();
  await expect(page.locator(".mk-reading-room")).toBeVisible({ timeout: 30_000 });
  const restored = cardsInLog(page).filter({ hasText: prompt });
  await expect(restored).toHaveCount(1);
  await expect(restored.getByText("你选的")).toBeVisible();
  await expect(restored.getByText(`“${chosen}”`)).toBeVisible();
  // Answered means answered: the options do not come back waiting for a tap
  // she has already made.
  await expect(restored.locator("ul > li > button")).toHaveCount(0);
  await expect(page.locator(".mk-plandial__disc")).toBeVisible();
  await expectNothingReadsAsRightOrWrong(page, "after reload");

  // ── a lens still summons, with chat cards on screen ───────────────────────
  // The one-open-card index (`atom_card_one_open_idx`, 0096) permits ONE open
  // card per atom. Chat cards were kept off `atom_card` for exactly this
  // reason; the day someone "tidies" them into that table, every lens summon
  // after the first card deadlocks. Nothing but a live summon proves it.
  await expect(cardsInLog(page).first()).toBeVisible();
  await page.getByRole("button", { name: /^透镜库 · \d+$/ }).click();
  const library = page.getByRole("dialog", { name: "透镜库" });
  const firstLens = library.locator(".mk-lens-library__row").first();
  const lensName = (await firstLens.locator(".mk-lens-library__row-name").textContent())?.trim() ?? "";
  expect(lensName.length).toBeGreaterThan(0);
  await firstLens.click();

  const lensCard = page.locator(".lens-connector").locator("..");
  await expect(lensCard).toBeVisible({ timeout: 180_000 });
  await expect(lensCard.getByText(lensName)).toBeVisible();
  // The chat card did not go anywhere, and no summon error was raised.
  await expect(restored).toHaveCount(1);
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expectNothingReadsAsRightOrWrong(page, "lens summoned over a chat card");
});
