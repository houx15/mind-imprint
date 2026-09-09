import type { Browser, BrowserContext } from "@playwright/test";

/**
 * freshAccount —— 给这一条 walk 一个**刚注册的、什么都没做过的**学生。
 *
 * ## 为什么需要它
 *
 * 整个套件共用 globalSetup 里那一个种子账号（phoebe），而有几件事是**一旦发生
 * 就回不去**的：主页一发布，`POST /pbl/projects` 那道门就永远开着；一件工具一
 * 被 accept，它的邀请卡就再也不渲染了。
 *
 * 于是「她还没有主页」这个前提，取决于**别的 spec 有没有先跑过**。Playwright
 * 按文件路径排序跑，所以这个前提实际上是由文件名的字母序在守——而它守不住：
 *
 *   - 2026-09-05：`courses-walk.spec.ts` 进来了（c 排在 h 和 j 前面），它开门，
 *     于是 `homepage-walk` 和 `journey-1` 一起红了，报的是「先做你自己的主页。
 *     找不到」——看上去像那道门坏了，其实门是好的，只是已经开了。
 *   - 同一天：`journey-2` 会 accept 掉「视觉基调」，`website-stages-walk` 于是
 *     等一张永远不会出现的邀请卡，超时 15 秒。
 *
 * `journey-1` 的文件头上写着「它必须第一个跑」——一条只能靠人记住的规矩，等于
 * 迟早会被下一个新 spec 破掉。这个 helper 把那条规矩换成一件代码保证的事。
 *
 * ## 怎么用
 *
 * 需要一个干净账号的 spec 自己开 context，不要用 `use.storageState` 那份：
 *
 *   const ctx = await freshAccount(browser, "homepage-walk");
 *   const page = await ctx.newPage();
 *
 * 🚨 名字要**每次跑都不一样**（这里用时间戳加随机数）。写死一个邮箱的话，同一个
 * 库上跑第二遍会拿到第一遍那个已经发布过主页的账号，这个 helper 就白写了。
 */

const API = (process.env.E2E_API_BASE ?? "").replace(/\/+$/, "");
const BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:5174";

// 注册必须带一个有效的班级 join code ——「不存在无组织账号」是组织不变式，
// 不是这里可以绕过的一步。
//
// 默认是种子里的 Demo Class（0002_seed.sql），本地那套一次性 Postgres 用它，
// 而本地的 globalSetup 会把那所学校翻成 lite。
//
// 🚨 线上不是这样：线上的 Demo School 是 **pro**，拿 DEMO-0001 注册出来的账号
// 打任何一条轻量版的路都回 404 —— 包括 `/api/v1/readings`，看上去像是接口没了。
// 线上要用轻量版体验班的 `G624-UXFE`（2026-09-09 从线上库里取出来的，在这之前
// 谁都没记下来过）：
//
//   E2E_API_BASE=https://mind-api.uni-robot.cn E2E_JOIN_CODE=G624-UXFE …
const JOIN_CODE = process.env.E2E_JOIN_CODE ?? "DEMO-0001";

export async function freshAccount(browser: Browser, label: string): Promise<BrowserContext> {
  const tag = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`;
  const email = `e2e-${label}-${tag}@demo.mindimprint.local`;
  const password = `e2e-${tag}-pass`;

  const ctx = await browser.newContext({ baseURL: BASE_URL });
  const up = await ctx.request.post(`${API}/api/v1/auth/signup`, {
    data: { email, password, display_name: `走查 ${label}`, join_code: JOIN_CODE },
  });
  if (!up.ok()) {
    await ctx.close();
    throw new Error(`注册失败（${label}）：${up.status()} ${await up.text()}`);
  }
  const inn = await ctx.request.post(`${API}/api/v1/auth/signin`, { data: { email, password } });
  if (!inn.ok()) {
    await ctx.close();
    throw new Error(`登录失败（${label}）：${inn.status()} ${await inn.text()}`);
  }
  return ctx;
}
