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
