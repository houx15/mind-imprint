import { test, expect } from "@playwright/test";
import { registerStudent, openRail, uniqueEmail } from "./helpers";
import { studentReply } from "./student-sim";

// J2 — Finish a course → where it surfaces.
// Drives the seed course "一条网络信息，该不该信" (phases 演示→引导→独立→回看→练一手)
// to real COMPLETION on the current runtime. With advance now floor-authoritative
// (Finding A fix), meeting each phase's floor + clicking 继续 advances:
// 演示=view steps · 引导=dispose CRAAP card · 独立/回看=≥1 genuine turn (student-sim).
// The backend is the only writer of finished.
const JOIN_CODE = "DEMO-0001";
const COURSE_TITLE = "一条网络信息，该不该信";

test("J2: course loop — start → walk phases → complete → report", async ({ page }) => {
  test.setTimeout(300_000);
  const email = uniqueEmail("j2-student");
  await registerStudent(page, { name: "E2E 学员", email, code: JOIN_CODE });

  // 1. Open the course → player.
  await openRail(page, "课程");
  await page.getByText(COURSE_TITLE).first().click();
  const nextBtn = page.getByLabel("下一步");
  await expect(nextBtn).toBeVisible({ timeout: 20_000 });
  await expect(page.getByText("演示").first()).toBeVisible();

  // On finish the runtime mints `finished` and the player AUTO-navigates to the
  // CourseReport (回看) — the 完成课程 button is never the completion signal; the
  // report heading is.
  const reportHeading = page.getByText(/课程完成/);
  const acceptOffer = page.getByRole("button", { name: "接受" });
  const skipCard = page.getByRole("button", { name: "跳过这张卡" });
  const askInput = page.getByPlaceholder("输入你的问题……");
  const askSend = page.getByLabel("发送");
  const bubbles = page.getByTestId("ask-bubble");

  // 2. Drive to completion. Each iteration: stop if finished, dispose an offered
  //    card, else answer the coach (student-sim) if it's waiting, then move
  //    forward. Every step is finish-tolerant — the moment the backend mints
  //    finished, 下一步 is replaced by 完成课程, so we re-check before each action.
  const finished = () => reportHeading.isVisible().catch(() => false);
  for (let i = 0; i < 40; i++) {
    if (await finished()) break;

    // 引导 floor: dispose the offered CRAAP card (skip counts as dispositioned).
    if (await acceptOffer.count()) {
      await acceptOffer.first().click();
      await expect(skipCard.first()).toBeVisible({ timeout: 10_000 });
      await skipCard.first().click();
      await page.waitForTimeout(600);
      continue;
    }

    // Coach waiting (a student_turns floor unmet auto-expands the panel) → answer
    // it concretely via the student-sim. Only when the input is present + enabled
    // (it disables while a turn is in flight).
    if ((await askInput.isVisible().catch(() => false)) && (await askInput.isEnabled().catch(() => false))) {
      const n = await bubbles.count();
      const coachText = n ? await bubbles.nth(n - 1).innerText() : "请继续。";
      const reply = await studentReply(coachText);
      await askInput.fill(reply).catch(() => {});
      const askResp = page
        .waitForResponse((r) => r.url().includes("/session/ask") && r.request().method() === "POST", { timeout: 90_000 })
        .catch(() => {});
      await askSend.click().catch(() => {});
      await askResp;
      await page.waitForTimeout(400);
    }

    // Move forward — but a boundary advance may mint finished, which swaps 下一步
    // for 完成课程. Re-check, then click only if 下一步 is still there.
    if (await finished()) break;
    if (await nextBtn.count()) {
      await nextBtn.first().click().catch(() => {});
      await page.waitForTimeout(2000);
    }
  }

  // 3. Completed: the backend minted finished and the app auto-navigated to the
  //    学习报告 · 课程完成 report (回看), where completion "appears".
  await expect(reportHeading).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText(COURSE_TITLE).first()).toBeVisible();

  // 4. Restart resets the session and returns to the course grid.
  await page.getByRole("button", { name: "重新开始" }).click();
  await expect(page.getByText("系统地学会一种思考方式")).toBeVisible({ timeout: 15_000 });
});
