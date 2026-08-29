import { chromium } from "@playwright/test";

/**
 * online.globalSetup.ts — the production twin of globalSetup.ts.
 *
 * Two things differ from the local harness, and both are forced by prod:
 *
 * 1. **No `docker exec`.** The local setup flips the seeded school to
 *    `edition='lite'`. Production's lite school is already lite; touching
 *    a live database from a test harness is never acceptable.
 * 2. **Sign in through the real UI, not the API.** Prod splits the app
 *    across two hosts (mind-lite / mind-api) and the session cookie is
 *    host-scoped to the API. Driving the real AuthScreen is the only way
 *    to end up with the cookie jar a real browser session actually has —
 *    and it walks the sign-in surface as a side effect.
 */
const BASE_URL = process.env.E2E_BASE_URL ?? "https://mind-lite.uni-robot.cn";
const EMAIL = process.env.E2E_EMAIL ?? "lite-smoke@test.mindimprint.cn";
const PASSWORD = process.env.E2E_PASSWORD ?? "Test123456";
const STORAGE_STATE = "e2e/.auth.online.json";

export default async function globalSetup(): Promise<void> {
  const browser = await chromium.launch();
  const ctx = await browser.newContext();
  const page = await ctx.newPage();

  await page.goto(BASE_URL, { waitUntil: "domcontentloaded" });
  await page.locator("#auth-email").fill(EMAIL);
  await page.locator("#auth-password").fill(PASSWORD);
  await page.getByRole("button", { name: "登录", exact: true }).click();

  // Success = the login control is gone AND the shell's rail is mounted.
  await page
    .getByRole("button", { name: "登录", exact: true })
    .waitFor({ state: "detached", timeout: 60_000 });
  await page
    .getByRole("navigation", { name: "主导航" })
    .waitFor({ state: "visible", timeout: 60_000 });

  await ctx.storageState({ path: STORAGE_STATE });
  await browser.close();
}
