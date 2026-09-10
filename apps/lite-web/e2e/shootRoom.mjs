// Take one look at the reading room a library article opens into.
//
// The walk (e2e/library-walk.spec.ts) asserts the pictures have pixels; this
// just produces a full-page screenshot a person can look at, because "the
// assertions pass" and "the page looks right" are different claims.
//
// Usage:
//   cd apps/lite-web && E2E_JOIN_CODE=G624-UXFE node e2e/shootRoom.mjs <out-dir> [slug] [tier]
//
// It lives here rather than in deploy/ because @playwright/test resolves from
// this workspace, not from the repo root.
import { chromium } from "@playwright/test";
import path from "node:path";

const API = process.env.E2E_API_BASE ?? "https://mind-api.uni-robot.cn";
const BASE = process.env.E2E_BASE_URL ?? "https://mind-lite.uni-robot.cn";
const JOIN = process.env.E2E_JOIN_CODE ?? "G624-UXFE";
const OUT = process.argv[2] ?? ".";
const SLUG = process.argv[3] ?? "nasa-osiris-rex-lands-samples-of-asteroid";
const TIER = process.argv[4] ?? "3";

const tag = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`;
const email = `shot-${tag}@demo.mindimprint.local`;
const password = `shot-${tag}-pass`;

const browser = await chromium.launch();
const ctx = await browser.newContext({ baseURL: BASE, viewport: { width: 1280, height: 1000 } });

const up = await ctx.request.post(`${API}/api/v1/auth/signup`, {
  data: { email, password, display_name: "截图", join_code: JOIN },
});
if (!up.ok()) throw new Error(`signup ${up.status()} ${await up.text()}`);
await ctx.request.post(`${API}/api/v1/auth/signin`, { data: { email, password } });

const started = await ctx.request.post(`${API}/api/v1/library/${SLUG}/levels/${TIER}`);
if (!started.ok()) throw new Error(`start ${started.status()} ${await started.text()}`);
const { id } = await started.json();

const page = await ctx.newPage();
await page.goto(`/readings/${id}`);
await page.locator("figure.mk-reading-figure").first().waitFor({ state: "visible", timeout: 30_000 });
// Scroll to the bottom so every lazy picture has been asked for, then wait
// until they have all actually decoded.
//
// It is the ARTICLE that scrolls, not the window. Scrolling the window moves
// nothing, so any picture that starts below the fold keeps loading="lazy" and
// never decodes — which looks exactly like a broken image and is not one. The
// five-picture articles in the second batch are where this first bit.
const scrollArticle = (frac) =>
  page.evaluate((f) => {
    const el = document.querySelector(".mk-reading-room__article");
    if (el) el.scrollTo(0, el.scrollHeight * f);
    else window.scrollTo(0, document.body.scrollHeight * f);
  }, frac);
for (const frac of [0.25, 0.5, 0.75, 1]) {
  await scrollArticle(frac);
  await page.waitForTimeout(400);
}
await page.waitForFunction(
  () => [...document.querySelectorAll("figure.mk-reading-figure img")].every((i) => i.complete && i.naturalWidth > 0),
  null,
  { timeout: 30_000 },
);
// Back to the top — and CHECK, because one scrollTo is not enough. The
// pictures finish decoding after the scroll down, each one growing the column
// under the viewport, and scroll anchoring drags the pane back down again. A
// file named "-top" that is actually the middle of the article is worse than
// no screenshot: it looks like the header is missing.
for (let i = 0; i < 5; i++) {
  await scrollArticle(0);
  await page.waitForTimeout(400);
  const at = await page.evaluate(
    () => document.querySelector(".mk-reading-room__article")?.scrollTop ?? 0,
  );
  if (at === 0) break;
  if (i === 4) console.warn(`WARNING: the article pane would not return to the top (scrollTop=${at})`);
}

const top = path.join(OUT, `reading-room-${SLUG}-t${TIER}-top.png`);
await page.screenshot({ path: top, fullPage: true });
console.log(`wrote ${top}  (reading ${id})`);

// The article is its OWN scroll container, so a full-page screenshot stops at
// the lead photo. Scroll the article pane itself to catch a section heading and
// a mid-article picture.
const article = page.locator(".mk-reading-room__article");
await article.evaluate((el) => el.scrollTo(0, el.scrollHeight * 0.42));
await page.waitForTimeout(600);
const mid = path.join(OUT, `reading-room-${SLUG}-t${TIER}-mid.png`);
await page.screenshot({ path: mid });
console.log(`wrote ${mid}`);
await browser.close();
