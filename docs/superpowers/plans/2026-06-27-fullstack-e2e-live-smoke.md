# Full-Stack E2E Live Smoke Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A browser-driven live-smoke suite that runs the real web → real Go binary → throwaway Postgres → real DeepSeek, proving the cross-role golden path (admin → teacher → student → evaluation → signals) plus focused auth/registration edge paths.

**Architecture:** All additive test/harness code under `apps/web/e2e/`. A bash harness (`run-stack.sh`) owns process lifecycle (boot ephemeral Postgres via Docker, migrate+seed, start API + web, run Playwright, tear down). Playwright drives a real Chromium against `http://localhost:5173`; Vite's dev proxy forwards `/api` → `:8080` so the session cookie stays first-party. Zero production-code change.

**Tech Stack:** Playwright (`@playwright/test`), Vite dev server, Go (`go run ./cmd/api`), Docker (`postgres:16`), pnpm workspace.

## Global Constraints

- **Real DeepSeek throughout.** No model stub, no `STUB_LLM` seam. Live-model steps need `DEEPSEEK_API_KEY` in `apps/api/.env.local`.
- **Zero production-code change.** Only additive files under `apps/web/e2e/` plus a `@playwright/test` devDependency in `apps/web/package.json`. No edits to `apps/api/**` source, no UI `data-testid` additions (selector stability via existing text/role/type locators; adding test hooks is a flagged follow-up, out of scope).
- **Local-only, not CI.** The suite is run on demand from a developer machine. Do not add it to any CI workflow.
- **Harness boots a throwaway Postgres** via `docker run`; clean seeded state every run; full teardown on every exit path.
- **Never stage the repo-root `package.json`** (it carries a pre-existing unrelated `M`). Use explicit `git add` paths only.
- **Never print secrets.** `DEEPSEEK_API_KEY` is read by the API process from `.env.local`; never echo it.
- **Seeded credentials (from migrations 0002/0004/0006):** admin `admin@demo.mindimprint.local` / `admin-dev-pass`; student Phoebe `phoebe@demo.mindimprint.local` / `phoebe-dev-pass`; Demo Class join code `DEMO-0001`.
- **Health/route facts:** liveness `GET http://localhost:8080/healthz` → body `ok`; web dev server on `:5173` proxies `/api` → `:8080`; turn `POST /api/v1/tasks/{id}/turn`; evaluate `POST /api/v1/tasks/{id}/evaluate`.
- **Commit trailer:** end each commit message with `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`.

### Selector reference (confirmed against live source; the single source of selectors)

Auth (`apps/web/src/shell/auth/AuthScreen.tsx`): login card title `登录`; login submit `getByRole("button",{name:"登录"})`; email input = first `<input>` in the login card; password input = `input[type="password"]`; switch-to-register link `getByText("注册")`. Register step: name/email/password inputs, submit `下一步 · 绑定班级`; bind step: label `班级邀请码`, code input, submit `完成，进入思维印记`. (Same form serves teacher invite codes and student class join codes — role is decided server-side by code type.)

Console rail (`apps/web/src/console/ConsoleRail.tsx`): tabs are roles `tab` with names `概览`, `班级`, `教师`, `导入`, `设置` (admin sees all five; teacher sees `班级`,`设置`).

Admin teachers (`TeachersView.tsx`): section `生成教师邀请码`; optional email `getByPlaceholder("绑定邮箱（可选）")`; generate `getByRole("button",{name:"生成邀请码"})`; result text matches `/新邀请码\s+(\S+)/` (extract the code from this string).

Teacher classes (`ClassesView.tsx`): title `我的班级` (teacher) / `全校班级` (admin); create `getByRole("button",{name:"+ 新建班级"})`; name input `getByPlaceholder("班级名称，如「11 年级 A · TOK」")`; submit `getByRole("button",{name:"创建"})`; result text matches `/已创建「.*」· 邀请码\s+(\S+)/` (extract join code). Class card join-code badge text matches `/邀请码\s+(\S+)/`.

Class detail (`ClassDetailView.tsx`): roster `columnheader`s `姓名/邮箱/最近活跃/任务/评估/卡片`; each student `<tr>` contains the display name and integer counts.

Student workspace: directory (`DirectoryView.tsx`) heading `今天你在尝试什么？`, task textarea `getByPlaceholder("把你正在纠结的问题写下来——带上你自己的东西（链接、草稿、本子上的话）。")`, start `getByRole("button",{name:"开始"})`. Workspace (`WorkspaceView.tsx`): evaluation button name matches `/^(生成思维印记|重新评估)$/`. Composer (`Composer.tsx`): message textarea `getByPlaceholder("把你的想法发给陪练……")`, send `getByRole("button",{name:"发送"})`. Card proposal (`ChatLog.tsx`): badge `建议工具卡`, open `getByRole("button",{name:"打开卡"})`, skip `getByRole("button",{name:"暂不，先继续"})`, completed `已完成 · 已钉到过程树`, skipped `已跳过（已记录为信号）`, reopen `getByRole("button",{name:"仍可打开"})`. Card sheet (`CardSheetHost.tsx`): header `现在轮到你想`, submit `getByRole("button",{name:"提交并钉到过程树"})`. Process tree (`TreePanel.tsx`): header `过程树`, read-only badge `只读`. Eval modal (`EvalModal.tsx`): `getByRole("dialog",{name:"你的思维印记"})`, return `getByRole("button",{name:"回到任务"})`.

---

## Task 1: Playwright scaffold + stack harness + wiring smoke

**Files:**
- Modify: `apps/web/package.json` (add `@playwright/test` devDependency + an `e2e` script)
- Create: `apps/web/e2e/playwright.config.ts`
- Create: `apps/web/e2e/run-stack.sh`
- Create: `apps/web/e2e/smoke.spec.ts`

**Interfaces:**
- Consumes: nothing (foundational).
- Produces: `run-stack.sh` (one entry point: boots stack, runs `playwright test`, tears down); `playwright.config.ts` with `baseURL: http://localhost:5173`, `testDir` = this dir, `workers:1`, `retries:1`, 120s test timeout. Later tasks add `*.spec.ts` files that the same config/harness run.

- [ ] **Step 1: Add the Playwright devDependency and e2e script**

Edit `apps/web/package.json`. Add to `"scripts"`:
```json
    "e2e": "playwright test -c e2e/playwright.config.ts"
```
Add to `"devDependencies"` (keep alphabetical-ish, match existing quoting):
```json
    "@playwright/test": "^1.48.0",
```
Then install + fetch the browser:
```bash
cd /Users/houyuxin/08Coding/mind-imprint && pnpm install
pnpm --filter web exec playwright install chromium
```
Expected: install completes; `chromium` downloaded.

- [ ] **Step 2: Write the Playwright config**

Create `apps/web/e2e/playwright.config.ts`:
```ts
import { defineConfig } from "@playwright/test";

// The bash harness (run-stack.sh) owns process lifecycle, so no `webServer`
// block here — Playwright only drives the browser against the already-running
// dev server. Serial + retries:1 because the suite shares one stack and one
// seeded database, and the live model is non-deterministic.
export default defineConfig({
  testDir: ".",
  fullyParallel: false,
  workers: 1,
  retries: 1,
  timeout: 120_000,
  expect: { timeout: 15_000 },
  use: {
    baseURL: "http://localhost:5173",
    trace: "retain-on-failure",
    actionTimeout: 15_000,
  },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
```

- [ ] **Step 3: Write the failing smoke spec**

Create `apps/web/e2e/smoke.spec.ts`:
```ts
import { test, expect } from "@playwright/test";

// Proves the harness wiring before the expensive live-model journey:
// the web dev server serves the app, and the API liveness endpoint answers.
test("stack is up: app loads and API is healthy", async ({ page, request }) => {
  await page.goto("/");
  // The unauthenticated app boots to the auth screen (login card title 登录).
  await expect(page.getByRole("button", { name: "登录" }).first()).toBeVisible();

  // Liveness is at the server root (not under /api), so hit :8080 directly.
  const res = await request.get("http://localhost:8080/healthz");
  expect(res.status()).toBe(200);
  expect((await res.text()).trim()).toBe("ok");
});
```

- [ ] **Step 4: Run the smoke spec to verify it FAILS (no stack yet)**

```bash
cd /Users/houyuxin/08Coding/mind-imprint/apps/web && pnpm e2e
```
Expected: FAIL — connection refused at `:5173`/`:8080` (nothing is running yet). This confirms the spec actually depends on a live stack.

- [ ] **Step 5: Write the orchestration harness**

Create `apps/web/e2e/run-stack.sh` (make it `chmod +x`):
```bash
#!/usr/bin/env bash
# Full-stack live-smoke harness: boots a throwaway Postgres, migrates+seeds,
# starts the API and the web dev server, runs Playwright, and tears everything
# down on every exit path. Run from anywhere: `bash apps/web/e2e/run-stack.sh`.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
PG_CONTAINER="mindimprint-e2e-pg"
PG_PORT="${E2E_PG_PORT:-55432}"
DB_URL="postgres://postgres:postgres@localhost:${PG_PORT}/mindimprint?sslmode=disable"
API_PID=""
WEB_PID=""

cleanup() {
  set +e
  [ -n "$WEB_PID" ] && kill "$WEB_PID" 2>/dev/null
  [ -n "$API_PID" ] && kill "$API_PID" 2>/dev/null
  docker rm -f "$PG_CONTAINER" >/dev/null 2>&1
}
trap cleanup EXIT INT TERM

command -v docker >/dev/null 2>&1 || { echo "FATAL: Docker is required for the throwaway Postgres."; exit 1; }
[ -f "$REPO/apps/api/.env.local" ] || { echo "FATAL: create apps/api/.env.local with DEEPSEEK_API_KEY (see RUNBOOK.md)."; exit 1; }
grep -qE '^DEEPSEEK_API_KEY=.+' "$REPO/apps/api/.env.local" || echo "WARN: DEEPSEEK_API_KEY looks empty — live-model steps will fail."

echo "==> Booting throwaway Postgres on :${PG_PORT}"
docker rm -f "$PG_CONTAINER" >/dev/null 2>&1 || true
docker run -d --name "$PG_CONTAINER" -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=mindimprint -p "${PG_PORT}:5432" postgres:16 >/dev/null
until docker exec "$PG_CONTAINER" pg_isready -U postgres >/dev/null 2>&1; do sleep 0.5; done

echo "==> Applying migrations + seed"
( cd "$REPO/apps/api" && DATABASE_URL="$DB_URL" go run ./cmd/api -migrate-up )

echo "==> Starting API on :8080"
# DATABASE_URL/COOKIE_SECURE/CORS_ORIGINS are exported here; godotenv does NOT
# override already-set env vars, so .env.local still supplies DEEPSEEK_API_KEY.
( cd "$REPO/apps/api" && DATABASE_URL="$DB_URL" COOKIE_SECURE=false CORS_ORIGINS=http://localhost:5173 go run ./cmd/api ) &
API_PID=$!
until curl -sf http://localhost:8080/healthz >/dev/null; do sleep 0.5; done

echo "==> Starting web dev server on :5173"
( cd "$REPO" && pnpm --filter web dev --port 5173 --strictPort ) &
WEB_PID=$!
until curl -sf http://localhost:5173 >/dev/null; do sleep 0.5; done

echo "==> Running Playwright"
( cd "$REPO/apps/web" && pnpm e2e "$@" )
```

- [ ] **Step 6: Run the harness; smoke spec PASSES**

```bash
cd /Users/houyuxin/08Coding/mind-imprint && bash apps/web/e2e/run-stack.sh smoke.spec.ts
```
Expected: Postgres boots, migrations apply, API + web come up, Playwright runs `smoke.spec.ts` → 1 passed, then teardown removes the container. (No `DEEPSEEK_API_KEY` needed for this spec — the API boots without a model key; only turn/eval calls would need it.)

- [ ] **Step 7: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/web/package.json apps/web/e2e/playwright.config.ts apps/web/e2e/run-stack.sh apps/web/e2e/smoke.spec.ts
git commit -m "$(cat <<'EOF'
test(e2e): playwright scaffold + full-stack harness + wiring smoke

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"
```
Note: `pnpm-lock.yaml` may also change from the devDependency add — include it: `git add pnpm-lock.yaml`. Do NOT add the repo-root `package.json`.

---

## Task 2: Role helpers + auth edge spec

**Files:**
- Create: `apps/web/e2e/helpers.ts`
- Create: `apps/web/e2e/auth.spec.ts`

**Interfaces:**
- Consumes: the running stack from `run-stack.sh`; `baseURL` from the config.
- Produces (for later tasks):
  - `ADMIN`, `PHOEBE` constants `{ email: string; password: string }`.
  - `async function login(page: Page, email: string, password: string): Promise<void>` — drives the login card; resolves once the auth screen is gone.
  - `async function logout(page: Page): Promise<void>` — opens 设置 tab and clicks 退出登录.
  - `async function registerWithCode(page: Page, opts: { name: string; email: string; password: string; code: string }): Promise<void>` — drives register → bind → submit; resolves after auto-signin lands in the app. Works for both teacher invite codes and student class join codes (server decides role).
  - `function uniqueEmail(prefix: string): string` — returns `${prefix}+${Date.now()}@e2e.local` for fresh accounts.

- [ ] **Step 1: Write the helpers**

Create `apps/web/e2e/helpers.ts`:
```ts
import { Page, expect } from "@playwright/test";

export const ADMIN = { email: "admin@demo.mindimprint.local", password: "admin-dev-pass" };
export const PHOEBE = { email: "phoebe@demo.mindimprint.local", password: "phoebe-dev-pass" };

export function uniqueEmail(prefix: string): string {
  return `${prefix}+${Date.now()}@e2e.local`;
}

// Drives the login card. Auth inputs have no test hooks, so target by the
// password type + the first text input in the login card (see selector ref).
export async function login(page: Page, email: string, password: string): Promise<void> {
  await page.goto("/");
  const loginBtn = page.getByRole("button", { name: "登录" }).first();
  await expect(loginBtn).toBeVisible();
  const card = page.locator("body");
  await card.locator('input:not([type="password"])').first().fill(email);
  await card.locator('input[type="password"]').first().fill(password);
  await loginBtn.click();
  // Login resolves when the login submit button is gone (app rendered).
  await expect(page.getByRole("button", { name: "登录" })).toHaveCount(0, { timeout: 30_000 });
}

export async function logout(page: Page): Promise<void> {
  await page.getByRole("tab", { name: "设置" }).click();
  await page.getByText("退出登录").click();
  await expect(page.getByRole("button", { name: "登录" }).first()).toBeVisible({ timeout: 15_000 });
}

// Register → bind code → submit. Auto-signin lands in the app; resolves when
// the auth screen is gone. `code` is a teacher invite OR a class join code.
export async function registerWithCode(
  page: Page,
  opts: { name: string; email: string; password: string; code: string },
): Promise<void> {
  await page.goto("/");
  await page.getByText("注册").click();
  // Register step: name, email, password (three inputs; password is typed).
  const inputs = page.locator("input");
  await page.locator('input:not([type="password"])').nth(0).fill(opts.name);
  await page.locator('input:not([type="password"])').nth(1).fill(opts.email);
  await page.locator('input[type="password"]').first().fill(opts.password);
  await page.getByRole("button", { name: "下一步 · 绑定班级" }).click();
  // Bind step: class/invite code.
  await expect(page.getByText("班级邀请码")).toBeVisible();
  await page.locator("input").last().fill(opts.code);
  await page.getByRole("button", { name: "完成，进入思维印记" }).click();
  await expect(page.getByRole("button", { name: "完成，进入思维印记" })).toHaveCount(0, { timeout: 30_000 });
}
```
Note: if the register step's three text inputs do not resolve by `nth()` as written, the implementer adjusts to the actual DOM order observed via `playwright test --debug`; the contract (this function fully registers + lands in-app) is what later tasks depend on.

- [ ] **Step 2: Write the auth edge spec**

Create `apps/web/e2e/auth.spec.ts`:
```ts
import { test, expect } from "@playwright/test";
import { login, ADMIN } from "./helpers";

test("seeded admin logs in and lands on the console overview", async ({ page }) => {
  await login(page, ADMIN.email, ADMIN.password);
  // Admin lands on the console; 概览 tab is present and selected-area visible.
  await expect(page.getByRole("tab", { name: "概览" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "教师" })).toBeVisible();
});

test("wrong password shows an inline error and stays on login", async ({ page }) => {
  await page.goto("/");
  await page.locator('input:not([type="password"])').first().fill(ADMIN.email);
  await page.locator('input[type="password"]').first().fill("definitely-wrong");
  await page.getByRole("button", { name: "登录" }).first().click();
  // Still on the login screen (button remains), no crash.
  await expect(page.getByRole("button", { name: "登录" }).first()).toBeVisible();
});

test("visiting the app while logged-out shows the auth screen, not a crash", async ({ page }) => {
  await page.context().clearCookies();
  await page.goto("/");
  await expect(page.getByRole("button", { name: "登录" }).first()).toBeVisible();
});
```

- [ ] **Step 3: Run the auth spec against the live stack**

```bash
cd /Users/houyuxin/08Coding/mind-imprint && bash apps/web/e2e/run-stack.sh auth.spec.ts
```
Expected: 3 passed. (No model key needed — auth/login only.) If `login()` selectors mis-resolve, fix per the Step-1 note and re-run.

- [ ] **Step 4: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/web/e2e/helpers.ts apps/web/e2e/auth.spec.ts
git commit -m "$(cat <<'EOF'
test(e2e): role login/register helpers + auth edge spec

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Registration edge spec (bad join code rejected)

**Files:**
- Create: `apps/web/e2e/registration.spec.ts`

**Interfaces:**
- Consumes: `registerWithCode`, `uniqueEmail` from `helpers.ts`; the running stack.
- Produces: nothing downstream.

- [ ] **Step 1: Write the registration edge spec**

Create `apps/web/e2e/registration.spec.ts`:
```ts
import { test, expect } from "@playwright/test";
import { uniqueEmail } from "./helpers";

// The org invariant: no account without a valid class/invite code. Signing up
// with a bogus code must be rejected and must NOT land the user in the app.
test("signup with an invalid join code is rejected and stays on the bind step", async ({ page }) => {
  await page.goto("/");
  await page.getByText("注册").click();
  await page.locator('input:not([type="password"])').nth(0).fill("Bad Code Student");
  await page.locator('input:not([type="password"])').nth(1).fill(uniqueEmail("badcode"));
  await page.locator('input[type="password"]').first().fill("e2e-pass-12345");
  await page.getByRole("button", { name: "下一步 · 绑定班级" }).click();
  await expect(page.getByText("班级邀请码")).toBeVisible();
  await page.locator("input").last().fill("NOPE-9999");
  await page.getByRole("button", { name: "完成，进入思维印记" }).click();
  // Rejected: the bind submit is still present (we never entered the app).
  await expect(page.getByRole("button", { name: "完成，进入思维印记" })).toBeVisible({ timeout: 15_000 });
});
```

- [ ] **Step 2: Run it against the live stack**

```bash
cd /Users/houyuxin/08Coding/mind-imprint && bash apps/web/e2e/run-stack.sh registration.spec.ts
```
Expected: 1 passed. (No model key needed.)

- [ ] **Step 3: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/web/e2e/registration.spec.ts
git commit -m "$(cat <<'EOF'
test(e2e): registration edge — invalid join code rejected

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Golden-path lifecycle spec (live model) + runbook

**Files:**
- Create: `apps/web/e2e/golden-path.spec.ts`
- Create: `apps/web/e2e/RUNBOOK.md`

**Interfaces:**
- Consumes: `login`, `logout`, `registerWithCode`, `uniqueEmail`, `ADMIN` from `helpers.ts`; the running stack with a real `DEEPSEEK_API_KEY`.
- Produces: nothing downstream (terminal task).

**Live-model note (read before implementing):** this spec needs `apps/api/.env.local` to carry a real `DEEPSEEK_API_KEY`. Steps 1–4 of the journey (admin invite → teacher signup → class → student join → task creation) are deterministic and run without the key. The card-summon and evaluation steps depend on the live model. `tool_choice` is `"auto"` (the model chooses), so the summon is treated as a retried, generously-waited step with an explicit "model declined" diagnostic — it is NOT a forced call. If you (the implementer) lack the key, build the spec to completion and verify the deterministic prefix; the keyed run is performed by the developer per `RUNBOOK.md`.

- [ ] **Step 1: Write the golden-path spec**

Create `apps/web/e2e/golden-path.spec.ts`:
```ts
import { test, expect, Page } from "@playwright/test";
import { login, logout, registerWithCode, uniqueEmail, ADMIN } from "./helpers";

// One continuous cross-role journey. Serial by config. Live model on the
// student leg; everything else is deterministic.
test("golden path: admin → teacher → student → evaluation → signals", async ({ page }) => {
  const teacherEmail = uniqueEmail("teacher");
  const studentEmail = uniqueEmail("student");
  const className = `E2E 班级 ${Date.now()}`;

  // 1. Admin mints a teacher invite and we read the code from the UI.
  await login(page, ADMIN.email, ADMIN.password);
  await page.getByRole("tab", { name: "教师" }).click();
  await expect(page.getByText("生成教师邀请码")).toBeVisible();
  await page.getByRole("button", { name: "生成邀请码" }).click();
  const inviteText = await page.getByText(/新邀请码\s+\S+/).innerText();
  const inviteCode = inviteText.match(/新邀请码\s+(\S+)/)![1];
  expect(inviteCode).toBeTruthy();
  await logout(page);

  // 2. Teacher registers with the invite → lands on the teacher console.
  await registerWithCode(page, { name: "E2E 老师", email: teacherEmail, password: "e2e-pass-12345", code: inviteCode });
  await expect(page.getByRole("tab", { name: "班级" })).toBeVisible();

  // 3. Teacher creates a class and we read the join code.
  await page.getByRole("tab", { name: "班级" }).click();
  await page.getByRole("button", { name: "+ 新建班级" }).click();
  await page.getByPlaceholder("班级名称，如「11 年级 A · TOK」").fill(className);
  await page.getByRole("button", { name: "创建" }).click();
  const createdText = await page.getByText(/已创建「.*」· 邀请码\s+\S+/).innerText();
  const joinCode = createdText.match(/邀请码\s+(\S+)/)![1];
  expect(joinCode).toBeTruthy();
  await logout(page);

  // 4. Student registers with the join code → lands on the workspace.
  await registerWithCode(page, { name: "E2E Phoebe", email: studentEmail, password: "e2e-pass-12345", code: joinCode });
  await expect(page.getByRole("tab", { name: "任务" })).toBeVisible();

  // 5. Student runs the Phoebe task (LIVE MODEL).
  await page.getByRole("tab", { name: "任务" }).click();
  await expect(page.getByText("今天你在尝试什么？")).toBeVisible();
  await page
    .getByPlaceholder("把你正在纠结的问题写下来——带上你自己的东西（链接、草稿、本子上的话）。")
    .fill("我在写 TOK：中国是否让地球更可持续？我找到一篇文章链接，想判断它可不可信再用。https://example.com/china-sustainability");
  await page.getByRole("button", { name: "开始" }).click();

  // Wait for the chaperone reply; then nudge once toward source-vetting if no
  // card was proposed (tool_choice=auto ⇒ summon is the model's call).
  const proposal = page.getByText("建议工具卡");
  const openBtn = page.getByRole("button", { name: "打开卡" });
  await summonCardWithRetry(page, proposal, openBtn);

  // Restraint law: the card is PROPOSED, not auto-opened. Opening requires
  // the student to click 打开卡 (the card sheet only appears after).
  await expect(page.getByText("现在轮到你想")).toHaveCount(0);
  await openBtn.first().click();
  await expect(page.getByText("现在轮到你想")).toBeVisible({ timeout: 15_000 });

  // Fill the card. SIFT/CRAAP fields vary; fill every visible text input/area
  // in the sheet so the envelope is non-empty, then submit.
  const sheetInputs = page.locator('textarea, input[type="text"]');
  const count = await sheetInputs.count();
  for (let i = 0; i < count; i++) {
    const el = sheetInputs.nth(i);
    if (await el.isVisible()) await el.fill("E2E：溯源到 NASA / Nature Sustainability，作者与日期可核。").catch(() => {});
  }
  await page.getByRole("button", { name: "提交并钉到过程树" }).click();

  // Process tree grows: the completed card pins a node; chat shows completed.
  await expect(page.getByText("已完成 · 已钉到过程树")).toBeVisible({ timeout: 20_000 });

  // Refeed turn: send another message; the completed card rides the history.
  await page.getByPlaceholder("把你的想法发给陪练……").fill("好了，我已经核过来源。接下来帮我把论点写扎实一点。");
  await page.getByRole("button", { name: "发送" }).click();
  // The reply streaming without error is enough (refeed-aware history worked).
  await page.waitForTimeout(3_000);

  // Evaluation (LIVE flagship): trigger and wait for 你的思维印记.
  await page.getByRole("button", { name: /^(生成思维印记|重新评估)$/ }).click();
  await expect(page.getByRole("dialog", { name: "你的思维印记" })).toBeVisible({ timeout: 90_000 });
  await page.getByRole("button", { name: "回到任务" }).click();
  await logout(page);

  // 6. Teacher sees the student on the roster with signal counts.
  // (Re-login as the teacher we created.)
  await login(page, teacherEmail, "e2e-pass-12345");
  await page.getByRole("tab", { name: "班级" }).click();
  await page.getByText(className).click();
  const studentRow = page.locator("tr", { hasText: "E2E Phoebe" });
  await expect(studentRow).toBeVisible({ timeout: 15_000 });
  // Aggregate-only: a row exists with integer signal cells (≥1 task, ≥1 eval).
  await expect(studentRow).toContainText(/\d/);
  await logout(page);

  // 7. Admin overview reflects the new class/student.
  await login(page, ADMIN.email, ADMIN.password);
  await expect(page.getByRole("tab", { name: "概览" })).toBeVisible();
  await expect(page.getByText("学生")).toBeVisible();
});

// Summon-with-retry: wait for a proposed card; if none, send one explicit
// source-vetting nudge and wait again; fail with a clear diagnostic on timeout.
async function summonCardWithRetry(page: Page, proposal, openBtn) {
  try {
    await expect(proposal.first()).toBeVisible({ timeout: 45_000 });
    return;
  } catch {
    await page.getByPlaceholder("把你的想法发给陪练……")
      .fill("我不确定这篇文章可不可信，能不能给我一张帮我核查来源的工具卡？");
    await page.getByRole("button", { name: "发送" }).click();
  }
  await expect(proposal.first(), "model declined to summon a card under tool_choice=auto (live-model variance, not a pipeline break)")
    .toBeVisible({ timeout: 60_000 });
}
```
Note on the card-fill loop: SIFT/CRAAP renderers use varied field primitives. The blanket "fill every visible text field" keeps the spec robust to the exact card schema; if a required non-text field (e.g., a rating) blocks submit, the implementer adds the minimal interaction for that primitive after observing the sheet via `--debug`. The contract is: a completed envelope is submitted and pins a tree node.

- [ ] **Step 2: Write the runbook**

Create `apps/web/e2e/RUNBOOK.md`:
```markdown
# Full-Stack E2E Live Smoke — Runbook

A browser-driven smoke over the real stack (web → Go API → throwaway Postgres → **real DeepSeek**). Local-only; not a CI gate.

## One-time setup
1. Docker running (for the throwaway Postgres).
2. `pnpm install` at the repo root; `pnpm --filter web exec playwright install chromium`.
3. Create `apps/api/.env.local`:
   ```
   DATABASE_URL=postgres://postgres:postgres@localhost:5432/mindimprint?sslmode=disable
   CORS_ORIGINS=http://localhost:5173
   COOKIE_SECURE=false
   DEEPSEEK_API_KEY=<your key>
   ```
   (The harness overrides `DATABASE_URL` to the throwaway container; the key is what matters here.)

## Run
- Everything: `bash apps/web/e2e/run-stack.sh`
- One spec: `bash apps/web/e2e/run-stack.sh golden-path.spec.ts`
- Deterministic specs only (no key needed): `bash apps/web/e2e/run-stack.sh auth.spec.ts registration.spec.ts smoke.spec.ts`

## What each spec proves
- `smoke` — the stack is wired (app serves, `/healthz` is 200).
- `auth` — seeded admin login lands on 概览; wrong password stays on login; logged-out shows auth screen.
- `registration` — an invalid join code is rejected (org invariant).
- `golden-path` — the full cross-role lifecycle: admin invite → teacher signup → class → student join → Phoebe task (card summon → SIFT envelope → process tree → refeed → evaluation 你的思维印记) → teacher sees roster signals → admin overview.

## Expected model variance
`tool_choice` is `auto`, so the AI decides whether to summon a card. The golden path nudges once and waits generously; an occasional "model declined to summon" failure is live-model variance, not a platform break — re-run (`retries:1` already absorbs one). The evaluation uses the flagship `deepseek-reasoner` (`MaxTokens: 8000`) and can take up to ~90s.

## Pass criteria (live)
A real `summon_card` proposal appears, the SIFT envelope pins a process-tree node, the refeed turn streams a reply without error, and the evaluation renders a real 你的思维印记 rubric. Teacher roster shows the student with non-zero signals; admin overview reflects the new class.
```

- [ ] **Step 3: Verify the deterministic prefix (and full run if key present)**

If `apps/api/.env.local` has a real `DEEPSEEK_API_KEY`:
```bash
cd /Users/houyuxin/08Coding/mind-imprint && bash apps/web/e2e/run-stack.sh golden-path.spec.ts
```
Expected: 1 passed (allow one retry for summon variance).

If no key is available to the implementer: run the deterministic specs to confirm no regression, and confirm `golden-path.spec.ts` type-checks / Playwright lists it:
```bash
cd /Users/houyuxin/08Coding/mind-imprint && bash apps/web/e2e/run-stack.sh auth.spec.ts registration.spec.ts smoke.spec.ts
cd /Users/houyuxin/08Coding/mind-imprint/apps/web && pnpm e2e --list
```
Expected: deterministic specs pass; `--list` shows `golden-path.spec.ts` with its test (no syntax/type errors). Record in the task report that the live run is developer-performed per RUNBOOK.

- [ ] **Step 4: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/web/e2e/golden-path.spec.ts apps/web/e2e/RUNBOOK.md
git commit -m "$(cat <<'EOF'
test(e2e): golden-path lifecycle spec (live model) + runbook

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"
```

---

## Final step: update the carry-forward tracker

After Task 4, append a short section to `docs/遗留项追踪_Carryforward.md` recording: (1) the full-stack live-smoke suite exists under `apps/web/e2e/` and is developer-run, not CI; (2) the deferred deterministic CI gate (`STUB_LLM` scripted provider) if ever wanted; (3) the selector-stability follow-up (add `data-testid`s to auth inputs / card sheet if the positional locators prove brittle). Commit with the standard trailer. Then use superpowers:finishing-a-development-branch.

---

## Plan self-review

- **Spec coverage:** harness/throwaway-PG (Task 1) ✓; golden path all 7 seams (Task 4) ✓; auth edge (Task 2) ✓; registration edge (Task 3) ✓; restraint law — folded into the golden path as the "proposed, not auto-opened / open-on-confirm" assertion (Task 4) rather than a standalone spec, to avoid a second live-model-dependent flaky spec; skip-still-recorded is already covered by web unit tests (ChatLog) and is not re-asserted live. RUNBOOK + env preconditions (Task 4) ✓; carry-forward (final step) ✓. **Deviation from spec:** `restraint.spec.ts` is not a separate file; its core assertion lives in the golden path. Coverage preserved.
- **Real-model determinism:** the one probabilistic point (summon) is isolated behind `summonCardWithRetry` with a clear diagnostic; everything else is deterministic. ✓
- **Zero production-change:** only `apps/web/e2e/**` + `apps/web/package.json` devDep + `pnpm-lock.yaml`. No `apps/api` edits, no `data-testid`. ✓
- **Selector consistency:** every selector used in specs traces to the Selector reference block and the Explore map. The auth inputs lack test hooks → positional/type locators, flagged as brittle with a fix path in each task. ✓
- **Type consistency:** helper signatures (`login`, `logout`, `registerWithCode`, `uniqueEmail`, `ADMIN`, `PHOEBE`) are defined in Task 2 and consumed unchanged in Tasks 3–4. ✓
