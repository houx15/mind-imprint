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
await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight));
await page.waitForFunction(
  () => [...document.querySelectorAll("figure.mk-reading-figure img")].every((i) => i.complete && i.naturalWidth > 0),
  null,
  { timeout: 30_000 },
);
await page.evaluate(() => window.scrollTo(0, 0));

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
