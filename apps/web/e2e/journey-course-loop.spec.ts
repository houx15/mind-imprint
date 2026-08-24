import { test, expect } from "@playwright/test";
import { registerStudent, openRail, uniqueEmail } from "./helpers";

// J2 — Finish a course → where it surfaces.
// Drives the seeded course "CRRAAB 信源评估" (a-mid) to real COMPLETION. a-mid
// ships with `structure` + `render_cache` but NO 2.0 `course_definition`, so
// CoursesContainer routes it to the LEGACY linear CoursePlayer (getCourseDefinition
// 404 → legacy). That player is deterministic + model-free: each step gates
// 下一步/完成课程 on revealing every segment (tap the page) AND answering every
// quiz (any option — never gated on correctness). Completing the last step opens
// the CourseReport (学习报告 · 课程完成).
const JOIN_CODE = "DEMO-0001";
const COURSE_TITLE = "CRRAAB 信源评估：从机构到亲历者到专家";

test("J2: course loop — open → walk steps → complete → report", async ({ page }) => {
  test.setTimeout(180_000);
  const email = uniqueEmail("j2-student");
  await registerStudent(page, { name: "E2E 学员", email, code: JOIN_CODE });

  // 1. 课程 tab → grid → open the course's detail → 开始学习 → the player.
  await openRail(page, "课程");
  await page.getByText(COURSE_TITLE).first().click();
  await expect(page.getByRole("heading", { name: COURSE_TITLE })).toBeVisible({ timeout: 15_000 });
  await page.getByRole("button", { name: /开始学习|继续|回顾/ }).click();

  // The legacy player: reveal hint (while more to reveal), the 提交 quiz button,
  // the 下一步/完成课程 nav gate, and — on completion — the report heading.
  const revealHint = page.getByText("点击页面任意处继续");
  const nextBtn = page.getByRole("button", { name: /^(下一步|完成课程)$/ });
  const reportHeading = page.getByText(/课程完成/);

  await expect(nextBtn).toBeVisible({ timeout: 20_000 });

  // 2. Drive to completion. Each loop: if a step still has hidden segments, reveal
  //    one; else answer the next un-submitted quiz (pick an option + 提交); else
  //    the gate is clear → advance (下一步, or 完成课程 which opens the report).
  const done = () => reportHeading.isVisible().catch(() => false);
  for (let i = 0; i < 120; i++) {
    if (await done()) break;

    // Reveal the next segment when the "点击页面任意处继续" hint is present.
    if (await revealHint.isVisible().catch(() => false)) {
      await revealHint.click().catch(() => {});
      await page.waitForTimeout(120);
      continue;
    }

    // Answer the first un-submitted quiz: select its first option, then 提交.
    // (After submit the block locks and its 提交 disappears, so `.first()` walks
    //  to the next quiz on the next pass.)
    const submit = page.getByRole("button", { name: "提交" }).first();
    if ((await submit.count()) && (await submit.isVisible().catch(() => false))) {
      const option = submit.locator("xpath=preceding-sibling::div[1]//button").first();
      if (await option.count()) await option.click().catch(() => {});
      if (await submit.isEnabled().catch(() => false)) await submit.click().catch(() => {});
      await page.waitForTimeout(150);
      continue;
    }

    // Fully revealed + answered → the gate is open. Advance (下一步 / 完成课程).
    if (await nextBtn.isEnabled().catch(() => false)) {
      await nextBtn.click().catch(() => {});
      await page.waitForTimeout(400);
      continue;
    }
    await page.waitForTimeout(150);
  }

  // 3. Completed: the player navigated to the 学习报告 · 课程完成 report.
  await expect(reportHeading).toBeVisible({ timeout: 15_000 });

  // 4. 返回课程 returns to the course grid.
  await page.getByRole("button", { name: "返回课程" }).click();
  await expect(page.getByText("系统地学会一种思考方式")).toBeVisible({ timeout: 15_000 });
});
