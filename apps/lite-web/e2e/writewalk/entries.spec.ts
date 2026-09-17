import { test, type Browser, type BrowserContext, type Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { studentFor } from "./brain";
import { settleRoom, summarize, walkRoom } from "./walkLoop";

/**
 * 写作的每一个入口，从进门走到「完成 → 修改出第二版 → 放弃修改 → 老师看到 →
 * AI 批改发回 → （作业）退回修改 → 重新提交」。
 *
 * 和 readwalk/entries.spec.ts 同一个形状：每一项检查记成一条 finding
 * （ok / bad / look），不断言，一条 bad 不拦后面的检查。
 *
 * ENTRY：
 *   homework-en      老师布置英文作文（托福独立写作题），学生从作业条「开始」
 *   homework-zh      老师布置中文作文
 *   homework-upload  老师布置英文作文，学生在作业里上传写好的 .txt
 *   direct-en        写作页直接输入一道托福题
 *   direct-zh        写作页直接输入一个中文题目
 *   reading          读完一篇英文文章 →「去写一写」
 *   tree             兴趣测试种出词 → 兴趣树 →「去写」→「在写作间打开」
 *   upload-txt       「带一篇写好的进来」上传英文 .txt
 *   upload-docx      「带一篇写好的进来」上传中文 .docx
 *
 * 引导型入口由模型演的学生走（walkLoop），上传型入口按脚本走。
 *
 * 跑法（本地）：
 *   E2E_BASE_URL=http://localhost:5184 E2E_API_BASE=http://localhost:8088 \
 *   ENTRY=direct-en npx playwright test --config e2e/writewalk/playwright.config.ts entries
 * 线上：E2E_JOIN_CODE / TEACHER_EMAIL / TEACHER_PASS 指向线上的体验班和老师。
 */

test.use({ trace: "off", video: "off" });

const API = (process.env.E2E_API_BASE ?? "https://mind-api.uni-robot.cn").replace(/\/+$/, "");
const BASE = process.env.E2E_BASE_URL ?? "https://mind-lite.uni-robot.cn";
const JOIN = process.env.E2E_JOIN_CODE ?? "DEMO-0001";
const TEACHER_EMAIL = process.env.TEACHER_EMAIL ?? "wu.teacher@demo.mindimprint.local";
const TEACHER_PASS = process.env.TEACHER_PASS ?? "phoebe-dev-pass";
const ENTRY = process.env.ENTRY ?? "direct-en";
const OUT = path.join(process.env.WRITEWALK_OUT ?? "e2e/.writewalk", `entry-${ENTRY}`);
const STEPS = Number(process.env.WRITEWALK_STEPS ?? 60);
const FIX = "e2e/writewalk/fixtures";

// 托福独立写作的真题题干（公开流传的题库），和两个中文题目。
const TOEFL_GROUPS =
  "Do you agree or disagree with the following statement? It is better for students to work in groups than to study alone. Use specific reasons and examples to support your answer.";
const TOEFL_CLASSES =
  "Some people believe that university students should be required to attend classes. Others believe that going to classes should be optional for students. Which point of view do you agree with? Use specific reasons and details to explain your answer.";
const TOEFL_TECH =
  "Do you agree or disagree with the following statement? Technology has made children less creative than they were in the past. Use specific reasons and examples to support your answer.";
const ZH_PHONE = "有人说「手机让中学生的生活变得更好了」。你是否同意这个说法？请结合自己的经历，写一篇议论文。";
const ZH_EXAM = "学校应不应该取消期中考试？请写一篇议论文，说明你的看法和理由。";

const READING_TEXT = fs.existsSync(path.join(FIX, "en-reading.txt"))
  ? fs.readFileSync(path.join(FIX, "en-reading.txt"), "utf8")
  : "";

type Finding = { check: string; verdict: "ok" | "bad" | "look"; detail: string };
const findings: Finding[] = [];
function note(check: string, verdict: Finding["verdict"], detail = "") {
  findings.push({ check, verdict, detail });
  console.log(`  ${verdict === "ok" ? "✅" : verdict === "bad" ? "❌" : "👀"} ${check}${detail ? " — " + detail : ""}`);
}

let shot = 0;
async function snap(page: Page, name: string, full = true) {
  shot++;
  await page
    .screenshot({ path: path.join(OUT, `${String(shot).padStart(2, "0")}-${name}.png`), fullPage: full })
    .catch(() => {});
}

async function j<T>(ctx: BrowserContext, method: string, p: string, data?: unknown): Promise<{ status: number; body: T }> {
  const r = await ctx.request.fetch(`${API}${p}`, { method, data, failOnStatusCode: false });
  let body: unknown = null;
  try {
    body = await r.json();
  } catch {
    body = null;
  }
  return { status: r.status(), body: body as T };
}

async function signup(browser: Browser, label: string) {
  const tag = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
  const email = `ww-${label}-${tag}@demo.mindimprint.local`;
  const password = `ww-${tag}-pass`;
  const ctx = await browser.newContext({ baseURL: BASE, viewport: { width: 1440, height: 1000 } });
  const up = await ctx.request.post(`${API}/api/v1/auth/signup`, {
    data: { email, password, display_name: `写作走查${label}`, join_code: JOIN },
  });
  if (!up.ok()) throw new Error(`signup ${up.status()} ${await up.text()}`);
  await ctx.request.post(`${API}/api/v1/auth/signin`, { data: { email, password } });
  const me = await j<{ user: { id: string; classes: { id: string }[] } }>(ctx, "GET", "/api/v1/auth/me");
  return { ctx, userId: me.body.user.id, classId: me.body.user.classes[0]!.id };
}

async function teacherContext(browser: Browser) {
  const ctx = await browser.newContext({ baseURL: BASE, viewport: { width: 1440, height: 1000 } });
  const r = await ctx.request.post(`${API}/api/v1/auth/signin`, { data: { email: TEACHER_EMAIL, password: TEACHER_PASS } });
  if (!r.ok()) throw new Error(`teacher signin ${r.status()} ${await r.text()}`);
  return ctx;
}

type Writing = { id: string; lang: string; status: string; finishedAt: string | null; revisingAt?: string | null; title: string };

/** `locator.isVisible({ timeout })` ignores the timeout and answers at once. */
async function seen(l: import("@playwright/test").Locator, ms: number): Promise<boolean> {
  return l
    .waitFor({ state: "visible", timeout: ms })
    .then(() => true)
    .catch(() => false);
}

function uploaded0() {
  return ENTRY.startsWith("upload");
}

function writingIdFromUrl(url: string): string | null {
  const m = url.match(/\/writings\/([0-9a-f-]{36})/);
  return m ? m[1]! : null;
}

async function onFinishedPage(page: Page): Promise<boolean> {
  return (
    (await page.getByRole("heading", { name: "版本", exact: true }).isVisible().catch(() => false)) &&
    (await page.getByRole("button", { name: "修改", exact: true }).isVisible().catch(() => false))
  );
}

/** 完成这篇（和它可能弹出来的起名弹窗）。 */
async function finishHere(page: Page) {
  await page.getByRole("button", { name: "完成这篇" }).click();
  const keep = page.getByRole("button", { name: "用原来的" });
  const done = await Promise.race([
    keep.waitFor({ timeout: 60_000 }).then(() => "modal" as const),
    page.getByRole("heading", { name: "版本", exact: true }).waitFor({ timeout: 60_000 }).then(() => "page" as const),
  ]).catch(() => "none" as const);
  if (done === "modal") {
    await snap(page, "name-modal", false);
    const text = (await page.getByRole("dialog").innerText().catch(() => "")) ?? "";
    if (/不再改/.test(text)) note("起名弹窗说的完成后果", "bad", "还写着「完成之后这一篇就不再改了」，而完成之后可以修改出新版本");
    await keep.click();
  }
  await page.getByRole("heading", { name: "版本", exact: true }).waitFor({ timeout: 60_000 }).catch(() => {});
}

async function gotoStage(page: Page, name: "结构" | "段落" | "成稿") {
  // 结构那一屏是全屏的 PlanningView，它的「去写」进入段落。
  const tab = page.getByRole("button", { name, exact: true }).first();
  if (await tab.isVisible().catch(() => false)) {
    await tab.click();
    await settleRoom(page);
  }
}

async function composeBox(page: Page) {
  return page.getByPlaceholder("请先写下你最想说的那句话", { exact: false });
}

// ─────────────────────────────────────────────────────────────────────────

test(`写作入口：${ENTRY}`, async ({ browser }) => {
  test.setTimeout(Number(process.env.WRITEWALK_MS ?? 100 * 60_000));
  fs.mkdirSync(OUT, { recursive: true });
  const t0 = Date.now();

  const { ctx, userId, classId } = await signup(browser, ENTRY);
  const page = await ctx.newPage();
  page.on("pageerror", (e) => note("页面脚本错误", "bad", e.message.slice(0, 200)));
  page.on("response", (r) => {
    if (r.url().includes("/api/v1/") && r.status() >= 500) note("接口 5xx", "bad", `${r.status()} ${r.url().replace(API, "")}`);
  });

  const teacher = await teacherContext(browser);
  const isHomework = ENTRY.startsWith("homework");
  const lang: "zh" | "en" = /-zh|docx|tree/.test(ENTRY) ? "zh" : "en";
  let assignmentId: string | null = null;

  // ── 1. 入口 ────────────────────────────────────────────────────────────
  if (isHomework) {
    const prompt = ENTRY === "homework-zh" ? ZH_PHONE : ENTRY === "homework-upload" ? TOEFL_GROUPS : TOEFL_TECH;
    const due = new Date(Date.now() + 3 * 24 * 3600_000).toISOString();
    const made = await j<{ assignment?: { id: string }; id?: string }>(
      teacher,
      "POST",
      `/api/v1/lite/teacher/classes/${classId}/assignments`,
      {
        kind: "writing",
        title: ENTRY === "homework-zh" ? "议论文：手机与中学生" : "TOEFL Independent Writing",
        instructions: "",
        payload: { prompt, targetWords: lang === "en" ? 300 : 600, lang },
        dueAt: due,
        userIds: [userId],
      },
    );
    assignmentId = made.body?.assignment?.id ?? made.body?.id ?? null;
    note("老师布置写作作业", made.status < 300 && assignmentId ? "ok" : "bad", `${made.status}`);

    await page.goto("/writings");
    await settleRoom(page);
    const strip = page.getByRole("region", { name: "作业" });
    const startBtn = strip.getByRole("button", { name: "开始" }).first();
    const stripShown = await seen(startBtn, 15_000);
    note("写作页作业条出现「开始」", stripShown ? "ok" : "bad");
    await snap(page, "landing-homework");
    await startBtn.click();
    await page.waitForURL(/\/writings\/[0-9a-f-]{36}/, { timeout: 30_000 }).catch(() => {});
  } else if (ENTRY === "direct-en" || ENTRY === "direct-zh") {
    await page.goto("/writings");
    await settleRoom(page);
    await page.getByPlaceholder("说说你想写点什么，直接开始").fill(ENTRY === "direct-en" ? TOEFL_CLASSES : ZH_EXAM);
    await page.getByRole("button", { name: "开始写作" }).click();
    await page.waitForURL(/\/writings\/[0-9a-f-]{36}/, { timeout: 30_000 }).catch(() => {});
  } else if (ENTRY === "reading") {
    const made = await j<{ id: string }>(ctx, "POST", "/api/v1/readings", {
      title: "Why Some Student Teams Work and Others Fall Apart",
      lang: "en",
    });
    const rid = made.body.id;
    const put = await j(ctx, "PUT", `/api/v1/readings/${rid}/source`, { text: READING_TEXT });
    note("建一篇英文阅读", put.status < 300 ? "ok" : "bad", `${put.status}`);
    // 读完：走真实的完成按钮太长，这里只要「读完之后」那一屏 —— 用完成接口。
    const fin = await j(ctx, "POST", `/api/v1/readings/${rid}/finish`, { takeaway: "Group work helps only when roles are clear." });
    note("阅读完成", fin.status < 300 ? "ok" : "look", `${fin.status}`);
    await page.goto(`/readings/${rid}`);
    await settleRoom(page);
    const go = page.getByRole("button", { name: "去写一写" }).first();
    const has = await seen(go, 120_000);
    await snap(page, "reading-questions");
    note("读完出现「去写一写」", has ? "ok" : "bad");
    if (!has) throw new Error("no 去写一写");
    const q = await go.locator("xpath=ancestor::div[contains(@class,'mk-rq-bubble')]").innerText().catch(() => "");
    await go.click();
    await page.waitForURL(/\/writings/, { timeout: 20_000 });
    await settleRoom(page);
    const box = page.getByPlaceholder("说说你想写点什么，直接开始");
    const v = await box.inputValue().catch(() => "");
    note("问题带进写作框", v.trim() !== "" && q.includes(v.trim().slice(0, 20)) ? "ok" : "bad", v.slice(0, 80));
    await snap(page, "landing-prefilled");
    await page.getByRole("button", { name: "开始写作" }).click();
    await page.waitForURL(/\/writings\/[0-9a-f-]{36}/, { timeout: 30_000 }).catch(() => {});
  } else if (ENTRY === "tree") {
    let planted = 0;
    for (let i = 0; i < 3 && planted === 0; i++) {
      const q = await j<{ id: string }>(ctx, "POST", "/api/v1/interest/quiz");
      const done = await j<{ keywords: unknown[]; harvested: boolean }>(ctx, "PUT", `/api/v1/interest/quiz/${q.body.id}`, {
        navigator: "",
        anchorWork: "流浪地球",
        anchorReason: "我喜欢里面人类一起想办法把地球推走，我一直在想如果真的要搬家，城市的水和粮食怎么保证，我还自己算过地下城要多少电。",
        hook: "",
        challengeChoice: "",
        challengeAttempts: 0,
      });
      planted = done.body?.keywords?.length ?? 0;
      note("兴趣测试种词", planted > 0 ? "ok" : "look", `第 ${i + 1} 次：${done.status} harvested=${done.body?.harvested} ${planted} 个`);
    }
    await page.goto("/tree");
    await settleRoom(page);
    await page.waitForTimeout(2500);
    await snap(page, "tree");
    const node = page.locator(".tree-node").first();
    await node.click({ timeout: 20_000 }).catch(() => {});
    const open = page.getByRole("button", { name: "在写作间打开" }).first();
    const has = await seen(open, 120_000);
    await snap(page, "tree-dig");
    note("兴趣树深挖出现「在写作间打开」", has ? "ok" : "bad");
    if (!has) throw new Error("no write seed");
    await open.click();
    await page.waitForURL(/\/writings\/[0-9a-f-]{36}/, { timeout: 30_000 }).catch(() => {});
  } else if (ENTRY === "upload-txt" || ENTRY === "upload-docx") {
    await page.goto("/writings");
    await settleRoom(page);
    await page.getByRole("button", { name: "带一篇写好的进来" }).click();
    const file = ENTRY === "upload-txt" ? "en-toefl-groups.txt" : "zh-phone-essay.docx";
    await page.locator('input[type="file"]').setInputFiles(path.join(FIX, file));
    const body = page.getByPlaceholder("把你写好的文章粘贴到这里");
    await page.waitForFunction(
      () => ((document.querySelector('textarea[placeholder="把你写好的文章粘贴到这里"]') as HTMLTextAreaElement | null)?.value ?? "") !== "",
      null,
      { timeout: 30_000 },
    ).catch(() => {});
    const bodyText = await body.inputValue();
    const titleText = await page.getByPlaceholder("这一篇叫什么").inputValue();
    note("文件读进正文框", bodyText.length > 200 ? "ok" : "bad", `${bodyText.length} 字符`);
    note(
      "题目取自文件",
      /Working in Groups|手机让中学生/.test(titleText) ? "ok" : "bad",
      `题目「${titleText}」`,
    );
    note(
      "正文不重复题目那一行",
      /^(Working in Groups or Alone|手机让中学生的生活更好了吗)\s*\n/.test(bodyText.trim()) ? "bad" : "ok",
      bodyText.slice(0, 40),
    );
    await snap(page, "bring-modal", false);
    await page.getByRole("button", { name: "请印记看看" }).click();
    await page.waitForURL(/\/writings\/[0-9a-f-]{36}/, { timeout: 30_000 }).catch(() => {});
  }

  await settleRoom(page);
  if (uploaded0()) {
    const chat = await page.locator("body").innerText();
    note("带进来的这一篇，对话里不把文件名当成她说的话", /en-toefl-groups|zh-phone-essay/.test(chat) ? "bad" : "ok");
  }
  const writingId = writingIdFromUrl(page.url());
  note("进到写作房间", writingId ? "ok" : "bad", page.url());
  if (!writingId) {
    await snap(page, "no-room");
    return report();
  }

  // ── 2. 设定弹窗 ──────────────────────────────────────────────────────────
  const dialog = page.getByRole("dialog", { name: "开始之前" });
  if (await seen(dialog, 15_000)) {
    await snap(page, "setup", false);
    const enBtn = dialog.getByRole("button", { name: /English/ });
    if (await enBtn.isVisible().catch(() => false)) {
      const pressed = await enBtn.getAttribute("aria-pressed");
      // 英文题 / 英文文章进来的，语言应该已经选好 English。
      if (lang === "en") note("设定弹窗预选 English", pressed === "true" ? "ok" : "bad", `aria-pressed=${pressed}`);
      if (lang === "zh") note("设定弹窗预选中文", pressed !== "true" ? "ok" : "bad");
      if (lang === "en" && pressed !== "true") await enBtn.click();
      const words = dialog.getByLabel("目标字数");
      if (await words.isVisible().catch(() => false)) await words.fill(lang === "en" ? "300" : "600");
    } else {
      note("作业设定只读", isHomework ? "ok" : "bad");
    }
    await dialog.getByRole("button", { name: "开始", exact: true }).click();
    await settleRoom(page);
  } else {
    note("设定弹窗出现", "bad");
  }
  const w0 = await j<Writing>(ctx, "GET", `/api/v1/writings/${writingId}`);
  note("这一篇的语言", w0.body.lang === lang ? "ok" : "bad", `lang=${w0.body.lang}，应为 ${lang}`);
  await snap(page, "room-first");

  // ── 3. 写 ────────────────────────────────────────────────────────────────
  const uploaded = ENTRY.startsWith("upload") || ENTRY === "homework-upload";
  if (ENTRY === "homework-upload") {
    // 作业房间里上传写好的文件。结构那一屏是全屏的，页眉上有「上传写好的文章」。
    const toUpload = page.getByRole("button", { name: "上传写好的文章" });
    const offered = await seen(toUpload, 15_000);
    note("结构页提供「上传写好的文章」", offered ? "ok" : "bad");
    if (offered) {
      await toUpload.click();
      await settleRoom(page);
    } else {
      await gotoStage(page, "成稿");
    }
    const up = page.locator('input[type="file"]');
    const canUpload = (await up.count()) > 0;
    note("作业房间里能上传写好的文件", canUpload ? "ok" : "bad");
    await snap(page, "homework-compose");
    if (canUpload) {
      await up.first().setInputFiles(path.join(FIX, "en-toefl-groups.txt"));
      await page.waitForTimeout(3000);
      await settleRoom(page);
      const confirm = page.getByRole("button", { name: "替换", exact: true });
      if (await confirm.isVisible().catch(() => false)) await confirm.click();
      await settleRoom(page);
    }
    const box = await composeBox(page);
    const v = await box.inputValue().catch(() => "");
    note("上传的文字进了成稿", v.includes("Working in Groups") || v.includes("group work") ? "ok" : "bad", `${v.length} 字符`);
  }

  if (uploaded) {
    const review = page.getByRole("button", { name: "请印记看看" }).first();
    await review.click({ timeout: 20_000 }).catch(() => {});
    await settleRoom(page);
    await page.waitForTimeout(1500);
    const panel = await page.locator("aside").innerText().catch(() => "");
    await snap(page, "review");
    note("通篇审阅出了意见", /意见|建议|先改|这一段|这句|第/.test(panel) && panel.length > 80 ? "ok" : "bad", panel.slice(0, 200));
    fs.writeFileSync(path.join(OUT, "review.txt"), panel);
    // 照着改一句，再完成。
    const box = await composeBox(page);
    const cur = await box.inputValue();
    await box.fill(
      cur +
        (lang === "en"
          ? "\n\nFor example, when our group of four split the river report, each of us owned one part, so nobody could hide."
          : "\n\n比如上周我们小组分工做班会展示，每个人负责一部分，谁也没法偷懒。"),
    );
    await box.blur();
    await page.waitForTimeout(2500);
    await finishHere(page);
  } else {
    const idea = ENTRY === "direct-zh" ? ZH_EXAM : ENTRY === "homework-zh" ? ZH_PHONE : ENTRY === "direct-en" ? TOEFL_CLASSES : ENTRY === "homework-en" ? TOEFL_TECH : "";
    const student = studentFor(lang, idea || (lang === "en" ? "the question from the article I just read" : "我刚才在兴趣树里看到的那个问题"), ENTRY);
    const goal =
      `在这个网站上把这篇${lang === "en" ? "英文" : "中文"}作文写完，并按「完成这篇」交上去。` +
      (idea ? `题目是：${idea}` : "题目就是屏幕上这一篇的标题。") +
      `篇幅大约 ${lang === "en" ? "300 个英文单词" : "600 字"}。`;
    const result = await walkRoom(page, student, {
      steps: STEPS,
      out: OUT,
      tag: ENTRY,
      goal,
      until: () => onFinishedPage(page),
    });
    const s = summarize(result);
    fs.writeFileSync(path.join(OUT, "walk.json"), JSON.stringify(result.log, null, 2));
    fs.writeFileSync(path.join(OUT, "walk-summary.json"), JSON.stringify(s, null, 2));
    note("学生自己走到完成", s.reached ? "ok" : "bad", `${s.steps} 步 · clarity ${s.clarity} · taught ${s.taught} · 写了 ${s.wrote} 字 · 介入 ${s.nudges}`);
    for (const sn of s.snags) note("学生卡点", "look", sn);
    if (!s.reached) {
      // 走查没走到，替她把后面的检查接上。
      await snap(page, "walk-not-finished");
      await gotoStage(page, "成稿");
      const box = await composeBox(page);
      if ((await box.inputValue().catch(() => "")).trim() === "") {
        await page.getByRole("button", { name: "从段落重新拼一次" }).click().catch(() => {});
        await settleRoom(page);
      }
      await finishHere(page);
    }
  }

  // ── 4. 完成页 + 版本 ────────────────────────────────────────────────────
  await settleRoom(page);
  await snap(page, "finished-v1");
  const v1 = await j<{ versions: { number: number }[]; locked: boolean }>(ctx, "GET", `/api/v1/writings/${writingId}/versions`);
  note("完成后有第 1 版", v1.body?.versions?.length === 1 ? "ok" : "bad", `${v1.status} ${JSON.stringify(v1.body).slice(0, 120)}`);
  const pageText = await page.locator("body").innerText();
  if (/Invalid Date|undefined|NaN/.test(pageText)) note("完成页出现坏字", "bad", pageText.match(/.{0,30}(Invalid Date|undefined|NaN).{0,30}/)?.[0] ?? "");

  // 报告
  await page.getByRole("button", { name: "报告", exact: true }).click().catch(() => {});
  await page.waitForTimeout(4000);
  await settleRoom(page);
  const rep = await page.locator("body").innerText();
  await snap(page, "report");
  note("报告能打开", /加载失败|出错/.test(rep) ? "bad" : "ok", rep.match(/加载失败.{0,60}/)?.[0] ?? "");
  await page.getByRole("button", { name: "成稿", exact: true }).click().catch(() => {});

  // 修改 → 改一句 → 完成 → 第 2 版
  await page.getByRole("button", { name: "修改", exact: true }).click();
  await settleRoom(page);
  await page.waitForTimeout(1500);
  await snap(page, "revising");
  const strip = await page.getByRole("button", { name: "放弃修改" }).isVisible().catch(() => false);
  note("修改时出现「放弃修改」", strip ? "ok" : "bad");
  await gotoStage(page, "成稿");
  const box2 = await composeBox(page);
  const before = await box2.inputValue().catch(() => "");
  note("修改时成稿里是已提交的正文", before.trim().length > 50 ? "ok" : "bad", `${before.length} 字符`);
  const addV2 = lang === "en" ? "\n\nIn short, clear roles turn a group into a team." : "\n\n总之，分工清楚，小组才真正成为团队。";
  await box2.fill(before + addV2);
  await box2.blur();
  await page.waitForTimeout(2500);
  await finishHere(page);
  await settleRoom(page);
  const v2 = await j<{ versions: { number: number }[] }>(ctx, "GET", `/api/v1/writings/${writingId}/versions`);
  note("再完成得到第 2 版", v2.body?.versions?.length === 2 ? "ok" : "bad", JSON.stringify(v2.body?.versions?.map((v) => v.number)));
  await snap(page, "finished-v2");
  // 看第 1 版 + 对比
  const v1btn = page.getByRole("button", { name: /第\s*1\s*版|v1|版本 1/ }).first();
  if (await v1btn.isVisible().catch(() => false)) {
    await v1btn.click();
    await page.waitForTimeout(1500);
    await snap(page, "view-v1");
    const cmp = page.getByText("与当前版本对比").first();
    if (await cmp.isVisible().catch(() => false)) {
      await cmp.click();
      await page.waitForTimeout(1000);
      const ins = await page.locator("ins").count();
      note("对比标出新增", ins > 0 ? "ok" : "bad", `${ins} 处`);
      await snap(page, "compare");
    } else note("第 1 版能与当前对比", "bad");
  } else note("版本列表能点开第 1 版", "bad");

  // 修改 → 放弃修改
  await page.getByRole("button", { name: "修改", exact: true }).click();
  await settleRoom(page);
  await gotoStage(page, "成稿");
  const box3 = await composeBox(page);
  const b3 = await box3.inputValue().catch(() => "");
  await box3.fill(b3 + "\n\nTHROWAWAY LINE");
  await box3.blur();
  await page.waitForTimeout(2500);
  await page.getByRole("button", { name: "放弃修改" }).click();
  await page.getByRole("button", { name: "确认放弃" }).click();
  await page.getByRole("heading", { name: "版本", exact: true }).waitFor({ timeout: 30_000 }).catch(() => {});
  const w3 = await j<Writing>(ctx, "GET", `/api/v1/writings/${writingId}`);
  const d3 = await j<{ body: string }>(ctx, "GET", `/api/v1/writings/${writingId}/draft`);
  note(
    "放弃修改回到第 2 版",
    !w3.body.revisingAt && !(d3.body?.body ?? "").includes("THROWAWAY") ? "ok" : "bad",
    `revisingAt=${w3.body.revisingAt} draft含改动=${(d3.body?.body ?? "").includes("THROWAWAY")}`,
  );
  await snap(page, "after-discard");

  // ── 5. 老师 ──────────────────────────────────────────────────────────────
  const item = await j<{ writing?: { versions?: unknown[]; body?: string } } & Record<string, unknown>>(
    teacher,
    "GET",
    `/api/v1/lite/teacher/classes/${classId}/students/${userId}/items/${writingId}`,
  );
  const itemText = JSON.stringify(item.body ?? {});
  note("老师能打开这一篇", item.status === 200 ? "ok" : "bad", `${item.status}`);
  note("老师看到的是第 2 版", itemText.includes(addV2.trim().slice(0, 12)) ? "ok" : "bad");
  note("老师看不到放弃掉的改动", itemText.includes("THROWAWAY") ? "bad" : "ok");

  const tpage = await teacher.newPage();
  await tpage.goto(`/classes/${classId}/students/${userId}/items/${writingId}`);
  await settleRoom(tpage);
  await tpage.waitForTimeout(2000);
  await snap(tpage, "teacher-item");
  const unitBad = lang === "en" && /\d+\s*字\s*\n?\s*写了/.test(await tpage.locator("body").innerText());
  note("老师报告英文按词计", unitBad ? "bad" : "ok");
  await tpage.getByRole("button", { name: "写作原文" }).first().click().catch(() => {});
  await tpage.getByRole("tab", { name: "写作原文" }).first().click().catch(() => {});
  await tpage.waitForTimeout(1500);
  await snap(tpage, "teacher-item-body");
  const tText = await tpage.locator("body").innerText();
  note("老师页面显示正文", tText.includes(addV2.trim().slice(0, 12)) ? "ok" : "bad");

  if (isHomework && assignmentId) {
    const det = await j<{ recipients: { userId: string; status: string; versionCount: number }[] }>(
      teacher,
      "GET",
      `/api/v1/lite/teacher/assignments/${assignmentId}`,
    );
    const me = det.body?.recipients?.find((r) => r.userId === userId);
    note("作业状态为已完成", me?.status === "done" ? "ok" : "bad", `${me?.status} · ${me?.versionCount} 版`);
    await tpage.goto(`/assignments/${assignmentId}?tab=grading`);
    await settleRoom(tpage);
    await snap(tpage, "teacher-assignment");
  }

  // AI 批改 → 发回
  const q = await j<{ grading?: { id: string }; id?: string }>(
    teacher,
    "POST",
    `/api/v1/lite/teacher/classes/${classId}/students/${userId}/items/${writingId}/gradings`,
    {},
  );
  const gid = q.body?.grading?.id ?? q.body?.id;
  note("老师发起 AI 批改", q.status < 300 && gid ? "ok" : "bad", `${q.status} ${JSON.stringify(q.body).slice(0, 160)}`);
  let gstatus = "";
  for (let i = 0; gid && i < 60; i++) {
    const g = await j<{ grading?: { status: string; error?: string }; status?: string }>(teacher, "GET", `/api/v1/lite/teacher/gradings/${gid}`);
    gstatus = g.body?.grading?.status ?? g.body?.status ?? "";
    if (gstatus === "draft" || gstatus === "failed") {
      if (gstatus === "failed") note("AI 批改失败", "bad", JSON.stringify(g.body).slice(0, 300));
      break;
    }
    await new Promise((r) => setTimeout(r, 5000));
  }
  note("AI 批改出草稿", gstatus === "draft" ? "ok" : "bad", gstatus);
  if (gid && gstatus === "draft") {
    const sent = await j(teacher, "POST", `/api/v1/lite/teacher/gradings/${gid}/send`);
    note("批改发给学生", sent.status < 300 ? "ok" : "bad", `${sent.status}`);
    await tpage.goto(`/gradings/${gid}`);
    await settleRoom(tpage);
    await snap(tpage, "teacher-grading");
    const sg = await j<{ gradings: unknown[] }>(ctx, "GET", `/api/v1/writings/${writingId}/gradings`);
    note("学生能读到批改", (sg.body?.gradings?.length ?? 0) > 0 ? "ok" : "bad");
    await page.reload();
    await settleRoom(page);
    await page.waitForTimeout(2000);
    await snap(page, "student-sees-grading");
    const sText = await page.locator("body").innerText();
    note("学生完成页上有老师批改", /老师批改|批改/.test(sText) ? "ok" : "bad");
  }

  // 作业：退回修改 → 重新提交
  if (isHomework && assignmentId) {
    const ret = await j(teacher, "POST", `/api/v1/lite/teacher/assignments/${assignmentId}/recipients/${userId}/return`, {
      dueAt: new Date(Date.now() + 2 * 24 * 3600_000).toISOString(),
      note: lang === "en" ? "Please add one more concrete example in the second body paragraph." : "第二个分论点再补一个具体的例子。",
    });
    note("老师退回修改", ret.status < 300 ? "ok" : "bad", `${ret.status}`);
    await page.goto("/writings");
    await settleRoom(page);
    await snap(page, "landing-returned");
    const lt = await page.locator("body").innerText();
    note("写作页显示已退回", /已退回/.test(lt) ? "ok" : "bad");
    await page.goto(`/writings/${writingId}`);
    await settleRoom(page);
    const ft = await page.locator("body").innerText();
    note("完成页显示退回说明", /退回说明/.test(ft) ? "ok" : "bad");
    await page.getByRole("button", { name: "修改", exact: true }).click();
    await settleRoom(page);
    await snap(page, "returned-room");
    const rt = await page.locator("body").innerText();
    note("修改房间里看得到退回说明", /退回/.test(rt) ? "ok" : "bad");
    await gotoStage(page, "成稿");
    const box4 = await composeBox(page);
    const b4 = await box4.inputValue();
    await box4.fill(
      b4 + (lang === "en" ? "\n\nAnother example: in chemistry lab, pairs caught each other's measuring mistakes." : "\n\n再比如，化学实验课上两人一组，互相检查读数，错误少了一半。"),
    );
    await box4.blur();
    await page.waitForTimeout(2500);
    await finishHere(page);
    const det2 = await j<{ recipients: { userId: string; status: string; versionCount: number }[] }>(
      teacher,
      "GET",
      `/api/v1/lite/teacher/assignments/${assignmentId}`,
    );
    const me2 = det2.body?.recipients?.find((r) => r.userId === userId);
    note("重新提交后状态", me2?.status === "resubmitted" ? "ok" : "bad", `${me2?.status} · ${me2?.versionCount} 版`);
    await snap(page, "resubmitted");
  }

  await teacher.close();
  await ctx.close();
  report();

  function report() {
    const out = { entry: ENTRY, minutes: Math.round((Date.now() - t0) / 60000), findings };
    fs.writeFileSync(path.join(OUT, "findings.json"), JSON.stringify(out, null, 2));
    const bad = findings.filter((f) => f.verdict === "bad");
    console.log(`\n=== ${ENTRY}: ${findings.length} 项，${bad.length} 项 bad ===`);
    for (const b of bad) console.log(`  ❌ ${b.check} — ${b.detail}`);
  }
});
