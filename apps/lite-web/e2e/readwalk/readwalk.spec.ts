import { test } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { readScreen, screenKey } from "./screen";
import { think, type ReadAction } from "./brain";

/**
 * 一个模型扮演的学生，从打开一篇英文文章走到读完为止。
 *
 * 这条 walk 回答的是两个别的东西回答不了的问题：
 *   1. 从打开到「完成这篇」，这条路走得通吗？
 *   2. 带读**教到她了吗**？（不是「跑通了吗」）
 *
 * 🚨 为什么非要一个模型来演学生：`shootGuidance.mjs` 每一轮回的都是同一句
 * 「好，我读完了这一段」。那不是学生，那是一段固定的字符串 —— 它永远满足不了
 * 「在文章里点出一句」这种步骤，于是走查看起来像产品在原地打转，而其实是走查
 * 自己走不动。见 [[camp-simulated-students-2026-09-04]] 里那四个坑。
 *
 * 这不是一条要绿的测试，它不断言任何东西。产出是 `e2e/.readwalk/` 下的记录和
 * 截图，以及最后那两个平均分（clarity / taught）。要看的是那份记录。
 *
 * 跑法：
 *   cd apps/lite-web && npx playwright test e2e/readwalk/readwalk.spec.ts
 */

test.use({ trace: "off", video: "off" });
test.describe.configure({ mode: "serial", retries: 0 });

const API = process.env.E2E_API_BASE ?? "https://mind-api.uni-robot.cn";
const BASE = process.env.E2E_BASE_URL ?? "https://mind-lite.uni-robot.cn";
const JOIN = process.env.E2E_JOIN_CODE ?? "G624-UXFE";
const OUT = process.env.READWALK_OUT ?? "e2e/.readwalk";
const SLUG = process.env.READWALK_SLUG ?? "aid-groups-israel-hamas-war";
const TIER = process.env.READWALK_TIER ?? "3";
const STEPS = Number(process.env.READWALK_STEPS ?? 40);

const tag = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`;
const email = `read-${tag}@demo.mindimprint.local`;
const password = `read-${tag}-pass`;

test("英文文章：一个学生从打开读到完成", async ({ browser }) => {
  test.setTimeout(Number(process.env.READWALK_MS ?? 45 * 60_000));
  fs.mkdirSync(OUT, { recursive: true });
  const ctx = await browser.newContext({ baseURL: BASE, viewport: { width: 1440, height: 1000 } });

  const up = await ctx.request.post(`${API}/api/v1/auth/signup`, {
    data: { email, password, display_name: "走查学生", join_code: JOIN },
  });
  if (!up.ok()) throw new Error(`signup ${up.status()} ${await up.text()}`);
  await ctx.request.post(`${API}/api/v1/auth/signin`, { data: { email, password } });
  const started = await ctx.request.post(`${API}/api/v1/library/${SLUG}/levels/${TIER}`);
  if (!started.ok()) throw new Error(`start ${started.status()} ${await started.text()}`);
  const { id } = await started.json();

  const page = await ctx.newPage();
  await page.goto(`/readings/${id}`);
  await page.locator(".mk-reading-room__article, .mk-report").first().waitFor({ timeout: 30_000 });

  /** 每一步她看懂了什么、有多清楚、有没有被教到、卡在哪儿。这份记录就是产出。 */
  type Row = {
    step: number;
    read: string;
    clarity?: number;
    taught?: number;
    snag?: string;
    board?: boolean;
    done?: boolean;
    action?: ReadAction;
  };
  const log: Row[] = [];
  const recent: string[] = [];
  let note: string | undefined;
  let lastKey = "";
  let sameFor = 0;

  /**
   * 等 印记 把这一轮做完。
   *
   * 🚨 要等的有两样，少等一样就会把产品记成坏的：
   *
   *   打字     不等它打完就读屏，会把「它还在打字」记成「它这一轮没给卡片」。
   *   忙       排一条读法要跑一次旗舰模型，四十秒起，这期间按钮是禁用的。
   *            走查第一次跑就死在这儿：学生读到「一个按不动的开始」，判定自己
   *            没路可走了。`aria-busy` 是 Button 在 loading 时挂上的。
   */
  async function settle() {
    await page
      .waitForFunction(
        () =>
          !document.querySelector('[aria-label="印记正在打字"]') &&
          !document.querySelector("button[aria-busy]"),
        null,
        { timeout: 180_000 },
      )
      .catch(() => {});
    await page.waitForTimeout(300);
  }

  /** 在正文某一段里真的划出一句话。模型没有手，这是替它做的那次拖动。 */
  async function pickSentence(paragraph: number, sentence: string): Promise<string | null> {
    const p = page.locator(".mk-reading-room__article-inner p[data-block-id]").nth(paragraph - 1);
    if (!(await p.count())) return `第${paragraph}段不存在`;
    await p.scrollIntoViewIfNeeded();
    const ok = await p.evaluate((el, want) => {
      const text = el.textContent ?? "";
      let at = text.indexOf(want);
      if (at < 0) {
        // 模型抄歪了一两个字符是常事。退而求其次：拿它前 24 个字符找。
        const head = want.slice(0, 24);
        at = head.length >= 8 ? text.indexOf(head) : -1;
        if (at < 0) return false;
        want = text.slice(at, Math.min(text.length, at + want.length));
      }
      // 在这个段落的文本节点里找到偏移，做一次真的选区。
      const walk = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
      let seen = 0;
      let startNode: Node | null = null;
      let startOff = 0;
      let endNode: Node | null = null;
      let endOff = 0;
      const end = at + want.length;
      for (let n = walk.nextNode(); n; n = walk.nextNode()) {
        const len = (n.textContent ?? "").length;
        if (!startNode && seen + len > at) {
          startNode = n;
          startOff = at - seen;
        }
        if (startNode && seen + len >= end) {
          endNode = n;
          endOff = end - seen;
          break;
        }
        seen += len;
      }
      if (!startNode || !endNode) return false;
      const range = document.createRange();
      range.setStart(startNode, startOff);
      range.setEnd(endNode, endOff);
      const sel = window.getSelection();
      if (!sel) return false;
      sel.removeAllRanges();
      sel.addRange(range);
      // Annotate 是在 mouseup 上读选区的，所以要发一个真的 mouseup。
      const r = range.getBoundingClientRect();
      el.dispatchEvent(
        new MouseEvent("mouseup", {
          bubbles: true,
          clientX: r.left + r.width / 2,
          clientY: r.top + r.height / 2,
        }),
      );
      return true;
    }, sentence);
    return ok ? null : `第${paragraph}段里找不到那句话（抄歪了）`;
  }

  for (let step = 0; step < STEPS; step++) {
    await settle();
    const screen = await readScreen(page);

    // 报告页 = 这一篇读完了。
    if (/我的阅读报告|阅读报告|读完了这一篇/.test(screen.text) && !screen.paragraphs.length) {
      log.push({ step, done: true, read: "看到阅读报告了" });
      console.log(`\n[${step}] ✅ 走到报告页了`);
      await page.screenshot({ path: path.join(OUT, `rw-${step}-report.png`), fullPage: true });
      break;
    }

    const key = screenKey(screen);
    sameFor = key === lastKey ? sameFor + 1 : 0;
    lastKey = key;
    note = sameFor >= 2 ? "上一步之后屏幕没有变化。" : undefined;

    const beat = await think({ screen, recent, note });
    log.push({ step, ...beat, board: Boolean(screen.board) });
    const a = beat.action;
    console.log(
      `[${step}] clarity=${beat.clarity} taught=${beat.taught} ${a.kind}` +
        `${beat.snag ? ` · snag: ${beat.snag}` : ""}\n      读到：${beat.read}`,
    );

    recent.push(`${a.kind}${a.kind === "say" ? "：" + a.text.slice(0, 40) : ""}`);
    if (recent.length > 6) recent.shift();

    if (a.kind === "stuck" || a.kind === "leave") {
      console.log(`      ↳ ${a.kind}: ${a.kind === "stuck" ? a.why : ""}`);
      await page.screenshot({ path: path.join(OUT, `rw-${step}-${a.kind}.png`), fullPage: true });
      break;
    }
    if (a.kind === "wait") {
      await page.waitForTimeout(2500);
      continue;
    }
    if (a.kind === "say") {
      const box = page.locator("textarea:visible, input[type=text]:visible").last();
      if (!(await box.count())) {
        note = "屏幕上没有能打字的地方。";
        continue;
      }
      await box.fill(a.text);
      await page.keyboard.press("Enter");
      await page.waitForTimeout(800);
      continue;
    }
    if (a.kind === "click") {
      const b = page.locator("button:visible").nth(a.button);
      if (!(await b.count())) {
        note = `没有第 ${a.button} 号按钮。`;
        continue;
      }
      await b.click({ timeout: 8000 }).catch(() => {});
      await page.waitForTimeout(800);
      continue;
    }
    if (a.kind === "pick") {
      const err = await pickSentence(a.paragraph, a.sentence ?? "");
      if (err) {
        note = err;
        console.log(`      ↳ ${err}`);
      }
      await page.waitForTimeout(600);
      continue;
    }
    if (a.kind === "place") {
      const chip = page.locator(".mk-board__loose .mk-board__chip").nth(a.chip);
      const bin = page.locator(".mk-board__bin").nth(a.bin);
      if (!(await chip.count()) || !(await bin.count())) {
        note = "板上没有那张卡片或那个格子。";
        continue;
      }
      await chip.click();
      await bin.click();
      await page.waitForTimeout(400);
      continue;
    }
  }

  await page.screenshot({ path: path.join(OUT, "rw-final.png"), fullPage: true });
  fs.writeFileSync(path.join(OUT, "readwalk.json"), JSON.stringify(log, null, 2));

  const scored = log.filter((b) => typeof b.clarity === "number");
  const avg = (k: "clarity" | "taught") =>
    (scored.reduce((s, b) => s + (b[k] ?? 0), 0) / (scored.length || 1)).toFixed(2);
  console.log(`\n=== ${scored.length} 步 ===`);
  console.log(`clarity 平均 ${avg("clarity")} · taught 平均 ${avg("taught")}`);
  const snags = log.filter((b) => b.snag).map((b) => `  [${b.step}] ${b.snag}`);
  console.log(snags.length ? `她卡住/觉得缺东西的地方：\n${snags.join("\n")}` : "她没提出任何卡点。");
  console.log(`板出现过：${log.some((b) => b.board) ? "是" : "否"}`);


});
