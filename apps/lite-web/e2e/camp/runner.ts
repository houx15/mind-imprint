import fs from "node:fs";
import path from "node:path";
import { expect, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { think, type Beat } from "./brain";
import { readScreen, screenKey, type Affordances } from "./screen";
import { DAYS, type Student } from "./students";

/**
 * runner.ts —— 一个学生走完四天。
 *
 * 每一步只做三件事：把屏幕读下来 → 交给扮演学生的模型 → 照它说的按下去。
 * 中间不插手。**这条 walk 不知道下一步该是什么**——它没有一句 `waitForTool`、
 * 没有一句 `getByRole("button", { name: "上线" })`。它只有一个循环。
 *
 * 断言也不在这里。这不是一条要绿的测试，是一次走查：它产出的是一份记录，
 * 记录里每一处 `stuck`、每一个 clarity ≤ 2 的时刻，都是营开始之前该看的东西。
 */

const API = (process.env.E2E_API_BASE ?? "").replace(/\/+$/, "");
const OUT = "e2e/.camp";

export type Step = {
  day: number;
  n: number;
  beat: Beat;
  /** 这一下真的按下去了没有，没有的话为什么。 */
  outcome: string;
  shot?: string;
  ms: number;
};

/**
 * 老师介入一次的记录。
 *
 * 营里有老师。学生卡住时老师会过来说一句，所以走查里也要有这一步，否则第一天
 * 卡住之后后面三天都走不到，同一个问题会被记录四遍。
 *
 * 每一次介入都要记下来，它就是「够不够清楚、够不够支撑」的量化答案：一个二十人
 * 的营，老师要过去多少次，说的是哪一句。三级介入是老师替她点了，等于这一步产品
 * 没有把人送到。
 */
export type Nudge = { step: number; level: 1 | 2 | 3; text: string };

export type DayLog = {
  day: number;
  title: string;
  brief: string;
  steps: Step[];
  /** 老师这一天走过来了几次。 */
  nudges: Nudge[];
  /** 今天这一段的目标达到了没有。 */
  done: boolean;
  /** 没达到的话，停在哪儿。 */
  stoppedBecause: string;
  ms: number;
};

export type CampLog = { student: Student; days: DayLog[] };

/* ── 账号 ─────────────────────────────────────────────────────────────── */

/**
 * 一个**全新的**账号。营的第一天就是第一次进来，所以这里不能复用种子里的
 * Phoebe——她已经有项目了，那道门对她是开的。
 *
 * 注册走的是产品自己的注册口（班级 join code，见 `signupStudent`），不是往库里
 * 塞行：这样连"注册这一段顺不顺"也一并被走到了。
 */
export async function freshStudent(browser: Browser, s: Student): Promise<BrowserContext> {
  const ctx = await browser.newContext({
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:5174",
    viewport: { width: 1440, height: 900 },
  });
  const up = await ctx.request.post(`${API}/api/v1/auth/signup`, {
    data: {
      email: s.email,
      password: s.password,
      display_name: s.name,
      join_code: "DEMO-0001", // 种子里的 Demo Class（0002_seed.sql）
    },
  });
  // 409 = 这个邮箱已经注册过（同一个库里重跑）。那就直接登录。
  if (!up.ok() && up.status() !== 409) {
    throw new Error(`注册失败：${up.status()} ${await up.text()}`);
  }
  const inn = await ctx.request.post(`${API}/api/v1/auth/signin`, {
    data: { email: s.email, password: s.password },
  });
  if (!inn.ok()) throw new Error(`登录失败：${inn.status()} ${await inn.text()}`);
  return ctx;
}

/* ── 今天这一段到没到 ────────────────────────────────────────────────── */

/**
 * 每一天的完成条件。**只读**——用接口看状态，绝不用接口去改状态。
 *
 * 🚨 这是这份走查唯一允许「知道产品内部」的地方，而且只用来判断"今天到了没有"，
 * 不用来给学生指路。学生那一侧拿到的还是只有一屏字。
 */
async function dayDone(page: Page, day: number): Promise<boolean> {
  const get = async <T>(p: string): Promise<T | null> => {
    try {
      const r = await page.request.get(`${API}${p}`);
      return r.ok() ? ((await r.json()) as T) : null;
    } catch {
      return null;
    }
  };
  const site = await get<{ published?: boolean; missing?: string[] }>("/api/v1/pbl/site");
  const projects =
    (await get<{ id: string; status: string; kind?: string }[]>("/api/v1/pbl/projects")) ?? [];

  if (day === 1) {
    // 第一天算到"她真的做完了至少一件工具"。开了房间但一件没做完，就是没到。
    const web = projects.find((p) => p.kind === "website");
    if (!web) return false;
    const tools =
      (await get<{ tool: string; finishedAt?: string | null }[]>(
        `/api/v1/pbl/projects/${web.id}/tools`,
      )) ?? [];
    return tools.some((t) => !!t.finishedAt);
  }
  if (day === 2) return !!site?.published;
  if (day === 3) {
    // 她自己的项目（不是主页那个），而且计划已经审过 → 进行中。
    return projects.some((p) => p.kind !== "website" && p.status === "running");
  }
  if (day === 4) {
    return projects.some(
      (p) => p.kind !== "website" && (p.status === "keeping" || p.status === "archived"),
    );
  }
  return false;
}

/* ── 一天 ─────────────────────────────────────────────────────────────── */

const MAX_STEPS = Number(process.env.CAMP_MAX_STEPS ?? 45);
const MAX_MS = Number(process.env.CAMP_DAY_MS ?? 25 * 60_000);
/** 连着这么多次 stuck 就散场——一个真的学生也不会在同一处杵一整天。 */
const MAX_STUCK = 3;

export async function runDay(
  page: Page,
  s: Student,
  day: number,
  /** 每一步都往盘上写一次。一天要跑二十几分钟，不能等到结束才看得见。 */
  flush?: (d: DayLog) => void,
): Promise<DayLog> {
  const meta = DAYS.find((d) => d.day === day)!;
  const log: DayLog = {
    day,
    title: meta.title,
    brief: meta.brief,
    steps: [],
    nudges: [],
    done: false,
    stoppedBecause: "",
    ms: 0,
  };
  const t0 = Date.now();
  const recent: string[] = [];
  let stuckCount = 0;
  let lostRun = 0;
  let nudgeLevel: 0 | 1 | 2 | 3 = 0;
  let lastKey = "";
  let sameKeyRuns = 0;
  let note = "";

  const shotDir = path.join(OUT, s.key);
  fs.mkdirSync(shotDir, { recursive: true });

  for (let n = 1; n <= MAX_STEPS; n++) {
    if (Date.now() - t0 > MAX_MS) {
      log.stoppedBecause = `这一天走到 ${Math.round((Date.now() - t0) / 60000)} 分钟还没结束，先停下`;
      break;
    }
    if (await dayDone(page, day)) {
      log.done = true;
      break;
    }

    const screen = await readScreen(page);
    const key = screenKey(screen);
    sameKeyRuns = key === lastKey ? sameKeyRuns + 1 : 0;
    lastKey = key;
    if (sameKeyRuns === 2) note = "你刚才那一下之后，屏幕看起来没有任何变化。";

    const stepStart = Date.now();
    let beat: Beat;
    try {
      beat = await think({ student: s, todayBrief: meta.brief, screen, recent, note });
    } catch (e) {
      log.stoppedBecause = `扮演学生的模型答不上来：${e instanceof Error ? e.message : String(e)}`;
      break;
    }
    note = "";

    const outcome = await act(page, screen, beat);
    const shot = path.join(shotDir, `d${day}-${String(n).padStart(2, "0")}.png`);
    // 截图只在值得看的时候拍：卡住、看不懂、每五步一张。整趟每一步都拍会拍出
    // 几百张没人看的图。
    const worth = beat.action.kind === "stuck" || beat.clarity <= 2 || n % 5 === 1;
    if (worth) await page.screenshot({ path: shot, fullPage: false }).catch(() => undefined);

    log.steps.push({
      day,
      n,
      beat,
      outcome,
      shot: worth ? shot : undefined,
      ms: Date.now() - stepStart,
    });
    recent.push(`${describe(beat, screen)} → ${outcome}`);
    while (recent.length > 8) recent.shift();
    flush?.(log);

    /* ── 老师走过来 ─────────────────────────────────────────────────────
     *
     * 迷路的判据是「她自己说不清楚该干什么」（clarity ≤ 2），或者她直接说卡住。
     * 连着四步都这样，一个真的老师早就注意到了。
     */
    const lost = beat.clarity <= 2 || beat.action.kind === "stuck";
    lostRun = lost ? lostRun + 1 : 0;
    // 🚨 老师要**先于**「散场」判定。原来的顺序是先数 stuck 到三次就结束这一天，
    // 而 stuck 本身也算迷路，于是三次 stuck 先到，老师永远来不及走过来——第一天
    // 一断，后面三天就都没跑上。营里不会发生这种事：学生一连卡三次，老师早过来了。
    if (lostRun >= 3) {
      lostRun = 0;
      nudgeLevel = Math.min(3, nudgeLevel + 1) as 1 | 2 | 3;
      const t = teacher(day, nudgeLevel);
      log.nudges.push({ step: n, level: nudgeLevel, text: t });
      note = `老师走过来跟你说：「${t}」`;
      if (nudgeLevel === 3) {
        // 第三级：老师直接替她点了。这一步产品自己没做到，记在案上。
        await page.goto("/projects", { waitUntil: "domcontentloaded" }).catch(() => undefined);
        await page.waitForTimeout(2500);
        note = "老师直接帮你点到了「项目」这一栏。";
      }
      stuckCount = 0;
      continue;
    }

    if (beat.action.kind === "stuck") {
      stuckCount++;
      // 只有老师已经走到第三级（替她点了）还卡着，才算这一天走不下去。
      // 在那之前卡住只是「还没被帮到」，不是「帮不了」。
      if (stuckCount >= MAX_STUCK && nudgeLevel === 3) {
        log.stoppedBecause = `老师帮到底了还是卡住：${beat.action.why}`;
        break;
      }
    }
    if (beat.action.kind === "leave") {
      log.stoppedBecause = "她认为今天这一段做完了";
      break;
    }
  }

  if (!log.done) log.done = await dayDone(page, day);
  if (!log.done && !log.stoppedBecause) log.stoppedBecause = `走满 ${MAX_STEPS} 步还没到`;
  log.ms = Date.now() - t0;
  return log;
}

/**
 * 老师那三句。
 *
 * 🚨 一级只说**目标**，不说按哪儿——因为一个真的老师第一次走过去也不会直接
 * 报坐标，她会先问「你今天要做的是什么来着」。产品要是够清楚，一级就够了。
 * 需要二级（报坐标）说明这一步在界面上找不着；需要三级（老师替她点）说明
 * 这一步产品根本没有把人送到。
 */
function teacher(day: number, level: 1 | 2 | 3): string {
  const goal: Record<number, string> = {
    1: "今天不是看新闻，是做你自己那一页——先想清楚你想让谁看见你。",
    2: "今天要把你的主页做完、发布出去，拿到一个能发给别人的链接。",
    3: "今天开你自己的项目，把你要解决的事说清楚，做出一份计划。",
    4: "今天把项目做完，复盘一次，拿到一份能给别人看的东西。",
  };
  if (level === 1) return goal[day] ?? goal[1];
  if (level === 2) return "左边那一栏里有「项目」，你要做的东西在那里面。";
  return "我帮你点到「项目」了，从这里继续。";
}

function describe(b: Beat, s: Affordances): string {
  const a = b.action;
  if (a.kind === "say") return `说：「${a.text.slice(0, 40)}」`;
  if (a.kind === "click") return `按了「${s.buttons[a.button]?.label ?? "?"}」`;
  if (a.kind === "hover") return `把光标停在「${s.buttons[a.button]?.label ?? "?"}」上`;
  if (a.kind === "fill") return `在「${s.fields[a.field]?.placeholder ?? "?"}」里写了字`;
  if (a.kind === "stuck") return `卡住：${a.why.slice(0, 40)}`;
  return a.kind;
}

/** 把学生的决定真的按下去。按不了就把原因还给她——她下一步会换一个做法。 */
async function act(page: Page, screen: Affordances, beat: Beat): Promise<string> {
  const a = beat.action;
  try {
    if (a.kind === "wait") {
      await page.waitForTimeout(5_000);
      return "等了五秒";
    }
    if (a.kind === "stuck" || a.kind === "leave") return a.kind;

    if (a.kind === "say") {
      const box = page.getByPlaceholder("请输入");
      if (!(await box.count())) return "这一屏没有对话框，说不了话";
      // 🚨 回灌那一轮里输入框是 disabled 的（印记正在读她刚做完的东西）。
      // 不等就 fill，报出来是「超时」，看着像输入框没了。
      try {
        await expect(box.first()).toBeEnabled({ timeout: 120_000 });
      } catch {
        return "对话框一直是灰的，打不了字";
      }
      await box.first().fill(a.text);
      await page.getByRole("button", { name: "发送" }).first().click();
      await page.waitForTimeout(1500);
      return "话发出去了";
    }

    if (a.kind === "click") {
      const b = screen.buttons[a.button];
      if (!b) return `没有第 ${a.button} 个按钮`;
      if (b.disabled) return `「${b.label}」按不动`;
      const target = page.locator("button:visible").nth(a.button);
      // 🚨 `force: true` 兜底，否则会造出一个假的死路。探索地图上那五颗球是
      // **一直在飘的**（`exp-drift`，见 ExploreView 的 SLOTS），Playwright 的
      // 可操作性检查里有一条「元素要停稳」，飘着的球永远等不到那一刻，于是
      // 每一次点击都以超时告终。第一次走查里学生因此连点了十几次、最后判定
      // 「这个页面坏了」——而一个真的学生用手去点一颗慢慢飘的球是点得中的。
      // 那是我的问题，不是产品的问题，不能让它冒充成一条结论。
      try {
        await target.click({ timeout: 6_000 });
      } catch {
        await target.click({ force: true, timeout: 6_000 });
      }
      await page.waitForTimeout(1200);
      return `按下了「${b.label}」`;
    }

    if (a.kind === "hover") {
      const b = screen.buttons[a.button];
      if (!b) return `没有第 ${a.button} 个按钮`;
      await page.locator("button:visible").nth(a.button).hover({ timeout: 6_000, force: true });
      await page.waitForTimeout(1500);
      return `把光标停在「${b.label}」上`;
    }

    if (a.kind === "fill") {
      const f = screen.fields[a.field];
      if (!f) return `没有第 ${a.field} 个输入框`;
      const target = page.locator("input:visible, textarea:visible").nth(a.field);
      await target.fill(a.text, { timeout: 10_000 });
      await page.waitForTimeout(400);
      return `在「${f.placeholder}」里写了字`;
    }
    return "不认识这个动作";
  } catch (e) {
    const msg = e instanceof Error ? e.message.split("\n")[0] : String(e);
    return `没按成：${msg}`;
  }
}

/* ── 四天 ─────────────────────────────────────────────────────────────── */

export async function runCamp(browser: Browser, s: Student, days: number[]): Promise<CampLog> {
  const ctx = await freshStudent(browser, s);
  const page = await ctx.newPage();
  page.on("pageerror", (e) => console.log(`[${s.key}] PAGEERROR:`, e.message));

  const out: CampLog = { student: s, days: [] };
  for (const d of days) {
    // 每天从落地页重新进来——营里的人第二天早上打开的就是这一屏。
    await page.goto("/", { waitUntil: "domcontentloaded" }).catch(() => undefined);
    await page.waitForTimeout(2500);
    const dl = await runDay(page, s, d, (partial) => {
      const snapshot: CampLog = { ...out, days: [...out.days, partial] };
      write(s, snapshot);
    });
    out.days.push(dl);
    write(s, out);
    console.log(
      `[${s.key}] 第${d}天 ${dl.done ? "走到了" : "没走到"}（${dl.steps.length} 步，` +
        `${Math.round(dl.ms / 1000)}s）${dl.stoppedBecause}`,
    );
    if (!dl.done) break; // 今天没到，后面的天没有意义——第三天要站在主页发布之后。
  }
  // 收尾出错不该把一整趟走查判红：记录已经写在盘上了。
  await ctx.close().catch(() => undefined);
  return out;
}

function write(s: Student, log: CampLog): void {
  fs.mkdirSync(OUT, { recursive: true });
  fs.writeFileSync(path.join(OUT, `${s.key}.json`), JSON.stringify(log, null, 2));
}
