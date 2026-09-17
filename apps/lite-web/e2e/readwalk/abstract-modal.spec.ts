import { test } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

/**
 * 只有摘要的那一篇：弹窗 → 继续读全文 → 粘太短被拦 → 粘全文 → 按全文重排。
 *
 * 星图上的「只有摘要」取决于那天哪个站拦了抓取，走查碰不碰得到看运气。所以分两段：
 *   PHASE=make   新注册一个账号、粘一段摘要，打印 id / 账号
 *   （之后在库里把这一篇标成 excerpt_only —— 只动这个一次性账号的这一篇）
 *   PHASE=check  用同一个账号打开，走弹窗
 */
const API = process.env.E2E_API_BASE ?? "https://mind-api.uni-robot.cn";
const BASE = process.env.E2E_BASE_URL ?? "https://mind-lite.uni-robot.cn";
const OUT = "e2e/.readwalk/abstract-modal";
const STATE = path.join(OUT, "state.json");
const ABSTRACT =
  "Ctenophores Aren't Just Beautiful. They're Biological Wonders.\n\nComb jellies, which may be the sister group to all other animals, have an unusual nervous system, and they can fuse their bodies together when injured. Researchers are only beginning to understand how they work.\n\nSource";

test("abstract modal", async ({ browser }) => {
  test.setTimeout(10 * 60_000);
  fs.mkdirSync(OUT, { recursive: true });
  const ctx = await browser.newContext({ baseURL: BASE, viewport: { width: 1440, height: 1000 } });
  const log: string[] = [];
  const say = (s: string) => {
    log.push(s);
    console.log(s);
  };

  if (process.env.PHASE === "make") {
    const tag = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
    const email = `abstract-${tag}@demo.mindimprint.local`;
    const password = `abstract-${tag}-pass`;
    await ctx.request.post(`${API}/api/v1/auth/signup`, {
      data: { email, password, display_name: "摘要走查", join_code: "G624-UXFE" },
    });
    await ctx.request.post(`${API}/api/v1/auth/signin`, { data: { email, password } });
    const made = await ctx.request.post(`${API}/api/v1/readings`, { data: { title: "栉水母不仅美丽", lang: "en" } });
    const { id } = (await made.json()) as { id: string };
    const put = await ctx.request.put(`${API}/api/v1/readings/${id}/source`, {
      data: {
        title: "栉水母不仅美丽",
        text: ABSTRACT,
      },
    });
    say(`source ${put.status()}`);
    fs.writeFileSync(STATE, JSON.stringify({ email, password, id }));
    say(`READING ${id}`);
    await ctx.close();
    return;
  }

  const { email, password, id } = JSON.parse(fs.readFileSync(STATE, "utf8")) as Record<string, string>;
  await ctx.request.post(`${API}/api/v1/auth/signin`, { data: { email, password } });
  // 先在摘要上排一份读法，看换全文之后是不是重排。
  const planned = await ctx.request.post(`${API}/api/v1/readings/${id}/plan`);
  const before = (await planned.json()) as { tasks?: { label: string }[] };
  say(`摘要上的读法：${before.tasks?.length ?? 0} 步`);

  const page = await ctx.newPage();
  await page.goto(`/readings/${id}`);
  const dlg = page.getByRole("dialog").first();
  const shown = await dlg.waitFor({ timeout: 30_000 }).then(() => true, () => false);
  say(`弹窗出现：${shown}`);
  await page.screenshot({ path: path.join(OUT, "1-modal.png") });
  if (!shown) {
    fs.writeFileSync(path.join(OUT, "log.txt"), log.join("\n"));
    return;
  }
  const t = await dlg.innerText();
  say(`标题在：${/Ctenophore|栉水母/.test(t)}；说明只拿到摘要：${/只获取到了这篇文章的摘要/.test(t)}；问读全文：${/想继续读全文吗/.test(t)}；Source 行被去掉：${!/^\s*Source\s*$/m.test(t)}`);
  const why = dlg.getByLabel("为什么拿不到全文");
  await why.hover();
  await page.waitForTimeout(500);
  const tip = await dlg.locator('[role="tooltip"]').innerText().catch(() => "");
  say(`「?」悬停：${tip.replace(/\n/g, " ")}`);
  await page.screenshot({ path: path.join(OUT, "2-tooltip.png") });
  say(`按钮：${(await dlg.getByRole("button").allInnerTexts()).join(" / ")}`);

  await dlg.getByRole("button", { name: "继续读全文" }).click();
  await page.waitForTimeout(600);
  const t2 = await dlg.innerText();
  say(`方式一/方式二：${/方式一/.test(t2)} / ${/方式二/.test(t2)}`);
  await page.screenshot({ path: path.join(OUT, "3-paste.png") });

  const box = dlg.getByLabel("文章全文");
  await box.fill("Comb jellies are strange.");
  await dlg.getByRole("button", { name: "保存全文" }).click();
  await page.waitForTimeout(800);
  say(`太短被拦：${/请确认复制的是文章全文/.test(await dlg.innerText())}`);
  await page.screenshot({ path: path.join(OUT, "4-too-short.png") });

  await box.fill(fs.readFileSync("e2e/readwalk/fixtures/long-article.txt", "utf8"));
  await dlg.getByRole("button", { name: "保存全文" }).click();
  await dlg.waitFor({ state: "hidden", timeout: 180_000 }).catch(() => {});
  await page.waitForTimeout(2500);
  const paras = await page.locator(".mk-reading-room__article-inner p[data-block-id]").count();
  const src = (await (await ctx.request.get(`${API}/api/v1/readings/${id}/source`)).json()) as { excerptOnly?: boolean };
  const after = (await (await ctx.request.get(`${API}/api/v1/readings/${id}/plan`)).json()) as { tasks?: { label: string }[] };
  say(`换成全文：${paras} 段；excerptOnly=${src.excerptOnly ?? false}；读法 ${before.tasks?.length ?? 0} → ${after.tasks?.length ?? 0} 步`);
  say(`新读法：${(after.tasks ?? []).map((x) => x.label).join(" / ")}`);
  say(`「打开原文」还在：${(await page.getByText("打开原文").count()) > 0}`);
  await page.screenshot({ path: path.join(OUT, "5-replaced.png") });
  fs.writeFileSync(path.join(OUT, "log.txt"), log.join("\n"));
  await ctx.close();
});
