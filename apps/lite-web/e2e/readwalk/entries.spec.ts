import { test, type Page, type BrowserContext } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { readScreen, screenKey } from "./screen";
import { think } from "./brain";

/**
 * 阅读室的每一个入口，从进门走到「完成 → 报告 → 继续阅读 → 再完成」。
 *
 * 2026-09-17 这一天阅读室收了四批反馈。这条 walk 按入口走一遍，把每一条反馈
 * 变成一项**看得见的检查**，记在 findings 里（ok / bad / look —— look 是要人
 * 看截图或原话判断的那一类）。它不断言：一条 bad 不该让后面的检查不跑。
 *
 * 入口（ENTRY）：
 *   text       阅读页粘一篇 18 段的英文 —— 长文通读截断那一条
 *   link       阅读页贴一个 Smithsonian 链接 —— 「You Might Also Like」那一条
 *   upload     阅读页上传一份中文议论文 DOCX —— 透镜/论证板那一条
 *   library    分级阅读库读一篇 PRO/CON —— 驳论那一套格子
 *   planet     星图点一颗星「现在读」（Smithsonian）
 *   abstract   星图点那颗只有摘要的星（Quanta）—— 摘要弹窗那一条
 *
 * 跑法：
 *   cd apps/lite-web && ENTRY=text npx playwright test --config e2e/readwalk/playwright.config.ts entries
 */

test.use({ trace: "off", video: "off" });

const API = process.env.E2E_API_BASE ?? "https://mind-api.uni-robot.cn";
const BASE = process.env.E2E_BASE_URL ?? "https://mind-lite.uni-robot.cn";
const JOIN = process.env.E2E_JOIN_CODE ?? "G624-UXFE";
const ENTRY = process.env.ENTRY ?? "text";
const OUT = path.join(process.env.READWALK_OUT ?? "e2e/.readwalk", `entry-${ENTRY}`);
const STEPS = Number(process.env.READWALK_STEPS ?? 60);
const FIX = "e2e/readwalk/fixtures";

const LINK_URL =
  "https://www.smithsonianmag.com/smart-news/a-66-million-year-old-feather-trapped-in-dinosaur-poop-might-help-explain-why-some-birds-survived-mass-extinction-180989496/";
const PLANET_URL_PART = "this-new-brain-map-shows-every-nerve-cell";
const ABSTRACT_URL_PART = "ctenophores-arent-just-beautiful";
const LIBRARY_TITLE = "PRO/CON: Data Centers";

type Finding = { check: string; verdict: "ok" | "bad" | "look"; detail: string };
const findings: Finding[] = [];
function note(check: string, verdict: Finding["verdict"], detail = "") {
  findings.push({ check, verdict, detail });
  console.log(`  ${verdict === "ok" ? "✅" : verdict === "bad" ? "❌" : "👀"} ${check}${detail ? " — " + detail : ""}`);
}

let shot = 0;
async function snap(page: Page, name: string, full = false) {
  shot++;
  await page
    .screenshot({ path: path.join(OUT, `${String(shot).padStart(2, "0")}-${name}.png`), fullPage: full })
    .catch(() => {});
}

async function settle(page: Page) {
  await page
    .waitForFunction(
      () =>
        !document.querySelector('[aria-label="印记正在打字"]') &&
        !document.querySelector("button[aria-busy]"),
      null,
      { timeout: 180_000 },
    )
    .catch(() => {});
  await page.waitForTimeout(400);
}

async function api<T>(ctx: BrowserContext, p: string): Promise<T | null> {
  const r = await ctx.request.get(`${API}${p}`);
  if (!r.ok()) return null;
  return (await r.json()) as T;
}

type Task = { id: string; kind: string; label: string; detail: string; blockId: string; status: string };
type Plan = { routineKey: string; routineName: string; tasks: Task[] };
type Source = { title: string; body?: string; blocks?: { id: string; text: string }[]; excerptOnly?: boolean };

/** 划一句（平时的划选路径 —— 透镜已经撤掉了）。 */
async function selectText(page: Page, paragraph: number, want: string): Promise<boolean> {
  const p = page.locator(".mk-reading-room__article-inner p[data-block-id]").nth(paragraph - 1);
  if (!(await p.count())) return false;
  await p.scrollIntoViewIfNeeded();
  return p.evaluate((el, want) => {
    const text = el.textContent ?? "";
    let at = text.indexOf(want);
    if (at < 0) {
      const head = want.slice(0, 24);
      at = head.length >= 6 ? text.indexOf(head) : -1;
      if (at < 0) return false;
      want = text.slice(at, Math.min(text.length, at + want.length));
    }
    const walk = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
    let seen = 0;
    let sN: Node | null = null;
    let sO = 0;
    let eN: Node | null = null;
    let eO = 0;
    const end = at + want.length;
    for (let n = walk.nextNode(); n; n = walk.nextNode()) {
      const len = (n.textContent ?? "").length;
      if (!sN && seen + len > at) {
        sN = n;
        sO = at - seen;
      }
      if (sN && seen + len >= end) {
        eN = n;
        eO = end - seen;
        break;
      }
      seen += len;
    }
    if (!sN || !eN) return false;
    const range = document.createRange();
    range.setStart(sN, sO);
    range.setEnd(eN, eO);
    const sel = window.getSelection();
    if (!sel) return false;
    sel.removeAllRanges();
    sel.addRange(range);
    const r = range.getBoundingClientRect();
    const opts = { bubbles: true, clientX: r.left + r.width / 2, clientY: r.top + r.height / 2 };
    el.dispatchEvent(new PointerEvent("pointerup", opts));
    el.dispatchEvent(new MouseEvent("mouseup", opts));
    return true;
  }, want);
}

const LEAK = /\b(advance|done|pending|skipped|task_id|block_id)\b/i;

test(`阅读室入口：${ENTRY}`, async ({ browser }) => {
  test.setTimeout(Number(process.env.READWALK_MS ?? 90 * 60_000));
  fs.mkdirSync(OUT, { recursive: true });
  const tag = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
  const email = `entry-${ENTRY}-${tag}@demo.mindimprint.local`;
  const password = `entry-${tag}-pass`;
  const ctx = await browser.newContext({ baseURL: BASE, viewport: { width: 1440, height: 1000 } });
  const up = await ctx.request.post(`${API}/api/v1/auth/signup`, {
    data: { email, password, display_name: "入口走查", join_code: JOIN },
  });
  if (!up.ok()) throw new Error(`signup ${up.status()} ${await up.text()}`);
  await ctx.request.post(`${API}/api/v1/auth/signin`, { data: { email, password } });
  const page = await ctx.newPage();

  // ─────────────────────────── 1. 进门 ───────────────────────────
  console.log(`\n=== 入口 ${ENTRY} ===`);
  if (ENTRY === "text" || ENTRY === "link" || ENTRY === "upload") {
    await page.goto("/readings");
    await page.getByLabel("文章正文或链接").waitFor({ timeout: 30_000 });
    if (ENTRY === "text") {
      // READWALK_FIXTURE / READWALK_TITLE：换一篇文章走同一条路（2026-09-18 起
      // 读法跟着体裁走，所以议论文之外的三种体裁各要走一遍）。
      await page.getByLabel("阅读的名字").fill(process.env.READWALK_TITLE ?? "停车场与公园");
      await page
        .getByLabel("文章正文或链接")
        .fill(fs.readFileSync(`${FIX}/${process.env.READWALK_FIXTURE ?? "long-article.txt"}`, "utf8"));
      await page.getByRole("button", { name: /开始阅读/ }).first().click();
    } else if (ENTRY === "link") {
      await page.getByLabel("文章正文或链接").fill(LINK_URL);
      await page.getByRole("button", { name: /开始阅读/ }).first().click();
    } else {
      await page.locator('input[type="file"]').first().setInputFiles(`${FIX}/zh-essay.docx`);
    }
  } else if (ENTRY === "library") {
    await page.goto("/readings/library");
    const card = page.locator("article", { hasText: LIBRARY_TITLE }).first();
    await card.waitFor({ timeout: 30_000 });
    await card.scrollIntoViewIfNeeded();
    await snap(page, "library-card");
    await card.locator(".reading-library-start").click();
  } else {
    const today = await api<{ planets: { titleZh: string; url: string }[] }>(ctx, "/api/v1/explore/today");
    const want = ENTRY === "abstract" ? ABSTRACT_URL_PART : PLANET_URL_PART;
    const planet = today?.planets.find((p) => p.url.includes(want));
    if (!planet) {
      note("星图上有要走的那颗星", "bad", `今天的星图里没有 ${want}`);
      throw new Error("no planet");
    }
    await page.goto("/");
    await page.getByRole("navigation", { name: "主导航" }).getByRole("button", { name: "探索" }).click();
    const star = page
      .getByRole("navigation", { name: "今日发现" })
      .getByRole("button", { name: planet.titleZh.slice(0, 12) })
      .first();
    await star.waitFor({ timeout: 45_000 });
    await star.click();
    // 选中一条只是把它摆到中间；「探索这件事」才打开那一页。
    await page.getByRole("button", { name: "探索这件事" }).click();
    await page.getByRole("button", { name: "现在读" }).waitFor({ timeout: 20_000 });
    await snap(page, "planet-sheet");
    await page.getByRole("button", { name: "现在读" }).click();
  }
  await page.waitForURL(/\/readings\/[0-9a-f-]{36}/, { timeout: 120_000 });
  const id = page.url().match(/readings\/([0-9a-f-]{36})/)![1];
  console.log(`reading ${id}`);
  await page.locator(".mk-reading-room__article").first().waitFor({ timeout: 60_000 });
  await page.waitForTimeout(2500);
  await snap(page, "entered");

  let src = await api<Source>(ctx, `/api/v1/readings/${id}/source`);
  const paraCount = await page.locator(".mk-reading-room__article-inner p[data-block-id]").count();
  note("正文进来了", paraCount > 0 ? "ok" : "bad", `${paraCount} 段；excerptOnly=${src?.excerptOnly}`);

  if (ENTRY === "link" || ENTRY === "planet") {
    const body = (src?.blocks ?? []).map((b) => b.text).join("\n") || (src?.body ?? "");
    const trailer = /you (might|may) also like/i.test(body);
    note("正文切掉了「You Might Also Like」", trailer ? "bad" : "ok", `正文 ${body.length} 字符；末段：${body.slice(-120)}`);
  }

  // ─────────────────────── 2. 只有摘要的弹窗 ───────────────────────
  if (ENTRY === "abstract") {
    const dlg = page.getByRole("dialog").first();
    const shown = await dlg.isVisible().catch(() => false);
    note("只有摘要：一进门就弹窗", shown ? "ok" : "bad", `excerptOnly=${src?.excerptOnly}`);
    if (shown) {
      const t = await dlg.innerText();
      note("弹窗里有标题", /栉水母|Ctenophore/i.test(t) ? "ok" : "bad");
      note("弹窗说明只拿到摘要并问要不要读全文", /只获取到了这篇文章的摘要/.test(t) && /想继续读全文吗/.test(t) ? "ok" : "bad");
      note("摘要里去掉了 Source 行", /^\s*Source\b/m.test(t) ? "bad" : "ok");
      const why = dlg.getByLabel("为什么拿不到全文");
      if (await why.count()) {
        await why.hover();
        await page.waitForTimeout(500);
        const tip = await dlg.locator('[role="tooltip"]').innerText().catch(() => "");
        note("「?」悬停说明为什么拿不到全文", tip.length > 10 ? "ok" : "bad", tip.slice(0, 80));
      } else note("「?」悬停说明为什么拿不到全文", "bad", "没有那个问号");
      await snap(page, "abstract-modal");
      await dlg.getByRole("button", { name: "继续读全文" }).click();
      await page.waitForTimeout(600);
      const t2 = await dlg.innerText();
      note("继续读全文：上传或粘贴两种方式", /方式一/.test(t2) && /方式二/.test(t2) ? "ok" : "bad");
      await snap(page, "abstract-modal-2");
      const planBefore = await api<Plan>(ctx, `/api/v1/readings/${id}/plan`);
      // 太短的一段 → 提醒复制的是不是全文
      const box = dlg.getByLabel("文章全文");
      await box.fill("Ctenophores are strange animals.");
      await dlg.getByRole("button", { name: /保存|确认|开始/ }).last().click().catch(() => {});
      await page.waitForTimeout(800);
      const warn = await dlg.innerText().catch(() => "");
      note("粘得太短 → 请确认复制的是文章全文", /请确认复制的是文章全文/.test(warn) ? "ok" : "bad");
      await box.fill(fs.readFileSync(`${FIX}/long-article.txt`, "utf8"));
      await snap(page, "abstract-modal-filled");
      await dlg.getByRole("button", { name: /保存|确认|开始/ }).last().click();
      await dlg.waitFor({ state: "hidden", timeout: 180_000 }).catch(() => {});
      await page.waitForTimeout(2500);
      src = await api<Source>(ctx, `/api/v1/readings/${id}/source`);
      const after = await page.locator(".mk-reading-room__article-inner p[data-block-id]").count();
      const planAfter = await api<Plan>(ctx, `/api/v1/readings/${id}/plan`);
      const sigOf = (p: Plan | null) => JSON.stringify(p?.tasks.map((t) => t.id) ?? []);
      note(
        "贴完全文：正文换掉、摘要标记清掉、按全文重排读法",
        after >= 18 && !src?.excerptOnly && (planAfter?.tasks.length ?? 0) > 0 && sigOf(planAfter) !== sigOf(planBefore)
          ? "ok"
          : "bad",
        `段数 ${after}，excerptOnly=${src?.excerptOnly}，读法 ${planBefore?.tasks.length ?? 0}→${planAfter?.tasks.length ?? 0} 步：` +
          (planAfter?.tasks.map((t) => t.label).join(" / ") ?? ""),
      );
      note("「打开原文」链接还在", (await page.getByText("打开原文").count()) > 0 ? "ok" : "look");
      await snap(page, "abstract-replaced");
    }
  }

  // ─────────────────────── 3. 模型学生读到读法走完 ───────────────────────
  const aiSeen = new Set<string>();
  const recent: string[] = [];
  let lastKey = "";
  let sameFor = 0;
  let noteText: string | undefined;
  let lastPlanSig = "";
  let plan: Plan | null = null;
  const turnsOnTask = new Map<string, number>();
  let guidanceSeen = 0;
  let readSteps = 0;
  const cardAskPairs: string[] = [];
  let finished = false;
  let helpAsked = 0;
  let openCardSteps = 0;
  let highlightedSteps = 0;
  const binsSeen = new Set<string>();

  for (let step = 0; step < STEPS; step++) {
    await settle(page);

    // —— 读法（服务端真相） ——
    plan = await api<Plan>(ctx, `/api/v1/readings/${id}/plan`);
    const sig = JSON.stringify(plan?.tasks.map((t) => [t.label, t.status]));
    if (plan && plan.tasks.length && sig !== lastPlanSig) {
      if (!lastPlanSig) {
        const lines = plan.tasks.map((t, i) => `${i + 1}. [${t.kind}] ${t.label}`).join("\n");
        console.log(`读法 ${plan.routineName}（${plan.tasks.length} 步）:\n${lines}`);
        fs.writeFileSync(path.join(OUT, "plan.json"), JSON.stringify(plan, null, 2));
      }
      lastPlanSig = sig;
    }
    const cur = plan?.tasks.find((t) => t.status === "pending");
    if (cur) turnsOnTask.set(cur.id, (turnsOnTask.get(cur.id) ?? 0) + 1);

    const body = await page.locator("body").innerText().catch(() => "");
    if (cur?.kind === "read" && /现在读这一部分/.test(body)) guidanceSeen++;
    if (cur?.kind === "read") readSteps++;

    // —— 印记说的新话 ——
    const bubbles = await page.locator('[data-role="assistant"]').allInnerTexts().catch(() => [] as string[]);
    for (const b of bubbles) {
      const t = b.trim();
      if (!t || aiSeen.has(t)) continue;
      aiSeen.add(t);
      if (LEAK.test(t)) note("印记的话里没有内部词", "bad", t.slice(0, 160));
      if (/她/.test(t.replace(/她们/g, ""))) note("印记当面不称「她」", "look", t.slice(0, 160));
      if (/透镜/.test(t)) note("印记不再提透镜", "look", t.slice(0, 160));
    }

    // —— 这一屏上的旧东西 ——
    if (await page.getByRole("button", { name: "带我过去" }).isVisible().catch(() => false))
      note("透镜已撤掉", "bad", `第 ${step} 步还出现了透镜横幅`);
    if (/(^|\n)\s*(在问|中心思想|分几部分|承重)\s*(\n|$)/.test(body)) note("导读用新名字", "bad", "还有旧标签");
    if (await page.getByRole("button", { name: "修改答案" }).count()) note("没有「修改答案」按钮", "bad");
    const composer = page.locator("[data-coach-log] ~ * textarea, textarea").last();
    if ((await composer.count()) && (await composer.isDisabled().catch(() => false))) {
      const busy = await page.locator('[aria-label="印记正在打字"], button[aria-busy]').count();
      if (!busy) note("对话框任何时候都能打字", "bad", `第 ${step} 步输入框是锁住的`);
    }
    // 卡片上的要求 vs 印记最后一句
    const cards = page.locator('[data-chat-row="card"]');
    const nCards = await cards.count();
    if (nCards) {
      const lastCard = (await cards.nth(nCards - 1).innerText().catch(() => "")).slice(0, 200);
      const lastAi = bubbles.at(-1)?.slice(-200) ?? "";
      const pair = `AI：${lastAi}\n卡：${lastCard}`;
      if (!cardAskPairs.includes(pair)) cardAskPairs.push(pair);
      // 同事 2026-09-18 第 4 条：卡片上摆着的句子要在正文里标出来。
      // 「敞开」= 最后一张卡还没作答（没有「你摆的 / 你选的」）而且摆着带段号的句子。
      if (!/你摆的|你选的|你排的/.test(lastCard) && /第\s*\d+\s*段/.test(lastCard)) {
        openCardSteps++;
        if (await page.locator("[data-card-quote]").count()) highlightedSteps++;
      }
      const boardText = await page.locator(".mk-board").last().innerText().catch(() => "");
      for (const bin of ["论点", "论据", "论证", "关键主张", "证据", "作者观点"]) {
        if (new RegExp(`(^|\\n)${bin}(\\n|$)`).test(boardText)) binsSeen.add(bin);
      }
    }

    if (/读法已全部完成/.test(body)) {
      finished = true;
      console.log(`[${step}] ✅ 读法走完了`);
      await snap(page, "plan-finished");
      break;
    }

    const screen = await readScreen(page);
    const key = screenKey(screen);
    sameFor = key === lastKey ? sameFor + 1 : 0;
    lastKey = key;
    noteText = sameFor >= 2 ? "上一步之后屏幕没有变化。" : undefined;
    const beat = await think({ screen, recent, note: noteText });
    const a = beat.action;
    console.log(
      `[${step}] ${cur ? `「${cur.label}」` : "（无读法）"} ${a.kind}` +
        `${a.kind === "say" ? "：" + a.text.slice(0, 30) : ""}${beat.snag ? ` · snag: ${beat.snag}` : ""}`,
    );
    recent.push(`${a.kind}${a.kind === "say" ? "：" + a.text.slice(0, 40) : ""}`);
    if (recent.length > 6) recent.shift();
    if (step % 6 === 0) await snap(page, `walk-${step}`);

    if (a.kind === "stuck" || a.kind === "leave") {
      await snap(page, `stuck-${step}`);
      // 真学生卡住了会开口问。对话框任何时候都能打字 —— 这本身是一条被报过的 bug。
      if (a.kind === "stuck" && helpAsked < 3) {
        helpAsked++;
        note("模型学生卡住（向印记求助）", "look", `第 ${step} 步：${a.why}`);
        const box = page.locator("textarea:visible").last();
        await box.fill(`我不知道现在该做什么：${a.why.slice(0, 80)}`);
        await box.press("Enter");
        await page.waitForTimeout(900);
        continue;
      }
      note("模型学生走到读法结束", "bad", `第 ${step} 步 ${a.kind}：${a.kind === "stuck" ? a.why : ""}`);
      break;
    }
    if (a.kind === "wait") {
      await page.waitForTimeout(2500);
      continue;
    }
    if (a.kind === "say") {
      // 卡片开着且自带输入框时，答案写进卡片 —— 真学生面前就是那个框。
      const cardBox = page.locator('[data-chat-row="card"] textarea:visible:enabled').last();
      const box = (await cardBox.count()) ? cardBox : page.locator("textarea:visible").last();
      if (!(await box.count())) {
        noteText = "屏幕上没有能打字的地方。";
        continue;
      }
      await box.fill(a.text);
      // 回车就能发 —— 这本身就是一条被报过的 bug（动手部分回车发不出去）。
      await box.press("Enter");
      await page.waitForTimeout(900);
      const still = await box.inputValue().catch(() => "");
      if (still.trim() === a.text.trim()) note("回车发送", "bad", `第 ${step} 步按回车后字还在框里`);
      continue;
    }
    if (a.kind === "click") {
      const b = page.locator("button:visible").nth(a.button);
      const label = (await b.innerText().catch(() => "")).trim();
      if (label === "完成这篇") {
        noteText = "这一篇还没读完（进度盘上还有没做的步骤），先别点「完成这篇」。";
        continue;
      }
      if (await b.count()) await b.click({ timeout: 8000 }).catch(() => {});
      await page.waitForTimeout(800);
      continue;
    }
    if (a.kind === "pick") {
      const ok = await selectText(page, a.paragraph, a.sentence ?? "");
      if (!ok) noteText = `第${a.paragraph}段里找不到那句话（抄歪了）`;
      await page.waitForTimeout(600);
      continue;
    }
    if (a.kind === "place") {
      // 🚨 和 screen.ts 数的必须是**同一块板**：还开着、不是排序板的那块。
      // 只在描述那一侧排除交过的板、动作这一侧仍然全页数，格子的下标就错位了 ——
      // 她点的是屏幕上那块板的「动作描写」，runner 点进的是上面一块交过的板
      // （2026-09-18 记叙文复走，同一个摆放连着二十步没生效）。
      const live = page.locator(".mk-board:not(.mk-order):not(.is-done)").last();
      const want = screen.board?.chips[a.chip]?.text ?? "";
      const chip = want
        ? live.locator(".mk-board__chip", { hasText: want }).first()
        : live.locator(".mk-board__chip").nth(a.chip);
      const bin = live.locator(".mk-board__bin").nth(a.bin);
      if ((await chip.count()) && (await bin.count())) {
        await chip.click();
        await bin.click();
      }
      await page.waitForTimeout(400);
    }
  }
  await snap(page, "walk-end");
  fs.writeFileSync(path.join(OUT, "card-vs-chat.txt"), cardAskPairs.join("\n\n———\n\n"));
  fs.writeFileSync(path.join(OUT, "ai-said.txt"), [...aiSeen].join("\n\n———\n\n"));

  // ── 同事 2026-09-18 那一批，一条一个检查 ──
  note(
    "卡片上的句子在正文里标出来",
    openCardSteps === 0 ? "look" : highlightedSteps > 0 ? "ok" : "bad",
    `敞开的卡 ${openCardSteps} 步，正文有标记 ${highlightedSteps} 步`,
  );
  const stars = [...aiSeen].filter((t) => t.includes("**"));
  note("印记的话里没有漏出来的 **", stars.length ? "bad" : "ok", stars[0]?.slice(0, 160) ?? "");
  if (binsSeen.size) {
    const argumentBins = ["论点", "论据", "论证"].some((b) => binsSeen.has(b));
    const oldBins = ["关键主张", "作者观点"].some((b) => binsSeen.has(b));
    note("议论文的板是 论点 / 论据 / 论证", oldBins ? "bad" : argumentBins ? "ok" : "look", [...binsSeen].join(" / "));
  }
  // 第 9 条：做完一步不以「这一步做完」收尾、等她回一句「好」。
  const dangling = [...aiSeen].filter((t) => /(这一步(就)?做完(了)?|往下走)[。！!]?\s*$/.test(t.trim()));
  note("做完一步的那一句同时交下一步（不以「这一步做完」收尾）", dangling.length ? "bad" : "ok", dangling[0]?.slice(-120) ?? "");
  // 第 5 条：卡片题目不是「请点出你想说的那一句」这种不问事的话。
  const vague = cardAskPairs.filter((p) => /点出你想说的那一句|请用你自己的话写一句。/.test(p));
  note("卡片题目是一道题", vague.length ? "bad" : "ok", vague[0]?.slice(-160) ?? "");
  // 第 7 条：她发过的话，悬停出编辑按钮，点了字回到输入框。
  const mineRows = page.locator('[data-chat-row="student"]');
  if (await mineRows.count()) {
    const row = mineRows.last();
    await row.scrollIntoViewIfNeeded();
    await row.hover();
    const edit = row.getByRole("button", { name: "编辑后重新发送" });
    if (await edit.count()) {
      const said = (await row.locator('[data-role="student"]').innerText()).trim();
      await edit.click();
      const box = page.locator("textarea:visible").last();
      const v = (await box.inputValue()).trim();
      note("她发过的话能放回输入框再编辑", v && said.includes(v.slice(0, 10)) ? "ok" : "bad", `气泡「${said.slice(0, 40)}」；输入框「${v.slice(0, 40)}」`);
      await snap(page, "edit-resend");
      await box.fill("");
    } else note("她发过的话能放回输入框再编辑", "bad", "悬停后没有编辑按钮");
  }

  // ─────────────────────── 4. 读法本身的形状 ───────────────────────
  plan = await api<Plan>(ctx, `/api/v1/readings/${id}/plan`);
  const tasks = plan?.tasks ?? [];
  fs.writeFileSync(path.join(OUT, "plan-final.json"), JSON.stringify(plan, null, 2));
  note("模型学生走完了读法", finished ? "ok" : "bad", `${tasks.filter((t) => t.status !== "pending").length}/${tasks.length} 步`);
  const reads = tasks.filter((t) => t.kind === "read");
  const nParas = (src?.blocks ?? []).length || paraCount;
  const ranges = reads.map((t) => [...`${t.label} ${t.detail}`.matchAll(/(\d+)\s*[–-]\s*(\d+)\s*段|第\s*(\d+)\s*段/g)]);
  const lastCovered = Math.max(
    0,
    ...ranges.flat().map((m) => Number(m[2] ?? m[3] ?? 0)),
  );
  note(
    "通读切成几部分（每步 2–4 段）并覆盖到最后一段",
    reads.length >= 2 && lastCovered >= nParas ? "ok" : reads.length >= 2 ? "look" : "bad",
    `通读 ${reads.length} 步：${reads.map((t) => t.label).join(" / ")}；共 ${nParas} 段，最后到第 ${lastCovered} 段`,
  );
  const focus = tasks.filter((t) => t.kind === "focus_block");
  note("精读挑了几段就走几步", "look", `精读 ${focus.length} 步：${focus.map((t) => t.label).join(" / ")}`);
  note("清单里没有透镜", tasks.some((t) => t.kind === "lens") ? "bad" : "ok", tasks.map((t) => t.kind).join(","));
  // 数的是**标注那一步**（kind=label），不是名字里带「论证/观点」的步骤 ——
  // en-argument 的「观点变化」是链接经验那一步，而通读的部分标题是模型起的，
  // 「通读第4–6段·论证与转折」也会被一个按名字的正则算进来（2026-09-18 误报）。
  const boards = tasks.filter((t) => t.kind === "label");
  note("论证练习只有一次", boards.length <= 2 ? "ok" : "bad", boards.map((t) => t.label).join(" / "));
  note("通读时文章上有「现在读这一部分」指引", guidanceSeen > 0 ? "ok" : readSteps ? "bad" : "look", `通读步里看到 ${guidanceSeen}/${readSteps} 次`);
  const stuckTask = [...turnsOnTask.entries()].sort((a, b) => b[1] - a[1])[0];
  if (stuckTask) {
    const t = tasks.find((x) => x.id === stuckTask[0]);
    note("同一步不反复", stuckTask[1] > 12 ? "look" : "ok", `停留最久的一步「${t?.label}」${stuckTask[1]} 个回合`);
  }
  // 导读 2026-09-18 起只在清单走完之后出现，当「全文总结」（同事的第 1 条）。
  const allDone = tasks.length > 0 && tasks.every((t) => t.status !== "pending");
  const outlineInCoach = await page.locator('[data-coach-log] [aria-label="全文总结"]').count();
  const outlineText = outlineInCoach ? await page.locator('[aria-label="全文总结"]').first().innerText() : "";
  note(
    "全文总结在清单走完之后出现（核心问题/关键结论/结构）",
    allDone ? (outlineInCoach && /核心问题|关键结论/.test(outlineText) ? "ok" : "bad") : outlineInCoach ? "bad" : "ok",
    allDone ? outlineText.slice(0, 120).replace(/\n/g, " ") : `清单未走完，总结${outlineInCoach ? "提前出现了" : "未出现"}`,
  );
  // 答过的选句卡：选项都在、标着「你选的」
  const chosen = page.locator('[data-chat-row="card"]', { hasText: "你选的" });
  const nChosen = await chosen.count();
  if (nChosen) {
    const t = await chosen.first().innerText();
    const opts = (t.match(/第\s*\d+\s*段/g) ?? []).length;
    note("答过的选句卡留着全部选项、标出你选的、选项带段号", opts >= 2 ? "ok" : "look", t.slice(0, 200).replace(/\n/g, " / "));
    await chosen.first().scrollIntoViewIfNeeded();
    await snap(page, "answered-card");
  } else note("答过的选句卡留着全部选项", "look", "这一篇没出现选句卡");

  // ─────────────────────── 5. 段落工具条 ───────────────────────
  if (!(await page.locator(".mk-reading-room__article").count())) {
    await page.goto(`/readings/${id}`);
    await Promise.race([page.locator(".mk-reading-room__article").first().waitFor({ timeout: 60_000 }), page.getByRole("region", { name: "学习数据概览" }).waitFor({ timeout: 60_000 })]).catch(() => {});
    const again0 = page.getByRole("button", { name: "继续阅读" });
    if (await again0.count()) {
      note("走查途中提前完成了，先继续阅读再测工具", "look");
      await again0.click();
      await page.locator(".mk-reading-room__article").first().waitFor({ timeout: 60_000 });
    }
  }
  await page.keyboard.press("Escape");
  const lens = await page
    .locator(".mk-reading-room__article-inner p[data-block-id]")
    .evaluateAll((els) => els.slice(0, 8).map((e) => (e.textContent ?? "").length));
  const pIdx = lens.indexOf(Math.max(...lens));
  const para = page.locator(".mk-reading-room__article-inner p[data-block-id]").nth(pIdx);
  await para.scrollIntoViewIfNeeded();
  await para.click({ position: { x: 40, y: 10 } });
  const bar = page.locator(".mk-blockbar");
  await bar.waitFor({ timeout: 10_000 }).catch(() => {});
  const tools = (await bar.locator("button").allInnerTexts().catch(() => [] as string[])).map((s) => s.trim()).filter(Boolean);
  note("工具条：没有把握度", tools.includes("把握度") ? "bad" : "ok", tools.join(" / "));
  await snap(page, "toolbar");

  async function tool(name: string): Promise<boolean> {
    if (!(await bar.isVisible().catch(() => false))) {
      await para.click({ position: { x: 40, y: 10 } });
      await bar.waitFor({ timeout: 8000 }).catch(() => {});
    }
    const b = bar.getByRole("button", { name, exact: true });
    if (!(await b.count())) return false;
    await b.click();
    return true;
  }

  // 中文文章（上传入口，或者文本入口粘进来的是中文）上没有查词 / 句子解析 —— 那两件是英文工具。
  const zh = ENTRY === "upload" || /[\u4e00-\u9fff]{4}/.test(await para.innerText());
  // 2026-09-18 产品负责人：「don't add the two word/sentence level to paragraph level.」
  note(
    "段落工具条上没有词、句两级的工具（查词 / 句子解析 / 语法）",
    tools.some((t) => /查词|句子解析|^语法$/.test(t)) ? "bad" : "ok",
    tools.join(" / "),
  );
  note("段落工具条上没有「拆开这一段」", (await bar.innerText().catch(() => "")).includes("拆开这一段") ? "bad" : "ok");
  // 同事 2026-09-18 第 6 条：结构解析要在段落内按句子拆层次，不是整段概述一句。
  if (zh && (await tool("结构解析"))) {
    const items = page.locator("[data-block-tools] ol li");
    await items.first().waitFor({ timeout: 90_000 }).catch(() => {});
    const n = await items.count();
    const txt = n ? (await items.allInnerTexts()).join(" / ") : "";
    note("结构解析按句子拆成几个层次", n >= 2 ? "ok" : "bad", `${n} 层：${txt.slice(0, 200)}`);
    await snap(page, "structure");
  }

  // 想一想：输入框，Shift+回车换行，回车交上去
  for (const name of ["想一想", "仿写"]) {
    if (!(await tool(name))) {
      note(`工具条上有${name}`, "bad");
      continue;
    }
    // 🚨 上一件工具的面板在新的一件出来之前还留着（连同它的输入框）—— 要等标着
    // 这件工具名字的那一块，否则仿写的答案会写进想一想的框里。
    const input = page
      .locator("[data-block-tools]")
      .filter({ has: page.getByText(name, { exact: true }) })
      .locator(".mk-tool-answer__input")
      .first();
    await input.waitFor({ timeout: 90_000 }).catch(() => {});
    await page.waitForTimeout(500);
    if (!(await input.count()) || !(await input.isEditable().catch(() => false))) {
      await snap(page, `tool-${name}-noinput`);
      note(`${name}：下面有输入框`, "bad");
      continue;
    }
    const before = await page.locator('[data-role="assistant"]').count();
    await input.click();
    await input.pressSequentially(zh ? "我觉得作者说得有道理" : "I think parking should not be free");
    await input.press("Shift+Enter");
    await input.pressSequentially(zh ? "但也要看是什么书" : "because it wastes land");
    const v = await input.inputValue();
    note(`${name}：Shift+回车换行`, v.includes("\n") ? "ok" : "bad", JSON.stringify(v));
    await input.press("Enter");
    // 交上去之后，她那段话带着「想一想 · 第N段」出现在对话里。
    const mine = page.locator('[data-role="student"]', { hasText: `${name} ·` });
    await mine.first().waitFor({ timeout: 15_000 }).catch(() => {});
    const sent = await mine.count();
    note(`${name}：回车交上去`, sent ? "ok" : "bad");
    await settle(page);
    await page.waitForTimeout(3000);
    await settle(page);
    const after = await page.locator('[data-role="assistant"]').count();
    const reply = after > before ? (await page.locator('[data-role="assistant"]').last().innerText()).slice(0, 200) : "";
    note(`${name}：印记给了反馈`, reply ? "look" : "bad", reply.replace(/\n/g, " "));
    await snap(page, `tool-${name}`);
  }

  // 划一句 / 划一个词 → 弹出的工具
  await page.keyboard.press("Escape");
  const p3text = (await para.innerText()).trim();
  const firstSentence = (zh ? p3text.split(/(?<=[。！？])/)[0] : p3text.split(/(?<=[.!?])\s/)[0]) ?? p3text;
  if (await selectText(page, pIdx + 1, firstSentence)) {
    await page.waitForTimeout(1200);
    // 🚨 看的是**贴着选区的那条工具条**（.mk-seltools），不是整屏的按钮。
    const vis = await page.locator(".mk-seltools button:visible").allInnerTexts();
    const kept = await page.evaluate(() => (window.getSelection()?.toString() ?? "").trim().length > 0);
    note("划一句 → 选区还在、贴着它能点句子解析", kept && vis.some((s) => /句子解析/.test(s)) ? "ok" : zh ? "look" : "bad", `选区还在=${kept}；工具条：${vis.join(" / ")}`);
    await snap(page, "select-sentence");
    const go = page.locator(".mk-seltools").getByRole("button", { name: "句子解析" });
    if (await go.count()) {
      await go.click();
      await page.locator(".mk-grammar").first().waitFor({ timeout: 90_000 }).catch(() => {});
      const g = page.locator(".mk-grammar").first();
      if (await g.count()) {
        const txt = await g.innerText();
        note("句子解析讲的是划的那一句、有句意", /句意|意思/.test(txt) ? "ok" : "look", txt.slice(0, 160).replace(/\n/g, " / "));
        await g.scrollIntoViewIfNeeded();
        await snap(page, "grammar-card");
      } else note("句子解析卡出来了", "bad", "等了 90 秒没有 .mk-grammar");
    }
  }
  const oneWord = zh ? p3text.slice(0, 2) : (p3text.match(/[A-Za-z]{6,}/) ?? ["question"])[0];
  if (await selectText(page, pIdx + 1, oneWord)) {
    await page.waitForTimeout(1200);
    const vis = await page.locator(".mk-seltools button:visible").allInnerTexts();
    const kept = await page.evaluate(() => (window.getSelection()?.toString() ?? "").trim().length > 0);
    // 中文文章上没有查词（它是英文工具），工具条本来就不出现。
    note("划一个词 → 选区还在、贴着它能查词义", kept && vis.some((s) => /查词/.test(s)) ? "ok" : zh ? "look" : "bad", `划了「${oneWord}」；选区还在=${kept}；工具条：${vis.join(" / ")}`);
    note("划一个词 → 工具条上只有查词、没有句子解析", vis.some((s) => /句子解析/.test(s)) ? "bad" : "ok", vis.join(" / "));
    await snap(page, "select-word");
    const look = page.locator(".mk-seltools").getByRole("button", { name: "查词" });
    if (await look.count()) {
      await look.click();
      await page.getByRole("button", { name: "收起这段讲解" }).first().waitFor({ timeout: 60_000 }).catch(() => {});
      const card = await page.getByRole("button", { name: "收起这段讲解" }).first().locator("xpath=../..").innerText().catch(() => "");
      note("查词讲的是划的那个词", card.toLowerCase().includes(oneWord.toLowerCase()) ? "ok" : "bad", `划的「${oneWord}」；卡片：${card.slice(0, 120).replace(/\n/g, " / ")}`);
      await snap(page, "word-lookup");
    }
  }
  await page.keyboard.press("Escape");
  await page.mouse.click(5, 500);
  await page.waitForTimeout(800);

  // 段落后面那一行淡字：开过哪些工具
  await para.scrollIntoViewIfNeeded();
  const around = await para.locator("xpath=..").innerText().catch(() => "");
  note("开过工具的段落后面有一行记录", /翻译|语法|查词|想一想|仿写|单词|写作/.test(around.replace(await para.innerText(), "")) ? "ok" : "look", around.slice(-120).replace(/\n/g, " / "));
  await snap(page, "paragraph-trail");

  // 对话框回车发送
  const chat = page.locator("textarea:visible").last();
  const nStudent = await page.locator('[data-role="student"]').count();
  await chat.fill(zh ? "第二段的主要意思是什么？" : "What does 'surface parking' mean in paragraph 2?");
  await chat.press("Enter");
  await page.waitForTimeout(1500);
  note("对话框回车发送", (await page.locator('[data-role="student"]').count()) > nStudent ? "ok" : "bad");
  await settle(page);
  await page.waitForTimeout(2000);
  await settle(page);

  // ─────────────────────── 6. 完成 → 报告 ───────────────────────
  const finishBar = page.getByText("读法已全部完成");
  if (await finishBar.count()) {
    const copy = await finishBar.locator("xpath=..").innerText();
    note("「读法已全部完成」那段文字不再提透镜、不再说不可修改", /透镜|不可再修改/.test(copy) ? "bad" : "ok", copy.replace(/\n/g, " "));
  }
  await page.getByRole("button", { name: "完成这篇" }).click();
  const dialog = page.getByRole("dialog", { name: "完成这篇" });
  await dialog.waitFor({ timeout: 10_000 });
  const dialogText = await dialog.innerText();
  note("完成确认框文案", /透镜|不可再修改|不能再改/.test(dialogText) ? "bad" : "look", dialogText.replace(/\n/g, " ").slice(0, 200));
  await dialog.getByRole("button", { name: "完成，看报告" }).click();
  await page.getByRole("region", { name: "学习数据概览" }).waitFor({ timeout: 150_000 });
  await page.waitForTimeout(2000);
  const report1 = await page.locator("body").innerText();
  note("报告有「段落工具」一节", /段落工具/.test(report1) ? "ok" : "bad");
  note("报告有「全文总结」一节", /全文总结/.test(report1) ? "ok" : "bad");
  // 产品负责人 2026-09-18：完成页顶上只要一颗返回；查看阅读记录在导出 / 分享旁边。
  note(
    "完成页顶上只有返回（没有页签、已完成、继续阅读）",
    (await page.getByRole("tab").count()) === 0 &&
      !(await page.getByRole("button", { name: "继续阅读" }).count()) &&
      (await page.getByRole("button", { name: "返回", exact: true }).count()) > 0
      ? "ok"
      : "bad",
  );
  fs.writeFileSync(path.join(OUT, "report-v1.txt"), report1);
  await snap(page, "report-v1", true);

  // 分享：段落工具默认不公开
  let publicUrl = "";
  const shareBtn = page.getByRole("button", { name: /^分享链接/ }).first();
  if (await shareBtn.count()) {
    await shareBtn.click();
    const gen = page.getByRole("button", { name: "生成分享链接" });
    await gen.waitFor({ timeout: 10_000 }).catch(() => {});
    if (await gen.count()) await gen.click();
    const linkBox = page.getByLabel("分享链接").last();
    await page.getByText("公开我的段落工具记录").waitFor({ timeout: 15_000 }).catch(() => {});
    publicUrl = await linkBox.inputValue().catch(async () => (await linkBox.innerText().catch(() => "")).trim());
    const toolsBox = page.getByRole("checkbox", { name: /公开我的段落工具记录/ });
    const checked = await toolsBox.isChecked().catch(() => null);
    note("分享：段落工具记录单独勾选、默认不勾", checked === false ? "ok" : "bad", `checked=${checked} url=${publicUrl}`);
    await snap(page, "share-panel");
    if (publicUrl.startsWith("http")) {
      const anon = await browser.newContext({ viewport: { width: 1280, height: 900 } });
      const pub = await anon.newPage();
      await pub.goto(publicUrl);
      await pub.getByRole("region", { name: "学习数据概览" }).waitFor({ timeout: 60_000 }).catch(() => {});
      const pubText = await pub.locator("body").innerText();
      note("分享页：没勾时看不到段落工具", /段落工具/.test(pubText) ? "bad" : "ok");
      if (checked === false) {
        await toolsBox.check().catch(() => {});
        await page.waitForTimeout(2500);
        await pub.reload();
        await pub.getByRole("region", { name: "学习数据概览" }).waitFor({ timeout: 60_000 }).catch(() => {});
        note("分享页：勾上之后能看到段落工具", /段落工具/.test(await pub.locator("body").innerText()) ? "ok" : "bad");
      }
      await anon.close();
    }
    await page.keyboard.press("Escape");
    await page.waitForTimeout(500);
  } else note("报告上有分享按钮", "bad");

  // ─────────────────────── 7. 继续阅读 → 再完成 ───────────────────────
  await page.goto(`/readings/${id}`);
  await page.getByRole("region", { name: "学习数据概览" }).waitFor({ timeout: 60_000 });
  // 2026-09-18 起继续阅读在「查看阅读记录」那一页的末尾（完成页顶上只留返回）。
  const record = page.getByRole("button", { name: "查看阅读记录" });
  note("报告右上角有「查看阅读记录」", (await record.count()) ? "ok" : "bad");
  if (await record.count()) await record.click();
  const again = page.getByRole("button", { name: "继续阅读" });
  if (!(await again.count())) {
    note("阅读记录页上有「继续阅读」", "bad");
  } else {
    const hint = await page.getByText(/继续阅读后，再次完成时报告会按新的阅读记录重新生成/).count();
    note("继续阅读下面说明报告会重新生成", hint ? "ok" : "bad");
    await again.click();
    await page.locator(".mk-reading-room__article").first().waitFor({ timeout: 60_000 });
    await page.waitForTimeout(2500);
    const hist = await page.locator('[data-role="assistant"]').count();
    note("继续阅读回到原来的阅读室、对话都在", hist > 3 ? "ok" : "bad", `${hist} 条印记的话`);
    await snap(page, "reopened");
    const box = page.locator("textarea:visible").last();
    const n0 = await page.locator('[data-role="assistant"]').count();
    await box.fill(zh ? "我还想再问一个问题：作者举苏轼的例子是想说明什么？" : "One more question: why does the author mention Seoul?");
    await box.press("Enter");
    await page.waitForTimeout(1500);
    await settle(page);
    await page.waitForTimeout(3000);
    await settle(page);
    const n1 = await page.locator('[data-role="assistant"]').count();
    const reply = n1 > n0 ? await page.locator('[data-role="assistant"]').last().innerText() : "";
    note("继续阅读后还能发消息、印记会回", reply ? "ok" : "bad", reply.slice(0, 160).replace(/\n/g, " "));
    await snap(page, "reopened-chat");
    await page.getByRole("button", { name: "完成这篇" }).click();
    const d2 = page.getByRole("dialog", { name: "完成这篇" });
    await d2.waitFor({ timeout: 10_000 });
    await d2.getByRole("button", { name: "完成，看报告" }).click();
    await page.getByRole("region", { name: "学习数据概览" }).waitFor({ timeout: 150_000 });
    await page.waitForTimeout(2000);
    const report2 = await page.locator("body").innerText();
    fs.writeFileSync(path.join(OUT, "report-v2.txt"), report2);
    note("再完成：报告标出第 2 版并提示内容可能变化", /第\s*2\s*版/.test(report2) && /重新生成/.test(report2) ? "ok" : "bad", (report2.match(/第\s*\d+\s*版[^\n]*/) ?? [""])[0]);
    await snap(page, "report-v2", true);
    if (publicUrl.startsWith("http")) {
      const anon = await browser.newContext();
      const pub = await anon.newPage();
      await pub.goto(publicUrl);
      await pub.getByRole("region", { name: "学习数据概览" }).waitFor({ timeout: 60_000 }).catch(() => {});
      const t = await pub.locator("body").innerText();
      note("同一个分享链接显示新版本", /第\s*2\s*版|作者继续阅读后/.test(t) ? "ok" : "bad");
      await anon.close();
    }
  }

  fs.writeFileSync(path.join(OUT, "findings.json"), JSON.stringify({ entry: ENTRY, id, email, findings }, null, 2));
  const bad = findings.filter((f) => f.verdict === "bad");
  console.log(`\n=== ${ENTRY}: ${findings.length} 项，${bad.length} 项 ❌ ===`);
  for (const f of bad) console.log(`  ❌ ${f.check} — ${f.detail}`);
  await ctx.close();
});
