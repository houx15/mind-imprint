import { test } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { readScreen, screenKey } from "./screen";
import { think, WRITE_STUDENTS, type WriteAction, type WriteStudent } from "./brain";

/**
 * 一个模型扮演的学生，从「想写点什么」走到一篇写完为止。中文一个、英文一个。
 *
 * 这条 walk 回答两个别的东西回答不了的问题：
 *   1. 从落地页到「完成这篇」，这条路走得通吗？
 *   2. 写作这一侧**教到她了吗**？（不是「跑通了吗」）
 *
 * 🚨 为什么非要一个模型来演学生：固定脚本的 walk（`writing-walk.spec.ts`）里
 * 每一句话都是我照着代码写的 —— 我知道下一步要什么，所以我喂什么它就收什么。
 * 那证明的是**这条路存在**，不是**一个不知道路的人走得通**。
 * 见 [[camp-simulated-students-2026-09-04]] 里那四个坑。
 *
 * 这不是一条要绿的测试，它不断言任何东西。产出是 `e2e/.writewalk/` 下的记录和
 * 截图，以及最后那两个平均分（clarity / taught）。要看的是那份记录。
 *
 * 跑法（默认打线上；本地把两个 env 指过去）：
 *   cd apps/lite-web && npx playwright test e2e/writewalk/writewalk.spec.ts
 *   E2E_BASE_URL=http://localhost:5174 E2E_API_BASE=http://localhost:8080 npx playwright test e2e/writewalk
 */

test.use({ trace: "off", video: "off" });
// 两个学生各自是各自的账号，互不相干，所以可以并行。
test.describe.configure({ mode: "parallel", retries: 0 });

const API = process.env.E2E_API_BASE ?? "https://mind-api.uni-robot.cn";
const BASE = process.env.E2E_BASE_URL ?? "https://mind-lite.uni-robot.cn";
const JOIN = process.env.E2E_JOIN_CODE ?? "G624-UXFE";
/**
 * 「第几个框」这个序号，**读和写必须用同一个选择器**。
 *
 * 🚨 2026-09-12 第十七轮才查出来：读那一侧（camp/screen.ts）用的是
 * `input:visible, textarea:visible`，而写这一侧用的是
 * `textarea:visible, input[type=text]:visible` —— 后者**把 `type=number` 排除
 * 在外**。于是在写作设定那个弹窗上（一个数字框「目标字数」+ 一个大文本框），
 * 两边的序号整个错位：
 *
 *   她看到的 [0] = 数字框、[1] = 大文本框
 *   她写到的 [0] = 大文本框、[1] = 不存在
 *
 * 她要往字数框填「300」，300 落进了正文大框；她把想写的事打进 [1]，
 * 那一下哪儿都没去。于是她连着九步在同一个弹窗上重打同一段话，
 * 最后得出结论「之前写的东西都没了」——**而她一个字都没写过**。
 *
 * 这也解释了好几轮里我当成「她自己打错了」的那条重复卡壳：
 *「字数框里有个600/800/300，不知道是不是我不小心打进去的」。
 * 不是她打错了，是这只眼睛的左手和右手数的不是同一排框。
 *
 * 错位**看起来完全合理**（序号都在、值也都在），所以它能活十几轮 ——
 * 这一类是最难查的：两处各自都对，只有放在一起才错。
 */
const FIELD_SELECTOR = "input:visible, textarea:visible";

const OUT = process.env.WRITEWALK_OUT ?? "e2e/.writewalk";
const STEPS = Number(process.env.WRITEWALK_STEPS ?? 45);

for (const student of WRITE_STUDENTS) {
  test(`写作走查 · ${student.lang === "en" ? "英文" : "中文"}`, async ({ browser }) => {
    test.setTimeout(Number(process.env.WRITEWALK_MS ?? 50 * 60_000));
    fs.mkdirSync(OUT, { recursive: true });

    const tag = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`;
    const email = `write-${student.key}-${tag}@demo.mindimprint.local`;
    const password = `write-${tag}-pass`;

    const ctx = await browser.newContext({ baseURL: BASE, viewport: { width: 1440, height: 1000 } });
    const up = await ctx.request.post(`${API}/api/v1/auth/signup`, {
      data: { email, password, display_name: "走查学生", join_code: JOIN },
    });
    if (!up.ok()) throw new Error(`signup ${up.status()} ${await up.text()}`);
    await ctx.request.post(`${API}/api/v1/auth/signin`, { data: { email, password } });

    const page = await ctx.newPage();
    await page.goto("/writings");

    type Row = {
      step: number;
      read: string;
      clarity?: number;
      taught?: number;
      snag?: string;
      board?: boolean;
      buttons?: string;
      /** 她到此为止真的写出来的字数 —— 这条走查最要紧的一个事实。 */
      written?: number;
      action?: WriteAction;
    };
    const log: Row[] = [];
    const recent: string[] = [];
    let note: string | undefined;
    let lastKey = "";
    let sameFor = 0;
    /** 老师走过去说了几次。次数本身是结果，不是噪音。 */
    let nudges = 0;

    /**
     * 等 印记 把这一轮做完。
     *
     * 🚨 要等的有两样，少等一样就会把产品记成坏的：
     *   打字  不等它打完就读屏，会把「它还在打字」记成「它这一轮什么都没说」。
     *   忙    通篇审阅走旗舰模型，几十秒起，这期间按钮是禁用的。走查会把
     *         「按不动」读成「没路可走」。`aria-busy` 是 Button 在 loading 时挂的。
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

    for (let step = 1; step <= STEPS; step++) {
      await settle();
      const screen = await readScreen(page);

      const key = screenKey(screen);
      sameFor = key === lastKey ? sameFor + 1 : 0;
      lastKey = key;
      // 🚨 她在同一屏上转不出去的时候，给一句越来越具体的话 —— 营地那条 walk
      // 里这叫「老师介入」，而且**介入次数本身就是结果**：一个二十人的营，
      // 老师要走过去多少次，就是「够不够清楚」的量化答案。
      //
      // 不给这一句的话，一个会认死理的学生能在设定弹窗上耗掉一半的步数，
      // 后面的段落、板、反馈就全都走不到了 —— 那不是产品的结论，
      // 是走查没走到那儿。
      if (sameFor >= 5) {
        nudges++;
        note =
          "你已经在这一屏上停了好几步，屏幕一直没变。别再改那几个框了，" +
          "直接按下面那几个按钮里能往下走的那一个（比如「开始」「跳过」「去写」）。";
      } else if (sameFor >= 2) {
        note = "上一步之后屏幕没有变化。";
      }

      const beat = await think({ student, screen, recent, note });
      note = undefined;

      const written = screen.proseBoxes.reduce((n, f) => n + f.value.length, 0);
      log.push({
        step,
        ...beat,
        board: Boolean(screen.board),
        // 🚨 把这一屏上的按钮也记下来。走查记录里只有她的转述时，
        //「她为什么不点那个按钮」只能靠猜 —— 而按钮在不在是个事实。
        buttons: screen.buttons.filter((b) => !b.disabled).map((b) => b.label).join(" | "),
        written,
      });

      const a = beat.action;
      console.log(
        `[${student.key}][${step}] clarity=${beat.clarity} taught=${beat.taught} ${a.kind}` +
          ` · 已写${written}字${screen.board ? " · 有板" : ""}` +
          `${beat.snag ? ` · snag: ${beat.snag}` : ""}`,
      );

      recent.push(
        `${a.kind}${a.kind === "say" ? "：" + a.text.slice(0, 40) : ""}` +
          `${a.kind === "write" ? "：写了" + a.text.length + "字" : ""}`,
      );
      if (recent.length > 6) recent.shift();

      if (a.kind === "stuck" || a.kind === "leave") {
        console.log(`      ↳ ${a.kind}: ${a.kind === "stuck" ? a.why : ""}`);
        await page.screenshot({ path: path.join(OUT, `ww-${student.key}-${step}-${a.kind}.png`), fullPage: true });
        break;
      }
      if (a.kind === "wait") {
        await page.waitForTimeout(2500);
        continue;
      }
      if (a.kind === "say") {
        // 🚨 说话要往**对话框**里说，不是「最后一个 textarea」。
        // 成稿那一页最后一个 textarea 是她的正文 —— 往那儿 fill 一句
        // 「我写好了」会**整篇覆盖掉她的稿子**，然后走查会把这记成
        //「产品把我的文章弄丢了」。screen.ts 按 placeholder 分开了这两个框。
        const idx = screen.chatBox?.i;
        if (idx === undefined) {
          note = "这一屏没有跟印记说话的地方。";
          continue;
        }
        const box = page.locator(FIELD_SELECTOR).nth(idx);
        if (!(await box.count())) {
          note = "找不到那个对话框。";
          continue;
        }
        await box.fill(a.text);
        await page.keyboard.press("Enter");
        await page.waitForTimeout(800);
        continue;
      }
      if (a.kind === "write") {
        const box = page.locator(FIELD_SELECTOR).nth(a.field);
        if (!(await box.count())) {
          note = `没有第 ${a.field} 号框。`;
          continue;
        }
        await box.fill(a.text);
        // 🚨 她把 write 用在**对话框**上的时候，替她按一下回车。
        //
        // 不这么做的话那句话永远发不出去：`write` 只填不发（正文框就该只填），
        // 于是印记一轮都不会回，规划那一步原地停住，而记录上看起来是
        //「她写了十八步一个字都没进去」—— 又一次把走查自己的毛病记成产品缺陷。
        // 她真实的意图很清楚（往那个框里说话），所以这里顺着她的意图做，
        // 而不是记一条 note 让她再猜一遍。
        if (a.field === screen.chatBox?.i) {
          await page.keyboard.press("Enter");
        } else {
          // 正文框是 onBlur 存的。
          await box.blur();
        }
        await page.waitForTimeout(900);
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

    await page.screenshot({ path: path.join(OUT, `ww-${student.key}-final.png`), fullPage: true });
    fs.writeFileSync(path.join(OUT, `writewalk-${student.key}.json`), JSON.stringify(log, null, 2));

    const scored = log.filter((b) => typeof b.clarity === "number");
    const avg = (k: "clarity" | "taught") =>
      (scored.reduce((s, b) => s + (b[k] ?? 0), 0) / (scored.length || 1)).toFixed(2);
    const wrote = Math.max(0, ...log.map((b) => b.written ?? 0));
    console.log(`\n=== ${student.key}：${scored.length} 步 ===`);
    console.log(`clarity 平均 ${avg("clarity")} · taught 平均 ${avg("taught")}`);
    // 🚨 这一行是这条走查最要紧的一个数字。一个只会回「好的」的学生也能把
    // 路走完，而一个字的作文都没有 —— 那种「走通了」毫无意义。
    console.log(`她真的写出来的字数：${wrote}`);
    console.log(`老师介入次数：${nudges}`);
    const snags = log.filter((b) => b.snag).map((b) => `  [${b.step}] ${b.snag}`);
    console.log(snags.length ? `她卡住/觉得缺东西的地方：\n${snags.join("\n")}` : "她没提出任何卡点。");
    console.log(`板出现过：${log.some((b) => b.board) ? "是" : "否"}`);

    await ctx.close();
  });
}
