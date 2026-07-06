import { test, expect, Page, Locator } from "@playwright/test";
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
  // Teacher invite codes are formatted "T-XXXXXX" (codes.go NewTeacherInviteCode;
  // alphabet A-Z2-9, ambiguous chars excluded). Extract the exact token rather
  // than a bare \S+, which would swallow trailing copy if the banner layout ever
  // loses its " · " separator.
  const inviteText = await page.getByText(/新邀请码\s+T-/).innerText();
  const inviteCode = inviteText.match(/(T-[A-Z2-9]+)/)![1];
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
  // The banner renders "…邀请码 {join_code}（分享给学生加入）" with NO space before
  // the parenthetical, so a bare \S+ would capture "GQB3-9D9D（分享给学生加入）" and
  // the student signup would be rejected as an invalid code. Match the exact join
  // code shape instead: XXXX-XXXX over the A-Z2-9 alphabet (codes.go NewClassJoinCode).
  const createdText = await page.getByText(/已创建「.*」· 邀请码\s+[A-Z2-9]{4}-[A-Z2-9]{4}/).innerText();
  const joinCode = createdText.match(/邀请码\s+([A-Z2-9]{4}-[A-Z2-9]{4})/)![1];
  expect(joinCode).toBeTruthy();
  await logout(page);

  // 4. Student registers with the join code → lands on the workspace.
  await registerWithCode(page, { name: "E2E Phoebe", email: studentEmail, password: "e2e-pass-12345", code: joinCode });
  await expect(page.getByRole("tab", { name: "任务" })).toBeVisible();

  // 5. Student runs the Phoebe task (LIVE MODEL).
  await page.getByRole("tab", { name: "任务" }).click();
  await expect(page.getByText("你想搞懂什么？")).toBeVisible();
  await page
    .getByPlaceholder("开一个新项目——把你正纠结的问题写下来，带上你自己的东西（链接、草稿、本子上的话）。")
    .fill("我在写 TOK：中国是否让地球更可持续？我找到一篇文章链接，想判断它可不可信再用。https://example.com/china-sustainability");
  await page.getByRole("button", { name: "开始" }).click();

  // Wait for the chaperone reply; then nudge once toward source-vetting if no
  // card was proposed (tool_choice=auto ⇒ summon is the model's call).
  const proposal = page.getByText("建议工具卡");
  const openBtn = page.getByRole("button", { name: "打开卡" });
  await summonCardWithRetry(page, proposal);

  // Restraint law: the card is PROPOSED, not auto-opened. Opening requires
  // the student to click 打开卡 (the card sheet only appears after).
  await expect(page.getByText("现在轮到你想")).toHaveCount(0);
  await openBtn.first().click();
  await expect(page.getByText("现在轮到你想")).toBeVisible({ timeout: 15_000 });

  // Fill the card. SIFT/CRAAP fields vary; fill every visible text input/area
  // in the sheet so the envelope is non-empty, then submit.
  // Scope to the card sheet to exclude the chat composer textarea.
  // CardSheetHost.tsx renders: root-overlay > inner-sheet (position:relative, maxWidth:880px)
  //   which contains the "现在轮到你想" header AND the "提交并钉到过程树" footer button.
  // .last() gives the innermost div matching both filters (the inner-sheet, not its parent).
  const sheet = page.locator("div").filter({
    has: page.getByText("现在轮到你想"),
  }).filter({
    has: page.getByRole("button", { name: "提交并钉到过程树" }),
  }).last();
  const sheetInputs = sheet.locator('textarea, input[type="text"]');
  const count = await sheetInputs.count();
  for (let i = 0; i < count; i++) {
    const el = sheetInputs.nth(i);
    if (await el.isVisible()) await el.fill("E2E：溯源到 NASA / Nature Sustainability，作者与日期可核。").catch(() => {});
  }
  await page.getByRole("button", { name: "提交并钉到过程树" }).click();

  // Process tree grows: the completed card pins a node; chat shows completed.
  await expect(page.getByText("已完成 · 已钉到过程树")).toBeVisible({ timeout: 20_000 });

  // Refeed turn: send another message; the completed card rides the history.
  // ChatLog.tsx renders every message (student or AI) as a child div inside
  // #mk-chat > div (the maxWidth-720 centering div). Count before send, then
  // assert the count grew by ≥2: +1 for the student message, +≥1 for the AI reply.
  const chatItems = page.locator("#mk-chat > div > div");
  const countBefore = await chatItems.count();
  await page.getByPlaceholder("把你的想法发给陪练……").fill("好了，我已经核过来源。接下来帮我把论点写扎实一点。");
  await page.getByRole("button", { name: "发送" }).click();
  // Fail if no AI reply renders (live-model error or pipeline break would leave count at +1).
  await expect(async () => {
    const n = await chatItems.count();
    expect(n).toBeGreaterThanOrEqual(countBefore + 2);
  }).toPass({ timeout: 60_000, intervals: [1_000] });

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
  // OverviewView.tsx renders stat cards: each outer div's textContent is label+count
  // (e.g. "学生3"). Assert the student stat shows a non-zero positive integer,
  // proving real data was fetched — not just that a static label is present.
  await expect(page.locator("div").filter({ hasText: /^学生[1-9]\d*$/ })).toBeVisible({ timeout: 15_000 });
});

// Summon-with-retry: wait for a proposed card; if none, send one explicit
// source-vetting nudge and wait again; fail with a clear diagnostic on timeout.
async function summonCardWithRetry(page: Page, proposal: Locator) {
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
