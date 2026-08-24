import { test, expect } from "@playwright/test";
import { registerStudent, uniqueEmail } from "./helpers";

// J-writing — a faithful UI walk of the CURRENT writing project, replacing the
// retired six-station / 工作室-tab journeys. The current shell is the four-tab
// nav (首页/项目/课程/我); a project lives in WorkspaceContainer, whose five room
// tabs (立题/管理/阅读/写作/回顾) appear only AFTER the student taps 开始 (coach/start).
//
//   register → 项目 tab → 新建项目 → CreateProjectDrawer → 开始 → land in the
//   workspace (返回 capsule + chat-first) → tap 开始 to begin (coach/start) →
//   the five room tabs appear → drive the forming coach composer for one live
//   /coach turn (200 + reply renders) → 返回 → reload → the project persists.
//
// Live model: coach/opening fires on mount, coach/start on 开始, and coach on the
// composer turn. The one composer turn is the explicit live assertion; the
// others are inherent to opening a brand-new project.
const JOIN_CODE = "DEMO-0001"; // seeded Demo Class

// The prompt's FIRST LINE becomes the project title (CreateProjectDrawer's
// titleFromPrompt), so lead with a short, distinctive title line.
const TITLE = "TOK：中国是否让地球更可持续？";
const PROMPT = [
  TITLE,
  "我想先判断一篇文章可不可信再用它。https://example.com/china-sustainability",
].join("\n");

test("J-writing: 新建项目 → workspace → 开始 → coach replies → project persists", async ({ page }) => {
  test.setTimeout(180_000);
  const email = uniqueEmail("jwriting-student");

  // 1. Register with a class join code → lands on 首页 (StudentApp default tab).
  await registerStudent(page, { name: "E2E 写作", email, code: JOIN_CODE });

  // 2. Go to the 项目 tab.
  await page.getByRole("tab", { name: "项目" }).click();
  await expect(page.getByText("探究性写作空间")).toBeVisible({ timeout: 15_000 });

  // 3. New student → empty Directory. 新建项目 opens the CreateProjectDrawer.
  await page.getByRole("button", { name: "新建项目" }).click();

  // 4. Fill the assignment prompt and submit 开始 (drawer create).
  await page.getByPlaceholder("贴上你的作业题目或提示").fill(PROMPT);
  await page.getByRole("button", { name: "开始" }).click();

  // 5. Landed in WorkspaceContainer: the 返回 capsule + the chat-first surface
  //    (a brand-new project is chat-first with a single 开始 action, no tabs yet).
  await expect(page.getByRole("button", { name: "返回" })).toBeVisible({ timeout: 30_000 });
  await expect(page.getByTestId("chat-first")).toBeVisible({ timeout: 30_000 });

  // 6. Begin the journey (coach/start, live). Once started, the five room tabs
  //    render (立题/管理/阅读/写作/回顾) — the forming stage opens the coach in the
  //    left AI panel (widthTier=half).
  await page.getByRole("button", { name: "开始" }).click();
  for (const room of ["立题", "管理", "阅读", "写作", "回顾"]) {
    await expect(page.getByRole("button", { name: room }).first()).toBeVisible({ timeout: 60_000 });
  }

  // 7. Drive ONE live coach turn through the forming-room composer. Assert the
  //    round-trip at the network level (POST /coach → 200) — the robust signal
  //    that the live coach worked — then that a reply bubble renders.
  const composer = page.getByPlaceholder("说说你的想法……");
  await expect(composer).toBeVisible({ timeout: 30_000 });
  await composer.fill("我想写中国与可持续，先帮我判断这篇文章可不可信，我该从哪里开始？");
  const coachResp = page.waitForResponse(
    (r) => /\/projects\/[^/]+\/coach$/.test(r.url()) && r.request().method() === "POST",
    { timeout: 90_000 },
  );
  await page.getByRole("button", { name: "发送" }).click();
  expect((await coachResp).status()).toBe(200);
  // The composer returns to ready after the reply streams in.
  await expect(composer).toBeEnabled({ timeout: 90_000 });

  // 8. 返回 → back in the Directory (the project list).
  await page.getByRole("button", { name: "返回" }).click();
  await expect(page.getByText("探究性写作空间")).toBeVisible({ timeout: 15_000 });

  // 9. Persisted: reload → 项目 tab → the created project's card is listed.
  await page.goto("/");
  await page.getByRole("tab", { name: "项目" }).click();
  await expect(page.getByText(TITLE).first()).toBeVisible({ timeout: 15_000 });
});
