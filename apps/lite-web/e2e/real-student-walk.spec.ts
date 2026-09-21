import { test, expect, type Page } from "@playwright/test";
import { mkdirSync, writeFileSync } from "node:fs";
import { freshAccount } from "./freshAccount";

/**
 * 一个真学生走一遍写作室 —— 从落地页进，用她自己的字，写完一整篇。
 *
 * ## 为什么要有这一条（产品负责人 2026-09-21）
 *
 * > "run a e2e test like a real user instead of using the writings that aims
 * >  for test and cannot test the real performance"
 *
 * `teaching-r4.spec.ts` 里那段王羲之是**照着缺口造出来的**：观点句 + 材料句，
 * 然后戛然而止，缺的正好是分析句。那种用例只能证明一件事 —— 线接上了。
 * 它证明不了陪练好不好，因为答案早就写在题面上。
 *
 * 这一条反过来：**稿子先有，判据后看**。
 * 一篇高一学生会真的交上来的《短视频有没有让我们变笨？》，
 * 五段、八百来字，毛病是学生真的会犯的那几种，而不是我挑出来的那一种：
 *
 *   1. 开头  —— 「在当今这个……时代」套话开场，绕三句才见论点。
 *   2. 分论点一 —— 两个例子摆得满满的，摆完就没了；结论句只是把观点句又说一遍。
 *   3. 分论点二 —— 和第一条**其实是同一件事**（注意力碎片化），分不开。
 *   4. 分论点三 —— 这一段**写得不错**：有细节、有真实经历、自己转出了一层意思。
 *   5. 结尾  —— 「双刃剑」「做时间的主人！」，喊口号。
 *
 * 🚨 第 4 段是这条走查真正的刀口，而**第一趟跑出来，刀口在我自己身上**。
 * 我本来在这里写了一条判据：这一段有细节、有亲身经历，不许判 revise，
 * 判了就是同事说的吹毛求疵。它红了 —— 印记判的是 revise，而**它判对了**：
 * 那一段的最后一句「问题也许不在短视频本身」，和她开头立的主张是反的，
 * 让步段没收回来。通篇那一轮独立地又说了同一件事，还一次点出三处。
 * 我把「这段文字好不好」当成了「这段在这篇里成不成立」，而议论文里
 * 说了算的是后面那件事。判据现在撤掉了，理由抄在文件末尾那一段注释里。
 *
 * 规划那一段也照真人来：第一句含含糊糊，第二句给了两条其实分不开的理由，
 * 然后**连着卡住两次**（"不知道还能写啥了"）—— 卡住是真学生每次都会做的事，
 * 而帮助升级（问 → 给选项 → 给句式）只有在真的卡住时才走得到。
 *
 * ## 这条走查断言什么、不断言什么
 *
 * **不**断言印记说了哪个词。写死「必须出现分析句」就又把答案写回题面上了，
 * 而且会诱使下一个人去改 prompt 迎合这条断言
 * （[[optimizing-a-detector-made-coaching-worse-2026-09-14]]）。
 *
 * 断言的是**不能坏的那几件事**，每一条都是真出过事的：
 *   - 每一轮都得有回音：解析失败过去是一个转不动的终端（R4 修的多 JSON 对象）。
 *   - 引文必须逐字出自她的正文：幻引。
 *   - 说了这篇有问题，就得指出至少一处**能照着改的**（kind === "issue"，
 *     不是 points 张数 —— 第一趟就栽在这个差别上，见下面那条判据）。
 *   - 她写下的字一个都不许被改：铁律①。
 *
 * 剩下的全部**抄进 transcript 给人读**。这条走查的产出是一份对话记录，
 * 不是一个绿点 —— 好不好要人看，[[test-logic-not-endless-frontend]]。
 *
 * ## 跑法（会花钱：十来次真模型调用）
 *
 *   npx playwright test -c e2e/online.config.ts real-student-walk
 *
 * 记录落在 `e2e/.writewalk/real-student-<时间戳>.md`（gitignored）。
 */

const BOX_PLACEHOLDER = "说说你想写点什么，直接开始";
const WRITING_URL = /\/writings\/[0-9a-f-]{36}$/;
const STAGE_NAV = "写作四步";

// ── 她的稿子 ─────────────────────────────────────────────────────────────
//
// 🚨 这五段是**先当成一篇作文写出来的**，不是按检查表反着造的。
// 改它的时候请保持这一点：毛病要是学生真会犯的，好的那段要真的好。

const OPENING = [
  "在当今这个科技飞速发展的时代，短视频已经成为了我们生活中不可缺少的一部分。",
  "打开手机，各种各样的短视频扑面而来，让人眼花缭乱。",
  "那么，短视频到底有没有让我们变笨呢？",
  "我认为，短视频正在悄悄地让我们变笨。",
].join("");

const BODY_1 = [
  "首先，短视频把我们的注意力切得很碎。",
  "我记得有一次我想看完一部两个小时的电影，结果中间我摸了五次手机，每次都是刷了十几分钟短视频才放下。",
  "我们班上的同学也是这样，现在看课文都要老师划了重点才看得下去，一篇长一点的文章根本读不完。",
  "所以短视频真的把我们的注意力切得很碎。",
].join("");

const BODY_2 = [
  "其次，短视频让我们越来越不喜欢看长的东西了。",
  "以前我还能看完一本小说，现在看两页就想去刷手机。",
  "很多人都说自己现在没有耐心了。",
  "短视频一个只有十几秒，看完一个马上就有下一个，我们已经习惯了这种快节奏。",
].join("");

// 🚨 这一段是**好的**。有具体到鸡蛋和盐水的细节、有她自己的经历、
// 而且最后一句她自己转出了一层意思（问题不在工具在使用方式）。
// 印记在这一段上说什么，是这条走查最值得读的地方。
const BODY_3 = [
  "但短视频也不全是坏的。",
  "上学期物理的浮力我怎么都想不明白，是在一个博主把鸡蛋放进盐水里、看着它一点点浮起来的视频里才看懂的。",
  "那三分钟里他做的事，其实是把课本上那一段抽象的文字变成了我眼睛能看见的过程。",
  "所以问题也许不在短视频本身，而在于我们是带着问题去看它，还是让它替我们决定看什么。",
].join("");

const CLOSING = [
  "总之，短视频是一把双刃剑。",
  "我们要合理利用短视频，不能让它控制我们的生活。",
  "让我们放下手机，做时间的主人！",
].join("");

/** 她在规划那一屏会说的话，按顺序。含两次真的卡住。 */
const PLAN_TURNS = [
  "我想写短视频让我们变笨了。因为我自己刷完之后感觉脑子空空的，什么都没记住。",
  "嗯……我觉得一个是注意力被切碎了，还有一个是我们现在不爱看长的东西了。",
  "我想到两个例子，我上次想看完一部两小时的电影结果中间摸了五次手机；还有我们班同学现在看课文都要划了重点才看得下去。",
  "不知道还能写啥了",
  "还是想不出来",
];

type Comment = {
  verdict?: string;
  summary?: string;
  points?: { kind?: string; text?: string; action?: string; quote?: string }[];
};

const STAMP = new Date().toISOString().replace(/[:.]/g, "-");

const lines: string[] = [];

/**
 * 🚨 记录要在**失败之后**也写得出来。
 *
 * 第一次跑的时候这段写在 test 的最后一行，而那一趟红在倒数第二行 ——
 * 于是整份对话记录一个字都没落盘，只剩 stdout 里捞。
 * 这条走查失败的那一趟恰恰是最值得读的一趟。
 */
test.afterEach(() => {
  if (!lines.length) return;
  mkdirSync("e2e/.writewalk", { recursive: true });
  const out = `e2e/.writewalk/real-student-${STAMP}.md`;
  writeFileSync(out, lines.join("\n"), "utf8");
  // eslint-disable-next-line no-console
  console.log(`\n记录写在：${out}`);
});

function log(s = ""): void {
  lines.push(s);
  // eslint-disable-next-line no-console
  console.log(s);
}

function dumpComment(where: string, c: Comment): void {
  log(`### ${where}`);
  log(`- **verdict**：\`${c.verdict ?? "(无)"}\``);
  log(`- **总评**：${c.summary ?? "(无)"}`);
  if (!c.points?.length) log("- 意见：**一条都没有**");
  for (const p of c.points ?? []) {
    log(`- \`[${p.kind ?? "?"}]\` ${p.text ?? ""}`);
    if (p.action) log(`  - 怎么改：${p.action}`);
    if (p.quote) log(`  - 划的是：「${p.quote}」`);
  }
  log();
}

/**
 * 挑衅的说法。和服务端 `writing_tone.go` 的表同源。
 *
 * 同事 2026-09-21：「尤其是请印记看一看那个部分，我觉得它一直在挑衅我。」
 * 那天的走查一趟就有四句，判断全对，写法全是对她努力的判决。
 *
 * 🚨 只扫**印记说的话**（summary / text / action），不扫她的正文 ——
 * 她自己写「根本读不完」是她的字，不是印记的语气。
 */
const HOSTILE = [
  "等于没说", "等于没写", "等于白", "白写", "白立", "白费", "白搭",
  "跟着塌", "结论塌", "全篇塌", "垮了", "崩了",
  "换成谁", "换谁写", "谁来写都", "谁都能写",
  // 🚨 「明明」不能光秃秃地收 —— 「明明白白」是正常副词，印记真写过。
  "你明明", "明明说", "明明写", "明明是", "明明已经", "可你却", "你倒是",
  "一文不值", "毫无意义", "没有任何意义", "完全站不住", "一无是处",
];

function hostileIn(s: string | undefined): string {
  for (const p of HOSTILE) if ((s ?? "").includes(p)) return p;
  return "";
}

/** 这一份意见里，印记有没有说过一句挑衅的话。 */
function expectNotHostile(c: Comment, where: string): void {
  const spots: [string, string | undefined][] = [["总评", c.summary]];
  for (const p of c.points ?? []) {
    spots.push([`[${p.kind}] text`, p.text]);
    spots.push([`[${p.kind}] action`, p.action]);
  }
  for (const [what, text] of spots) {
    const hit = hostileIn(text);
    expect(
      hit,
      `${where} 的${what}写成了对她的判决（「${hit}」）—— 同事说的「一直在挑衅我」就是这个：\n${text}`,
    ).toBe("");
  }
}

/**
 * 一条意见里的引文必须逐字出自她写下的字。
 *
 * 语料**只含她的正文**，不含印记自己说过的话 ——
 * [[prompt-twice-then-make-it-checkable-2026-09-12]]：幻引的来源就是它的上文，
 * 把上文放进语料等于把判据关掉。
 */
function expectQuotesAreHers(c: Comment, hers: string, where: string): void {
  for (const p of c.points ?? []) {
    if (!p.quote) continue;
    expect(
      hers.includes(p.quote),
      `${where}：印记划的「${p.quote}」不在她写的字里 —— 幻引`,
    ).toBeTruthy();
  }
}

test("一个真学生写完一整篇《短视频有没有让我们变笨？》", async ({ browser }) => {
  test.setTimeout(30 * 60_000);

  const stamp = STAMP;
  const ctx = await freshAccount(browser, "real-student");
  const page: Page = await ctx.newPage();

  log(`# 真学生走查 · ${stamp}`);
  log();
  log("题目：短视频有没有让我们变笨？（高一，目标 800 字）");
  log();

  // ── 从落地页进，像她真的会做的那样 ──────────────────────────────────
  await page.goto("/writings");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("写作");
  await page.getByPlaceholder(BOX_PLACEHOLDER).fill("短视频有没有让我们变笨？");
  await page.getByRole("button", { name: "开始写作", exact: true }).click();
  await expect(page).toHaveURL(WRITING_URL, { timeout: 60_000 });
  const writingId = new URL(page.url()).pathname.split("/").pop()!;
  log(`写作 id：\`${writingId}\``);
  log();

  // ── 设定：她填了字数，也写了一句自己的话 ────────────────────────────
  const dialog = page.getByRole("dialog", { name: "开始之前" });
  await expect(dialog).toBeVisible({ timeout: 60_000 });
  await dialog.getByRole("button", { name: "中文" }).click();
  await dialog.getByLabel("目标字数").fill("800");
  await dialog.getByLabel("还想说点什么").fill("老师说要有自己的观点，不要抄网上的。");
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/setup") && r.request().method() === "PUT"),
    dialog.getByRole("button", { name: "开始", exact: true }).click(),
  ]);
  await expect(dialog).toHaveCount(0, { timeout: 30_000 });

  // 印记先开口 —— 这一屏不该是静的。
  await expect(page.getByPlaceholder("说说你的想法")).toBeVisible({ timeout: 60_000 });

  // ── 规划：五轮，后两轮是真的卡住 ────────────────────────────────────
  log("## 一 · 规划这一屏");
  log();
  for (const [i, said] of PLAN_TURNS.entries()) {
    const [resp] = await Promise.all([
      page.waitForResponse(
        (r) => r.url().includes("/plan/turn") && r.request().method() === "POST",
        { timeout: 300_000 },
      ),
      (async () => {
        await page.getByPlaceholder("说说你的想法").fill(said);
        await page.getByRole("button", { name: "发送", exact: true }).click();
      })(),
    ]);
    // 🚨 先看状态码，再 .json()。
    //
    // 2026-09-21 第二趟：第 5 轮回的是 **502 `model_unavailable`**（上游模型
    // 临时不可用）。当时这里直接 .json() 然后 `turn.outline.map(...)`，于是
    // 走查炸在一句 `Cannot read properties of undefined (reading 'map')` 上，
    // 读起来像走查自己写错了 —— 而产品那边其实做对了：屏幕上是
    //「后台错误：AI 响应错误（model_unavailable）」，一条真的报错，
    // 不是一句编出来的回复（[[ai-errors-must-surface-never-fake]]）。
    //
    // 走查看不懂产品给的报错，就是 [[observation-tool-is-the-bug-2026-09-12]]
    // 那条的又一次：那只眼睛自己的毛病，会被记成产品的毛病。
    if (!resp.ok()) {
      const body = await resp.text();
      log(`**第 ${i + 1} 轮后台报错**：HTTP ${resp.status()} ${body}`);
      log();
      // 屏幕上要看得见 —— 报错必须到她眼前，不能只躺在网络面板里。
      await expect(page.getByRole("alert")).toBeVisible({ timeout: 15_000 });
      throw new Error(
        `第 ${i + 1} 轮 /plan/turn 回了 HTTP ${resp.status()}：${body}\n` +
          `（model_unavailable = 上游模型临时不可用，多半重跑就好；` +
          `产品这一侧已经把它如实显示给她了，这不是代码问题。）`,
      );
    }
    const turn = (await resp.json()) as {
      reply: string;
      outline: { text: string; kind?: string; depth: number }[];
    };

    log(`**她（第 ${i + 1} 轮）**：${said}`);
    log();
    log(`**印记**：${turn.reply}`);
    log();
    log(`图上现在有：${turn.outline.map((o) => `${o.kind ?? "?"}「${o.text}」`).join(" · ") || "(空)"}`);
    log();

    // 🚨 每一轮都得有回音。解析失败在 R4 之前的样子是一个转不动的终端 ——
    // 她那边看不出是坏了还是印记不想说话。
    expect(turn.reply.trim().length, `第 ${i + 1} 轮印记一个字都没说`).toBeGreaterThan(5);
    await expect(page.getByRole("alert")).toHaveCount(0);
  }

  // ── 去写 ────────────────────────────────────────────────────────────
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/stage") && r.request().method() === "POST"),
    page.getByRole("button", { name: /去写/ }).click(),
  ]);
  await expect(page.getByRole("heading", { name: "行文" })).toBeVisible({ timeout: 60_000 });
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/stage") && r.request().method() === "POST"),
    page.getByRole("button", { name: /去写段落/ }).click(),
  ]);
  await expect(page.getByRole("navigation", { name: STAGE_NAV })).toBeVisible({ timeout: 60_000 });
  await expect(page.getByRole("heading", { name: "段落" })).toBeVisible();

  // ── 段落：把五段放进她规划出来的那叠卡片里 ──────────────────────────
  //
  // 卡片是从她的图派生的，张数只有跑到这里才知道 ——
  // 所以不写死张数：开头放第一张、结尾放最后一张、主体按顺序填中间那几张，
  // 中间不够就把剩下的并进最后一张主体卡（真学生也会这么干）。
  const cards = page.locator("[data-write-card]");
  await expect(cards.first()).toBeVisible({ timeout: 60_000 });
  const cardCount = await cards.count();
  const titles = await cards.allTextContents();
  log("## 二 · 段落这一屏");
  log();
  log(`她规划出来的卡片（${cardCount} 张）：${titles.map((t) => t.replace(/\s+/g, " ").trim()).join(" | ")}`);
  log();

  // 引导在到达时就该在，不该藏在「卡住了？」后面。
  await expect(page.getByText("写作引导").first()).toBeVisible({ timeout: 300_000 });
  const guideText = (await page.locator("aside").first().innerText()).replace(/\n{3,}/g, "\n\n");
  log("到达时左栏的写作引导（第一张卡）：");
  log();
  log("```");
  log(guideText);
  log("```");
  log();

  const bodies = [BODY_1, BODY_2, BODY_3];
  const middles = Math.max(0, cardCount - 2);
  const plan: string[] = new Array(cardCount).fill("");
  if (cardCount === 1) {
    plan[0] = [OPENING, ...bodies, CLOSING].join("\n\n");
  } else {
    plan[0] = OPENING;
    plan[cardCount - 1] = CLOSING;
    for (let i = 0; i < middles; i += 1) {
      plan[i + 1] = i === middles - 1 ? bodies.slice(i).join("") : (bodies[i] ?? "");
    }
  }

  const paper = page.getByPlaceholder("写这一段……");
  for (let i = 0; i < cardCount; i += 1) {
    if (!plan[i]) continue;
    await cards.nth(i).click();
    await paper.fill(plan[i]!);
    await Promise.all([
      page.waitForResponse((r) => r.url().includes("/snippets") && r.request().method() === "PUT"),
      paper.blur(),
    ]);
  }
  await expect(page.getByText("保存这一段失败，请重试。")).toHaveCount(0);

  // ── 每一段都请印记看一看 ────────────────────────────────────────────
  log("## 三 · 她一段一段请印记看");
  log();
  const verdicts: { title: string; verdict?: string; text: string }[] = [];
  for (let i = 0; i < cardCount; i += 1) {
    const mine = plan[i];
    if (!mine) continue;
    await cards.nth(i).click();
    await expect(paper).toHaveValue(mine, { timeout: 30_000 });

    const [resp] = await Promise.all([
      page.waitForResponse(
        (r) => /\/snippets\/[0-9a-f-]{36}\/comment/.test(r.url()) && r.request().method() === "POST",
        { timeout: 300_000 },
      ),
      page.getByRole("button", { name: "请印记看看这一段", exact: true }).click(),
    ]);
    const comment = ((await resp.json()) as { comment: Comment }).comment ?? {};

    const title = (titles[i] ?? `第 ${i + 1} 张`).replace(/\s+/g, " ").trim();
    log(`**她写的**（${title}）：`);
    log();
    log(`> ${mine}`);
    log();
    dumpComment(`印记的意见 · ${title}`, comment);
    verdicts.push({ title, verdict: comment.verdict, text: mine });

    // ── 不能坏的那几件事 ──────────────────────────────────────────────
    expect(["pass", "polish", "revise"], `${title}：verdict 不在闭表里`).toContain(
      comment.verdict ?? "",
    );
    expect(
      (comment.summary ?? "").trim().length,
      `${title}：总评是空的`,
    ).toBeGreaterThan(5);
    // 🚨 R4 修过的那个：说了这篇有问题，就得指出至少一处**能照着改的**。
    //
    // 数的是 kind === "issue"，不是 points.length —— 2026-09-21 第一次真学生
    // 走查就栽在这个差别上：开头那一段 verdict 是 polish、总评说「进题太慢」，
    // 而 points 里只有一条 `good`（夸她最后一句把结论亮出来了）。
    // 按张数数，这一段是「给了意见」；按她能不能照着改数，这一段什么都没给。
    //
    // 服务端 collectWritingComment 本来就查这一条（writingHasIssue），但它
    // 重试一次之后**照样把结果交出去**（那是故意的：一句不好的总评下面挂着
    // 几条验过的意见，也比整轮扣下让她拿不到东西强）。所以这条判据只在这里
    // 才拦得住。
    if (comment.verdict !== "pass") {
      const issues = (comment.points ?? []).filter((p) => p.kind === "issue");
      expect(
        issues.length,
        `${title}：verdict 是 ${comment.verdict}、总评也说了有问题（「${comment.summary}」），` +
          `可 points 里一条 issue 都没有（只有 ${(comment.points ?? []).map((p) => p.kind).join("/") || "空"}）` +
          ` —— 她读到一句「你这儿不太行」，却没有任何一句话可以照着改`,
      ).toBeGreaterThan(0);
    }
    expectQuotesAreHers(comment, mine, title);
    expectNotHostile(comment, title);
    // 铁律①：看完之后她的字一个都不许变。
    await expect(paper).toHaveValue(mine);
    await expect(page.getByRole("alert")).toHaveCount(0);
  }

  // ── 成稿：整篇再看一次 ──────────────────────────────────────────────
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/stage") && r.request().method() === "POST"),
    page.getByRole("navigation", { name: STAGE_NAV }).getByRole("button", { name: "成稿" }).click(),
  ]);
  await expect(page.getByRole("heading", { name: "成稿", level: 2 })).toBeVisible();
  const draftBox = page.getByPlaceholder("请先写下你最想说的那句话，再围绕它展开。");
  await expect(draftBox).not.toHaveValue("", { timeout: 60_000 });
  const whole = await draftBox.inputValue();

  log("## 四 · 整篇成稿");
  log();
  log(`拼出来的正文（${whole.replace(/\s/g, "").length} 字）：`);
  log();
  log("```");
  log(whole);
  log("```");
  log();

  const [reviewResp] = await Promise.all([
    page.waitForResponse((r) => r.url().includes("/review") && r.request().method() === "POST", {
      timeout: 300_000,
    }),
    page.getByRole("button", { name: "请印记看看", exact: true }).click(),
  ]);
  const review = ((await reviewResp.json()) as { comment: Comment }).comment ?? {};
  dumpComment("印记对整篇的意见", review);
  expectQuotesAreHers(review, whole, "整篇");
  expectNotHostile(review, "整篇");
  // 整篇看完，也不许动她的字。
  await expect(draftBox).toHaveValue(whole);
  await expect(page.getByText("这次体检没成功，请重试。")).toHaveCount(0);

  // ── 收尾：把分级摆出来给人看 ────────────────────────────────────────
  log("## 五 · 分级一览");
  log();
  for (const v of verdicts) log(`- ${v.title}：\`${v.verdict}\``);
  log();

  // 🚨 这里**曾经**有一条判据，写的是「鸡蛋那一段不许判 revise」——
  // 理由是它有细节、有亲身经历、她自己还转出了一层意思，判 revise 就是吹毛求疵。
  //
  // 2026-09-21 第一次跑，它红了。**错的是这条判据，不是陪练。**
  //
  // 印记判 revise 的理由是：这一段最后一句「问题也许不在短视频本身」，
  // 和她开头立的「短视频正在悄悄地让我们变笨」是反的 —— 让步段没收回来，
  // 走到最后把自己的立场让掉了。通篇那一轮独立地又说了同一件事，
  // 还把三处（开头的主张句、第四段的段尾、结尾的双刃剑）一起点了出来。
  // 它没有按检查表点名缺哪一句，它读懂了这一段在整篇里干了什么。
  //
  // 我把「这段文字好不好」当成了「这段在这篇里成不成立」。
  // 一段话写得漂亮，和它站不站得住，是两件事 ——
  // 而议论文恰恰是后面那件事说了算。
  //
  // 教训和 [[detector-must-target-the-real-failure]] 是同一条：
  // 判据写的是**我以为的失败**，不是真失败。这种判据红起来的样子，
  // 和产品真坏了一模一样，而它会诱人去改 prompt 迎合它
  // （[[optimizing-a-detector-made-coaching-worse-2026-09-14]]）。
  //
  // 所以这里现在**不判**质量，只把它抄下来给人读。
  // 这条走查的产出是一份记录，不是一个绿点。
  const good = verdicts.find((v) => v.text.includes("鸡蛋放进盐水里"));
  if (good) {
    log(
      `那段有细节、有亲身经历的（${good.title}）拿到的是 \`${good.verdict}\` —— ` +
        `请人读一读上面它给的理由，判得对不对只有人看得出来。`,
    );
    log();
  }

  await ctx.close();
});
